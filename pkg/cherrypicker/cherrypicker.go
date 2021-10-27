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
	readPRcommits(context.Context, *State, *Options, *github.PullRequest) ([]*github.Commit, error)
	createBranch(*State, *Options, string, *github.PullRequest) (string, error)
	cherrypickCommits(*State, *Options, string, []string) error
	cherrypickMergeCommit(*State, *Options, string, []string, int) error
	getPRMergeMode(context.Context, *State, *Options, *github.PullRequest, []*github.Commit) (string, error)
	findCommitPatchTree(context.Context, *State, *Options, *github.PullRequest, []*github.Commit) (int, error)
	GetRebaseCommits(context.Context, *State, *Options, *github.PullRequest, []*github.Commit) ([]string, error)
	getPullRequest(context.Context, *State, *Options, int) (*github.PullRequest, error)
	createPullRequest(context.Context, *State, *Options, *github.PullRequest, string, string) (*github.PullRequest, error)
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
	pr, err := cp.impl.getPullRequest(ctx, &cp.state, &cp.options, prNumber)
	if err != nil {
		return errors.Wrapf(err, "getting pull request %d", prNumber)
	}

	// The first thing we need to create the CPs is to pull the commits
	// from the pull request
	commits, err := cp.impl.readPRcommits(ctx, &cp.state, &cp.options, pr)
	if err != nil {
		return errors.Wrapf(err, "reading commits from PR #%d", pr.Number)
	}

	// Next step: Find out how the PR was merged
	mergeMode, err := cp.impl.getPRMergeMode(ctx, &cp.state, &cp.options, pr, commits)
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
		parent, err2 := cp.impl.findCommitPatchTree(ctx, &cp.state, &cp.options, pr, commits)
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
		rebaseCommits, err2 := cp.impl.GetRebaseCommits(ctx, &cp.state, &cp.options, pr, commits)
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
	pullrequest, err := cp.impl.createPullRequest(
		ctx, &cp.state, &cp.options, pr, featureBranch, branch,
	)
	if err != nil {
		return errors.Wrap(err, "creating pull request in github")
	}

	logrus.Info(fmt.Sprintf("Successfully created pull request #%d", pullrequest.Number))

	return nil
}

type defaultCPImplementation struct{}

// readPRcommits returns the SHAs of all commits in a PR
func (impl *defaultCPImplementation) readPRcommits(
	ctx context.Context, state *State, opts *Options, pr *github.PullRequest,
) (commitList []*github.Commit, err error) {
	// Fixme read response and add retries
	commitList, _, err = state.github.ListCommits(
		ctx, pr.RepoOwner, pr.RepoName, pr.Number,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for commits in PR %d", pr.Number)
	}

	logrus.Info(fmt.Sprintf("Read %d commits from PR %d", len(commitList), pr.Number))
	return commitList, nil
}

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

// findCommitPatchTree analyzes the parents of a merge commit and
// returns the parent ID whose treee will be used to generate the
// diff for the cherry pick.
func (impl defaultCPImplementation) findCommitPatchTree(
	ctx context.Context, state *State, opts *Options,
	pr *github.PullRequest, commits []*github.Commit,
) (parentNr int, err error) {
	if len(commits) == 0 {
		return 0, errors.New("unable to find patch tree, commit list is empty")
	}
	// They way to find out which tree to use is to search the tree from
	// the last commit in the PR. The tree sha in the PR commit will match
	// the tree in the PR parent

	// Get the commit information
	mergeCommit, err := state.repo.GetCommit(ctx, pr.MergeCommitSHA)
	if err != nil {
		return 0, errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}
	if mergeCommit == nil {
		return 0, errors.Errorf("commit returned empty when querying sha %s", pr.MergeCommitSHA)
	}

	// First, get the tree hash from the last commit in the PR
	prTree := commits[len(commits)-1].GetCommit().GetTree()
	prSHA := prTree.GetSHA()

	// Now, cycle the parents, fetch their commits and see which one matches
	// the tree hash extracted from the commit
	for pn, parent := range mergeCommit.Parents {
		parentCommit, err := state.repo.GetCommit(ctx, parent.SHA)
		if err != nil {
			return 0, errors.Wrapf(err, "querying GitHub for parent commit %s", parent.GetSHA())
		}
		if parentCommit == nil {
			return 0, errors.Errorf("commit returned empty when querying sha %s", parent.GetSHA())
		}

		parentTree := parentCommit.Commit.GetTree()
		logrus.Info(fmt.Sprintf("PR: %s - Parent: %s", prSHA, parentTree.GetSHA()))
		if parentTree.GetSHA() == prSHA {
			logrus.Info(fmt.Sprintf("Cherry pick to be performed diffing the parent #%d tree ", pn))
			return pn, nil
		}
	}

	// If not found, we return an error to make sure we don't use 0
	return 0, errors.Errorf(
		"unable to find patch tree of merge commit among %d parents", len(mergeCommit.Parents),
	)
}

