// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"
	"fmt"

	gogithub "github.com/google/go-github/v39/github"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type PRImplementation interface {
	loadRepository(context.Context, *PullRequest)
	getMergeMode(ctx context.Context, pr *PullRequest, commits []*Commit) (mode string, err error)
	getCommits(ctx context.Context, pr *PullRequest) ([]*Commit, error)
	findPatchTree(ctx context.Context, pr *PullRequest) (parentNr int, err error)
	getRebaseCommits(ctx context.Context, pr *PullRequest) (commits []*Commit, err error)
}

type defaultPRImplementation struct {
	githubAPIUser
}

// loadRepository  returns the repo where the PR lives
func (impl *defaultPRImplementation) loadRepository(ctx context.Context, pr *PullRequest) {
	ghRepo, _, err := impl.githubAPIUser.GitHubClient().Repositories.Get(ctx, pr.RepoOwner, pr.RepoName)
	if err != nil {
		logrus.Error(err)
		return
	}
	pr.Repository = impl.githubAPIUser.NewRepository(ghRepo)
}

// GetMergeMode implements an algo to try and determine how the PR was
// merged. It should work for most cases except in single commit PRs
// which have been squashed or rebased, but for practical purposes this
// edge case in non relevant.
//
// The PR commits must be fetched beforehand and passed to this function
// to be able to mock it properly.
func (impl *defaultPRImplementation) getMergeMode(
	ctx context.Context, pr *PullRequest, commits []*Commit,
) (mode string, err error) {
	if pr.GetRepository(ctx) == nil {
		return "", errors.New("unable to get merge mode, pull request has no repo")
	}

	if pr.MergeCommitSHA == "" {
		return "", errors.New("unable to get merge mode, pr does not have merge commit SHA")
	}

	// Fetch the PR data from the github API
	mergeCommit, err := pr.GetRepository(ctx).GetCommit(ctx, pr.MergeCommitSHA)
	if err != nil {
		return "", errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}
	if mergeCommit == nil {
		return "", errors.Errorf("commit returned empty when querying sha %s", pr.MergeCommitSHA)
	}

	// If the SHA commit has more than one parent, it is definitely a merge commit.
	if len(mergeCommit.Parents) > 1 {
		logrus.Infof("PR #%d merged via a merge commit", pr.Number)
		return MERGE, nil
	}

	// A special case: if the PR only has one commit, we cannot tell if it was rebased or
	// squashed. We return "squash" preemptibly to avoid recomputing trees unnecessarily.
	if len(commits) == 1 {
		logrus.Infof("Considering PR #%d as squash as it only has one commit", pr.Number)
		return SQUASH, nil
	}

	// Now, to be able to determine if the PR was squashed, we have to compare the trees
	// of `merge_commit_sha` and the last commit in the PR.
	//
	// In both cases (squashed and rebased) the sha in that field *is not a merge commit*:
	//  * If the PR was squashed, the sha will point to the single resulting commit.
	//  * If the PR was rebased, it will point to the last commit in the sequence
	//
	// If we compare the tree in `merge_commit_sha` and it matches the tree in the last
	// commit in the PR, then we are looking at a rebase.
	//
	// If the tree in the `merge_commit_sha` commit is different from the last commit,
	// then the PR was squashed (thus generating a new tree of al commits combined).

	// Fetch trees from both the merge commit and the last commit in the PR
	mergeTree := mergeCommit.ChangeTree()
	prTree := commits[len(commits)-1].ChangeTree()

	logrus.Infof("Merge tree: %s - PR tree: %s", mergeTree, prTree)

	// Compare the tree shas...
	if mergeTree == prTree {
		// ... if they match the PR was rebased
		logrus.Info(fmt.Sprintf("PR #%d was merged via rebase", pr.Number))
		return REBASE, nil
	}

	// Otherwise it was squashed
	logrus.Info(fmt.Sprintf("PR #%d was merged via squash", pr.Number))
	return SQUASH, nil
}

// getCommits returns the commits of the PR. These are not the merged
// commits. The trees from these are copied to the branch when the PR
// is merged. THis means the SHAs change but the tree ids do not.
func (impl *defaultPRImplementation) getCommits(ctx context.Context, pr *PullRequest) ([]*Commit, error) {
	// Todo: Fixme read response and add retries
	commitList, _, err := impl.githubAPIUser.GitHubClient().PullRequests.ListCommits(
		ctx, pr.RepoOwner, pr.RepoName, pr.Number, &gogithub.ListOptions{},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for commits in PR %d", pr.Number)
	}

	list := []*Commit{}
	for _, ghCommit := range commitList {
		ghcommit2, _, err := impl.GitHubClient().Repositories.GetCommit(
			ctx, pr.RepoOwner, pr.RepoName, ghCommit.GetSHA(), &gogithub.ListOptions{},
		)
		if err != nil {
			return nil, errors.Wrapf(err, "querying GitHub for commit %s", ghCommit.GetSHA())
		}
		if ghcommit2 == nil {
			return nil, errors.Errorf("commit returned empty when querying sha %s", ghCommit.GetSHA())
		}
		list = append(list, impl.githubAPIUser.NewCommit(ghcommit2))
	}

	logrus.Info(fmt.Sprintf("Read %d commits from PR %d", len(commitList), pr.Number))
	return list, nil
}

