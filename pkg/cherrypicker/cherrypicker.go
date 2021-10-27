package cherrypicker

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"github.com/puerco/mattermod-refactor/pkg/github"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/util"
)

const (
	gitCommand      = "git"
	rebaseMagic     = ".git/rebase-apply"
	newBranchSlug   = "automated-cherry-pick-of-"
	REBASE          = "rebase"
	MERGE           = "merge"
	SQUASH          = "squash"
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
	options Options
}

// NewCherryPicker returns a cherrypicker with default opts
func NewCherryPicker() *CherryPicker {
	return NewCherryPickerWithOptions(defaultCherryPickerOpts)
}

// NewCherryPicker returns a cherrypicker with default opts
func NewCherryPickerWithOptions(opts Options) *CherryPicker {
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

var defaultCherryPickerOpts = Options{
	RepoPath:  ".",
	Remote:    "origin",
	ForkOwner: "",
}

type State struct {
	Repository *git.Repository    // Repo object to
	github     *github.GitHub     // go-github client
	repo       *github.Repository // Repository where the cherrypicker will operate
}

// Actual implementation of the CP interfaces
type cherryPickerImplementation interface {
	initialize(context.Context, *State, *Options) error
	createBranch(*State, *Options, string, *github.PullRequest) (string, error)
	cherrypickCommits(*State, *Options, string, []string) error
	cherrypickMergeCommit(*State, *Options, string, []string, int) error
	pushFeatureBranch(*State, *Options, string) error
}

// Initialize checks the environment and populates the state
func (impl *defaultCPImplementation) initialize(ctx context.Context, state *State, opts *Options) error {
	state.github = github.New()

	// Check the repository path exists
	if util.Exists(filepath.Join(opts.RepoPath, rebaseMagic)) {
		return errors.New("there is a rebase in progress, unable to cherry pick at this time")
	}

	// Open the repo
	repo, err := git.PlainOpen(opts.RepoPath)
	if err != nil {
		return errors.Wrapf(err, "opening repository from %s", opts.RepoPath)
	}

	// And add it to the state
	state.Repository = repo
	return nil
}

// CreateCherryPickPR creates a cherry-pick PR to the the given branch
func (cp *CherryPicker) CreateCherryPickPR(prNumber int, branch string) error {
	return cp.CreateCherryPickPRWithContext(context.Background(), prNumber, branch)
}

// CreateCherryPickPR creates a cherry-pick PR to the the given branch
func (cp *CherryPicker) CreateCherryPickPRWithContext(ctx context.Context, prNumber int, branch string) error {
	if err := cp.impl.initialize(ctx, &cp.state, &cp.options); err != nil {
		return errors.Wrap(err, "verifying environment")
	}

	// Fetch the pull request
	pr, err := cp.state.repo.GetPullRequest(ctx, prNumber)
	if err != nil {
		return errors.Wrapf(err, "getting pull request %d", prNumber)
	}

	// Next step: Find out how the PR was merged
	mergeMode, err := pr.GetMergeMode(ctx)
	if err != nil {
		return errors.Wrapf(err, "getting merge mode for PR #%d", pr.Number)
	}

	// Create the CP branch
	featureBranch, err := cp.impl.createBranch(&cp.state, &cp.options, branch, pr)
	if err != nil {
		return errors.Wrap(err, "creating the feature branch")
	}

	var cpError error

	// The easiest case: PR was squashed. In this case we only need to CP
	// the sha returned in merge_commit_sha
	if mergeMode == SQUASH {
		cpError = cp.impl.cherrypickCommits(
			&cp.state, &cp.options, branch, []string{pr.MergeCommitSHA},
		)
	}

	// Next, if the PR resulted in a merge commit, we only need to cherry-pick
	// the `merge_commit_sha` but we have to find out which parent's tree we want
	// to generate the diff from:
	if mergeMode == MERGE {
		parent, err2 := pr.PatchTreeID(ctx)
		if err2 != nil {
			return errors.Wrap(err2, "searching for parent patch tree")
		}
		cpError = cp.impl.cherrypickMergeCommit(
			&cp.state, &cp.options, branch, []string{pr.MergeCommitSHA}, parent,
		)
	}

	// Last case. We are dealing with a rebase. In this case we have to take the
	// merge commit and go back in the git log to find the previous trees and
	// CP the commits where they merged
	if mergeMode == REBASE {
		rebaseCommits, err2 := pr.GetRebaseCommits(ctx)
		if err2 != nil {
			return errors.Wrapf(err2, "while getting commits in rebase from PR #%d", pr.Number)
		}

		if len(rebaseCommits) == 0 {
			return errors.Errorf("empty commit list while searching from commits from PR#%d", pr.Number)
		}

		cpError = cp.impl.cherrypickCommits(
			&cp.state, &cp.options, branch, rebaseCommits,
		)
	}

	if cpError != nil {
		return errors.Errorf("while cherrypicking pull request %d of type %s", pr.Number, mergeMode)
	}

	if err = cp.impl.pushFeatureBranch(&cp.state, &cp.options, featureBranch); err != nil {
		return errors.Wrap(err, "pushing branch to git remote")
	}

	// Create the pull request
	pullrequest, err := cp.state.repo.CreatePullRequest(
		ctx, featureBranch, branch,
		fmt.Sprintf(prTitleTemplate, prNumber, branch),
		fmt.Sprintf(prBodyTemplate, prNumber, branch, prNumber, branch, pr.Username),
		&github.NewPullRequestOptions{MaintainerCanModify: true},
	)
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

	// Create the new branch:
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "branch", branchName).RunSilentSuccess(); err != nil {
		return "", errors.Wrap(err, "creating CP branch")
	}

	// Create the new branch:
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "checkout", branchName).RunSilentSuccess(); err != nil {
		return "", errors.Wrap(err, "creating CP branch")
	}

	logrus.Info("created cherry-pick feature branch " + branchName)
	return branchName, nil
}