// GetRebaseCommits searches for the commits in the branch history
// that match the modifications in the pull request
func (impl *defaultCPImplementation) GetRebaseCommits(
	ctx context.Context, state *State, opts *Options,
	pr *github.PullRequest, prCommits []*github.RepositoryCommit) (commitSHAs []string, err error) {
	// To find the commits, we take the last commit from the PR.
	// The patch should match the commit int the pr `merge_commit_sha` field.
	// From there we navigate backwards in the history ensuring all commits match
	// patches from all commits.

	// First, the merge_commit_sha commit:
	branchCommit, err := impl.getCommit(
		ctx, state,
		pr.GetBase().GetRepo().GetOwner().GetLogin(),
		pr.GetBase().GetRepo().GetName(),
		pr.GetMergeCommitSHA(),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}

	commitSHAs = []string{}

	// Now, lets cycle and make sure we have the right SHAs
	for i := len(prCommits); i > 0; i-- {
		// Get the shas from the trees. They should match
		prTreeSHA := prCommits[i-1].GetCommit().GetTree().GetSHA()
		branchTreeSha := branchCommit.GetCommit().GetTree().GetSHA()
		if prTreeSHA != branchTreeSha {
			return nil, errors.Errorf(
				"Mismatch in PR and branch hashed in commit #%d PR:%s vs Branch:%s",
				i, prTreeSHA, branchTreeSha,
			)
		}

		logrus.Info(fmt.Sprintf("Match #%d PR:%s vs Branch:%s", i, prTreeSHA, branchTreeSha))

		// Append the commit sha to the list (note not to use the *tree hash* here)
		commitSHAs = append(commitSHAs, branchCommit.GetSHA())

		// While we traverse the PR commits linearly, we follow
		// the git graph to get the neext commit int th branch
		branchCommit, err = impl.getCommit(
			ctx, state,
			pr.GetBase().GetRepo().GetOwner().GetLogin(),
			pr.GetBase().GetRepo().GetName(),
			branchCommit.Parents[0].GetSHA(),
		)
		if err != nil {
			return nil, errors.Wrapf(
				err, "while fetching branch commit #%d - %s", i, branchCommit.Parents[0].GetSHA(),
			)
		}
	}

	// Reverse the list of shas to preserve the PR order
	for i, j := 0, len(commitSHAs)-1; i < j; i, j = i+1, j-1 {
		commitSHAs[i], commitSHAs[j] = commitSHAs[j], commitSHAs[i]
	}

	return commitSHAs, nil
}

// getCommit gets info about a commit from the github API
func (impl *defaultCPImplementation) getCommit(
	ctx context.Context, state *State, owner, repo, commitSHA string,
) (cmt *github.Commit, err error) {
	// Get the commit from the API:
	cmt, _, err = state.github.Repositories.GetCommit(ctx, owner, repo, commitSHA)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for commit %s", commitSHA)
	}
	if cmt == nil {
		return nil, errors.Errorf("commit returned empty when querying sha %s", commitSHA)
	}

	return cmt, nil
}

// getPullRequest fetches a pull request from GitHub
func (impl *defaultCPImplementation) getPullRequest(
	ctx context.Context, state *State, opts *Options, prNumber int,
) (*github.PullRequest, error) {
	pr, _, err := state.github.PullRequests.Get(
		ctx, opts.RepoOwner, opts.RepoName, prNumber)
	if err != nil {
		return nil, errors.Wrapf(err, "getting pull request %d from GitHub", prNumber)
	}
	return pr, nil
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

// createPullRequest cresates the cherry-picks pull request
func (impl *defaultCPImplementation) createPullRequest(
	ctx context.Context, state *State, opts *Options, pr *github.PullRequest, featureBranch, baseBranch string,
) (*github.PullRequest, error) {
	// We will pass the branchname to git
	headBranchName := featureBranch
	// Unless a fork is defined in the options. IN this case we append the fork Org
	// to the branch and use that as the head branch
	if opts.ForkOwner != "" {
		headBranchName = fmt.Sprintf("%s:%s", opts.ForkOwner, featureBranch)
	}
	title := fmt.Sprintf(prTitleTemplate, pr.Number, baseBranch)
	body := fmt.Sprintf(prBodyTemplate, pr.Number, baseBranch, pr.Number, baseBranch, pr.Username)
	newPullRequest := &github.NewPullRequest{
		Title:               &title,
		Head:                &headBranchName,
		Base:                &baseBranch,
		Body:                &body,
		MaintainerCanModify: github.Bool(true),
	}

	// Send the PR to GItHub:
	pullrequest, _, err := state.github.CreatePullRequest(ctx, opts.RepoOwner, opts.RepoName, newPullRequest)
	if err != nil {
		return pullrequest, errors.Wrap(err, "creating pull request")
	}

	return pullrequest, nil
}