// findPatchTree analyzes the parents of the PR's merge commit and
// returns the parent ID whose tree should be used to generate diff for
// the cherry pick.
//
// A merge commit has a Patch Tree and a Branch Tree (correct these names)
// if there is another, more official or appropriate nomenclature.
func (impl *defaultPRImplementation) findPatchTree(
	ctx context.Context, pr *PullRequest,
) (parentNr int, err error) {
	// Get the pull request commits
	commits, err := impl.getCommits(ctx, pr)
	if err != nil {
		return 0, errors.Wrap(err, "getting pr commits")
	}
	if len(commits) == 0 {
		return 0, errors.New("unable to find patch tree, commit list is empty")
	}

	// They way to find out which tree to use is to search the tree from
	// the last commit in the PR. The tree sha in the PR commit will match
	// the tree in the PR parent

	// Get the commit information
	repoCommit, _, err := impl.GitHubClient().Repositories.GetCommit(
		ctx, pr.RepoOwner, pr.RepoName, pr.MergeCommitSHA, &gogithub.ListOptions{},
	)
	if err != nil {
		return 0, errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}
	if repoCommit == nil {
		return 0, errors.Errorf("commit returned empty when querying sha %s", pr.MergeCommitSHA)
	}

	mergeCommit := impl.githubAPIUser.NewCommit(repoCommit)
	if len(mergeCommit.Parents) == 0 {
		return 0, errors.Errorf("commit %s has no parents defined", mergeCommit.SHA)
	}
	// First, get the tree hash from the last commit in the PR
	prSHA := commits[len(commits)-1].TreeSHA

	// Now, cycle the parents, fetch their commits and see which one matches
	// the tree hash extracted from the commit
	// TODO: mergeCommit.GetParents()
	for pn, parent := range mergeCommit.Parents {
		parentCommit, _, err := impl.GitHubClient().Repositories.GetCommit(
			ctx, pr.RepoOwner, pr.RepoName, parent, &gogithub.ListOptions{})
		if err != nil {
			return 0, errors.Wrapf(err, "querying GitHub for parent commit %s", parent)
		}
		if parentCommit == nil {
			return 0, errors.Errorf("commit returned empty when querying sha %s", parent)
		}

		parentTreeSHA := parentCommit.Commit.GetTree().GetSHA()
		logrus.Info(fmt.Sprintf("PR: %s - Parent: %s", prSHA, parentTreeSHA))
		if parentTreeSHA == prSHA {
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
// that match each modifications in the pull request's commit.
// Remember: The commits in the PR are not the same commits in
// the branch but their trees hashes must match
func (impl *defaultPRImplementation) getRebaseCommits(
	ctx context.Context, pr *PullRequest) (commits []*Commit, err error) {
	// To find the commits, we take the last commit from the PR.
	// The patch should match the commit int the pr `merge_commit_sha` field.
	// From there we navigate backwards in the history ensuring all commits match
	// patches from all commits.

	repo := &defaultRepoImplementation{}
	prCommits, err := impl.getCommits(ctx, pr)
	if err != nil {
		return nil, errors.Wrap(err, "fetching commits from pr")
	}

	// First, the merge_commit_sha commit:
	branchCommit, err := repo.getCommit(ctx, pr.RepoOwner, pr.RepoName, pr.MergeCommitSHA)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}
	if len(branchCommit.Parents) == 0 {
		return nil, errors.New("branch commit has no parents")
	}

	commits = []*Commit{}

	// Now, lets cycle and make sure we have the right SHAs
	for i := len(prCommits); i > 0; i-- {
		// Get the SHAs from the change. They should match
		prTreeSHA := prCommits[i-1].ChangeTree()
		branchTreeSHA := branchCommit.ChangeTree()
		if prTreeSHA != branchTreeSHA {
			return nil, errors.Errorf(
				"Mismatch in checktrees on commit #%d PR:%s vs Branch:%s",
				i, prTreeSHA, branchTreeSHA,
			)
		}

		logrus.Debugf("Match #%d PR:%s vs Branch:%s", i, prTreeSHA, branchTreeSHA)

		// Append the commit sha to the list (note not to use the *tree hash* here)
		commits = append(commits, branchCommit)
		// While we traverse the PR commits linearly, we follow
		// the git graph to get the next commit in the branch
		parentSHA := branchCommit.Parents[0]
		branchCommit, err = repo.getCommit(
			ctx, pr.RepoOwner, pr.RepoName, parentSHA,
		)
		if err != nil {
			return nil, errors.Wrapf(
				err, "while fetching branch commit (prent #%d) %s", i, parentSHA,
			)
		}
	}

	// Reverse the list of shas to preserve the PR order
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}

	return commits, nil
}
