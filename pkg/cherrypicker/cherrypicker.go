package cherrypicker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mattermost/cicd-sdk/pkg/git"
	"github.com/mattermost/cicd-sdk/pkg/github"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/util"
)

const (
	gitCommand      = "git"
	rebaseMagic     = ".git/rebase-apply"
	newBranchSlug   = "automated-cherry-pick-of-"
	prTitleTemplate = "Automated cherry pick of #%d on %s"
	prBodyTemplate  = `Automated cherry pick of #%d on %s

Cherry pick of #%d on %s.

/cc  @%s

` + "```release-note\nNONE\n```\n"
)

// CherryPicker captures the cherry-pick creation logic in go
type CherryPicker struct {
	impl    cherryPickerImplementation
	state   State
	options *Options
}

// New returns a cherrypicker with default opts
func New() *CherryPicker {
	return NewWithOptions(defaultCherryPickerOpts)
}

// NewCherryPicker returns a cherrypicker with default opts
func NewWithOptions(opts *Options) *CherryPicker {
	if opts.Remote == "" {
		opts.Remote = defaultCherryPickerOpts.Remote
	}
	if opts.RepoPath == "" {
		opts.RepoPath = defaultCherryPickerOpts.RepoPath
	}
	return &CherryPicker{
		options: opts,
		state:   State{},
		impl:    &defaultCPImplementation{},
	}
}

type Options struct {
	RepoPath  string // Local path to the repository
	RepoOwner string // Org of the repo we are using
	RepoName  string // Name of the repository
	ForkOwner string
	Remote    string
}

var defaultCherryPickerOpts = &Options{
	RepoPath:  ".",
	Remote:    "origin",
	ForkOwner: "",
}

type State struct {
	github *github.GitHub  // github client
	git    *git.Git        // git client
	repo   *git.Repository // Repository where the cherrypicker will operate
	ghrepo *github.Repository
}

// Actual implementation of the CP interfaces
type cherryPickerImplementation interface {
	initialize(context.Context, *State, *Options) error
	createBranch(*State, *Options, string, *github.PullRequest) (string, error)
	cherrypickCommits(*State, *Options, []string, string) error
	cherrypickMergeCommit(*State, *Options, string, string, int) error
	pushFeatureBranch(*State, *Options, string) error
	getPullRequest(context.Context, int, *github.Repository) (*github.PullRequest, error)
	getMergeMode(context.Context, *github.PullRequest) (string, error)
	cherryPickRebasedPR(context.Context, *State, *Options, *github.PullRequest, string) error
	createPullRequest(ctx context.Context, ghrepo *github.Repository, featureBranch, branch string,
		originalPR *github.PullRequest) (*github.PullRequest, error)
}

// Initialize checks the environment and populates the state
func (impl *defaultCPImplementation) initialize(ctx context.Context, state *State, opts *Options) (err error) {
	state.github = github.New()
	state.git = git.New()

	// Check the repository path exists
	if util.Exists(filepath.Join(opts.RepoPath, rebaseMagic)) {
		return errors.New("there is a rebase in progress, unable to cherry pick at this time")
	}

	state.ghrepo = github.NewRepository(opts.RepoOwner, opts.RepoName)

	// TODO: Add a bit more checks to the current repo state

	var repo *git.Repository
	// If we do not have a path to the repository, we clone the repo
	if opts.RepoPath == "" {
		tmpDir, err2 := os.MkdirTemp("", "git-repo-tmpclone-")
		if err2 != nil {
			return errors.Wrap(err, "while cloning repository")
		}
		logrus.Debugf("cloning %s/%s to %s", opts.RepoOwner, opts.RepoName, opts.RepoPath)
		repo, err = state.git.CloneRepo(git.GitHubURL(opts.RepoOwner, opts.RepoName), tmpDir)
	} else {
		// Open an existing repository
		repo, err = state.git.OpenRepo(opts.RepoPath)
	}
	if err != nil {
		return errors.Wrapf(
			err, "opening or cloning repo %s/%s", opts.RepoOwner, opts.RepoName,
		)
	}

	// And add it to the state
	state.repo = repo
	return nil
}