// cherrypickCommits calls the git command via the shell to cherry-pick the list of
// commits passed into the current repository path.
func (impl *defaultCPImplementation) cherrypickCommits(
	state *State, opts *Options, branch string, commits []string,
) (err error) {
	logrus.Infof("Cherry picking %d commits to branch %s", len(commits), branch)
	cmd := command.NewWithWorkDir(opts.RepoPath, gitCommand, append([]string{"cherry-pick"}, commits...)...)
	if _, err = cmd.RunSilent(); err != nil {
		return errors.Wrap(err, "running git cherry-pick")
	}

	// Check if the cp was halted due to unmerged commits
	output, err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "status", "--porcelain",
	).RunSuccessOutput()
	if err != nil {
		return errors.Wrap(err, "while trying to look for merge conflicts")
	}
	for _, line := range strings.Split(output.Output(), "\n") {
		if strings.HasPrefix(line, "U") {
			return errors.Errorf("conflicts detected, cannot merge:\n%s", output.Output())
		}
	}
	return nil
}

func (impl *defaultCPImplementation) cherrypickMergeCommit(
	state *State, opts *Options, branch string, commits []string, parent int,
) (err error) {
	cmd := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, append([]string{"cherry-pick", "-m", fmt.Sprintf("%d", parent)}, commits...)...,
	)
	if err = cmd.RunSuccess(); err != nil {
		return errors.Wrap(err, "running git cherry-pick")
	}
	return nil
}

// pushFeatureBranch pushes thw new branch with the CPs to the remote
func (impl *defaultCPImplementation) pushFeatureBranch(
	state *State, opts *Options, featureBranch string,
) error {
	// Push the feature branch to the specified remote
	if err := command.NewWithWorkDir(
		opts.RepoPath, gitCommand, "push", opts.Remote, featureBranch,
	).RunSilentSuccess(); err != nil {
		return errors.Wrapf(err, "pushing branch %s to remote %s", featureBranch, opts.Remote)
	}
	logrus.Info(fmt.Sprintf("Successfully pushed %s to remote %s", featureBranch, opts.Remote))
	return nil
}
