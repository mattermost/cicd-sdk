package git

import (
	"fmt"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/command"
)

type Repository struct {
	impl   repositoryImplementation
	opts   *RepoOptions
	client *gogit.Repository
}

type RepoOptions struct {
	Path          string
	DefaultRemote string
}

var defaultRepositoryOptions = &RepoOptions{
	DefaultRemote: "origin",
}

func NewRepository() *Repository {
	return NewRepositoryWithOptions(defaultRepositoryOptions)
}

func NewRepositoryWithOptions(opts *RepoOptions) *Repository {
	return &Repository{
		impl: &defaultRepositoryImpl{},
		opts: opts,
	}
}

func (repo *Repository) SetClient(c *gogit.Repository) {
	repo.client = c
}

func (repo *Repository) CreateBranch(branchName string) error {
	return repo.impl.createBranch(repo.client, repo.opts, branchName)
}

// HasMergeConflicts returns a bool indicating if a merge conflict is on
func (repo *Repository) HasMergeConflicts() (hasConflicts bool, files []string, err error) {
	status, err := repo.impl.statusRaw(repo.opts)
	if err != nil {
		return false, nil, errors.Wrap(err, "getting repository status")
	}
	return repo.impl.hasMergeConflicts(repo.opts, status)
}

// Checkout checks out the reference named `refName` in the repository. Currently
// works with branches only
func (repo *Repository) Checkout(refName string) error {
	return repo.impl.checkout(repo.client, repo.opts, refName)
}

// CherryPickCommits cherry picks the commits in `commits` to a target branch
func (repo *Repository) CherryPickCommits(commits []string, targetBranch string) error {
	return repo.impl.cherryPickCommits(repo.client, repo.opts, commits, targetBranch)
}

func (repo *Repository) CherryPickMergeCommit(branch, commitSHA string, parent int) error {
	return repo.impl.cherryPickMergeCommit(repo.client, repo.opts, branch, commitSHA, parent)
}

func (repo *Repository) PushBranch(branch, remote string) error {
	return repo.impl.pushBranch(repo.client, repo.opts, branch, remote)
}

type repositoryImplementation interface {
	statusRaw(*RepoOptions) (string, error)
	createBranch(*gogit.Repository, *RepoOptions, string) error
	hasMergeConflicts(opts *RepoOptions, rawStatus string) (bool, []string, error)
	checkout(*gogit.Repository, *RepoOptions, string) error
	cherryPickCommits(client *gogit.Repository, opts *RepoOptions, commits []string, branch string) error
	pushBranch(client *gogit.Repository, opts *RepoOptions, branch, remote string) error
	cherryPickMergeCommit(client *gogit.Repository, opts *RepoOptions, branch, commitSHA string, parent int) error
}

type defaultRepositoryImpl struct{}

// statusRaw return the output of git status --porcelainto get the status of the
// repository. The output is return as is, no interpretation is done
func (di *defaultRepositoryImpl) statusRaw(opts *RepoOptions) (string, error) {
	// Check if the cp was halted due to unmerged commits
	output, err := command.NewWithWorkDir(
		opts.Path, gitCommand, "status", "--porcelain",
	).RunSuccessOutput()
	if err != nil {
		return "", errors.Wrap(err, "while trying to get repo status")
	}
	return output.Output(), nil
}

// createBranch creates a new Branch in the repo
func (di *defaultRepositoryImpl) createBranch(client *gogit.Repository, opts *RepoOptions, branchName string) error {
	logrus.Infof("Creating branch %s at %s", branchName, plumbing.NewBranchReferenceName(branchName))
	// nolint: gocritic
	/*
		// PUre go implementation. Not working yet
		if err := client.CreateBranch(&config.Branch{
			Name:   branchName,
			Merge:  plumbing.NewBranchReferenceName(branchName),
			Remote: opts.DefaultRemote,
		}); err != nil {
			return errors.Wrapf(err, "creating new branch %s", branchName)
		}
		logrus.Infof("Created branch %s", branchName)
	*/
	return errors.Wrap(
		command.NewWithWorkDir(opts.Path, gitCommand, "branch", branchName).RunSilentSuccess(),
		"creating branch",
	)
}

// hasMergeConflicts interprets a rawStatus to determine if
// files are unmerged suring a cherry pick or rebase
func (di *defaultRepositoryImpl) hasMergeConflicts(opts *RepoOptions, status string) (
	hasConflicts bool, files []string, err error,
) {
	files = []string{}
	hasConflicts = false
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "U") {
			logrus.Infof("conflicts detected, cannot merge:\n%s", status)
			hasConflicts = true
		}
	}

	// TODO: Parse files with conflicts

	return hasConflicts, files, nil
}

func (di *defaultRepositoryImpl) cherryPickCommits(
	client *gogit.Repository, opts *RepoOptions, commits []string, branch string,
) error {
	// First, checkout to the target branch
	if err := di.checkout(client, opts, branch); err != nil {
		return errors.Wrapf(err, "checking out branch %s", branch)
	}
	logrus.Infof("Cherry picking %d commits to branch %s", len(commits), branch)
	// go-git does not yet support cherry picking, so we call the shell:
	cmd := command.NewWithWorkDir(opts.Path, gitCommand, append([]string{"cherry-pick"}, commits...)...)
	if err := cmd.RunSilentSuccess(); err != nil {
		return errors.Wrap(err, "running git cherry-pick")
	}
	return nil
}

// cherrypickMergeCommit cherry picks a merge commit
func (di *defaultRepositoryImpl) cherryPickMergeCommit(
	client *gogit.Repository, opts *RepoOptions, branch string, commitSHA string, parent int,
) error {
	cmd := command.NewWithWorkDir(
		opts.Path, gitCommand, "cherry-pick", "-m", fmt.Sprintf("%d", parent), commitSHA,
	)
	return errors.Wrap(cmd.RunSuccess(), "running git cherry-pick")
}

// checkout calls the current worktree and checks out a reference. In the future this
// function should work with commits, tags and other objects, but currently it only
// works with
func (di *defaultRepositoryImpl) checkout(client *gogit.Repository, opts *RepoOptions, refName string) error {
	tree, err := client.Worktree()
	if err != nil {
		return errors.Wrap(err, "getting repository worktree")
	}
	// TODO: Check if reference is a hash and use that
	if err := tree.Checkout(
		&gogit.CheckoutOptions{
			Branch: plumbing.NewBranchReferenceName(refName),
		},
	); err != nil {
		return errors.Wrapf(err, "checking out %s", refName)
	}
	return nil
}

// pushBranch pushes a branch to a remote
func (di *defaultRepositoryImpl) pushBranch(
	client *gogit.Repository, opts *RepoOptions, branch, remote string,
) error {
	if remote == "" {
		remote = opts.DefaultRemote
		logrus.Infof("Using default remote %s as default for push", remote)
	}
	logrus.Infof("Pushing branch %s to %s", branch, remote)
	// Push the feature branch to the specified remote
	if err := command.NewWithWorkDir(
		opts.Path, gitCommand, "push", remote, branch,
	).RunSilentSuccess(); err != nil {
		return errors.Wrapf(err, "pushing branch %s to remote %s", branch, remote)
	}
	return nil
}