// CreateCherryPickPR creates a cherry-pick PR to the the given branch
func (cp *CherryPicker) CreateCherryPickPR(prNumber int, branch string) error {
	return cp.CreateCherryPickPRWithContext(context.Background(), prNumber, branch)
}

// CreateCherryPickPR creates a cherry-pick PR to the the given branch
func (cp *CherryPicker) CreateCherryPickPRWithContext(ctx context.Context, prNumber int, branch string) error {
	if err := cp.impl.initialize(ctx, &cp.state, cp.options); err != nil {
		return errors.Wrap(err, "verifying environment")
	}

	// Fetch the pull request
	pr, err := cp.impl.getPullRequest(ctx, prNumber, cp.state.ghrepo)
	if err != nil {
		return errors.Wrapf(err, "getting pull request %d", prNumber)
	}

	// Next step: Find out how the PR was merged
	mergeMode, err := cp.impl.getMergeMode(ctx, pr)
	if err != nil {
		return errors.Wrapf(err, "getting merge mode for PR #%d", pr.Number)
	}

	// Create the CP branch
	featureBranch, err := cp.impl.createBranch(&cp.state, cp.options, branch, pr)
	if err != nil {
		return errors.Wrap(err, "creating the feature branch")
	}

	switch mergeMode {
	case github.MMSQUASH:
		// The easiest case: PR was squashed. In this case we only need to CP
		// the sha returned in merge_commit_sha
		if err := cp.impl.cherrypickCommits(
			&cp.state, cp.options, []string{pr.MergeCommitSHA}, branch,
		); err != nil {
			return errors.Wrap(err, "cherrypicking squashed commit")
		}
	case github.MMMERGE:
		// Next, if the PR resulted in a merge commit, we only need to cherry-pick
		// the `merge_commit_sha` but we have to find out which parent's tree we want
		// to generate the diff from:
		parent, err := pr.PatchTreeID(ctx)
		if err != nil {
			return errors.Wrap(err, "searching for parent patch tree")
		}
		if err := cp.impl.cherrypickMergeCommit(
			&cp.state, cp.options, branch, pr.MergeCommitSHA, parent,
		); err != nil {
			return errors.Wrap(err, "cherrypicking merge commit")
		}
	case github.MMREBASE:
		// Last case. We are dealing with a rebase. In this case we have to take the
		// merge commit and go back in the git log to find the previous trees and
		// CP the commits where they merged
		if err := cp.impl.cherryPickRebasedPR(
			ctx, &cp.state, cp.options, pr, branch,
		); err != nil {
			return errors.Wrap(err, "cherrypicking squashed commit")
		}
	}

	if err = cp.impl.pushFeatureBranch(&cp.state, cp.options, featureBranch); err != nil {
		return errors.Wrap(err, "pushing branch to git remote")
	}

	// Create the pull request
	pullrequest, err := cp.impl.createPullRequest(ctx, cp.state.ghrepo, featureBranch, branch, pr)
	if err != nil {
		return errors.Wrap(err, "creating pull request in github")
	}

	logrus.Info(fmt.Sprintf("Successfully created pull request #%d", pullrequest.Number))

	return nil
}

type defaultCPImplementation struct{}

// createBranch creates the new branch for the cherry pick and
// switches to it. The new branch is created frp, sourceBranch.
func (impl *defaultCPImplementation) createBranch(
	state *State, opts *Options, sourceBranch string, pr *github.PullRequest,
) (branchName string, err error) {
	// The new name of the branch, we append the date to make it unique
	branchName = newBranchSlug + fmt.Sprintf("%d", pr.Number) + "-" + fmt.Sprintf("%d", (time.Now().Unix()))

	// Switch to the sourceBranch, this ensures it exists and from there we branch
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "checkout", sourceBranch).RunSilentSuccess(); err != nil {
		return "", errors.Wrapf(err, "switching to source branch %s", sourceBranch)
	}

	if err := state.repo.CreateBranch(branchName); err != nil {
		return "", errors.Wrap(err, "creating cherry pick branch")
	}

	logrus.Info("created cherry-pick feature branch " + branchName)
	return branchName, nil
}

// cherrypickCommits calls the git command via the shell to cherry-pick the list of
// commits passed into the current repository path.
func (impl *defaultCPImplementation) cherrypickCommits(
	state *State, opts *Options, commits []string, branch string,
) (err error) {
	logrus.Infof("Cherry picking %d commits to branch %s", len(commits), branch)
	if err := state.repo.CherryPickCommits(commits, branch); err != nil {
		return errors.Wrapf(err, "cherry picking %d commits to %s", len(commits), branch)
	}
	conflicts, _, err := state.repo.HasMergeConflicts()
	if err != nil {
		return errors.Wrap(err, "checking for conflicts")
	}
	if conflicts {
		return errors.Wrap(err, "conflicts found while cherrypicking")
	}
	return nil
}

func (impl *defaultCPImplementation) cherrypickMergeCommit(
	state *State, opts *Options, branch, commit string, parent int,
) (err error) {
	if err := state.repo.CherryPickMergeCommit(branch, commit, parent); err != nil {
		return errors.Wrapf(err, "cherry-picking merge commit %s into %s", commit, branch)
	}
	conflicts, _, err := state.repo.HasMergeConflicts()
	if err != nil {
		return errors.Wrap(err, "checking for conflicts")
	}
	if conflicts {
		return errors.Wrap(err, "conflicts found while cherrypicking")
	}
	return nil
}

// pushFeatureBranch pushes thw new branch with the CPs to the remote
func (impl *defaultCPImplementation) pushFeatureBranch(
	state *State, opts *Options, featureBranch string,
) error {
	if err := state.repo.PushBranch(featureBranch, opts.Remote); err != nil {
		return errors.Wrap(err, "pushing CP feature branch")
	}
	logrus.Info(fmt.Sprintf("Successfully pushed %s to remote %s", featureBranch, opts.Remote))
	return nil
}

// getPullRequest gets the pull request we are cherrypicking
func (impl *defaultCPImplementation) getPullRequest(
	ctx context.Context, prNumber int, ghrepo *github.Repository,
) (*github.PullRequest, error) {
	// Fetch the pull request from the repository
	return ghrepo.GetPullRequest(ctx, prNumber)
}

func (impl *defaultCPImplementation) getMergeMode(ctx context.Context, pr *github.PullRequest) (string, error) {
	return pr.GetMergeMode(ctx)
}

// cherryPickRebasedPR
func (impl *defaultCPImplementation) cherryPickRebasedPR(
	ctx context.Context, state *State, opts *Options, pr *github.PullRequest, branch string,
) error {
	// Get the lsit of commits rebased in the PR
	rebaseCommits, err := pr.GetRebaseCommits(ctx)
	if err != nil {
		return errors.Wrapf(err, "while getting commits in rebase from PR #%d", pr.Number)
	}
	// To open a PR we need to make sure we have at least one commit
	if len(rebaseCommits) == 0 {
		return errors.Errorf("empty commit list while searching from commits from PR#%d", pr.Number)
	}

	if err := impl.cherrypickCommits(
		state, opts, rebaseCommits, branch,
	); err != nil {
		return errors.Wrap(err, "cherrypicking squashed commit")
	}
	return nil
}

// createPullRequest opens
func (impl *defaultCPImplementation) createPullRequest(
	ctx context.Context, ghrepo *github.Repository, featureBranch, branch string,
	originalPR *github.PullRequest) (*github.PullRequest, error) {
	// Create the pull request in te repository
	return ghrepo.CreatePullRequest(
		ctx, featureBranch, branch,
		fmt.Sprintf(prTitleTemplate, originalPR.Number, branch),
		fmt.Sprintf(prBodyTemplate, originalPR.Number, branch, originalPR.Number, branch, originalPR.Username),
		&github.NewPullRequestOptions{MaintainerCanModify: true},
	)
}
