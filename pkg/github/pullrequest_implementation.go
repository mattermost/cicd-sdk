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

type defaultPRImplementation struct {
	githubAPIUser
}

// loadRepository  returns the repo where the PR lives
func (impl *defaultPRImplementation) loadRepository(pr *PullRequest) {
	logrus.Fatal("Not implemented")
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

	if pr.GetRepository() == nil {
		return "", errors.New("unable to get merge mode, pull request has no repo")
	}

	// Fetch the PR data from the github API
	mergeCommit, err := pr.GetRepository().GetCommit(ctx, pr.MergeCommitSHA)
	if err != nil {
		return "", errors.Wrapf(err, "querying GitHub for merge commit %s", pr.MergeCommitSHA)
	}
	if mergeCommit == nil {
		return "", errors.Errorf("commit returned empty when querying sha %s", pr.MergeCommitSHA)
	}

	// If the SHA commit has more than one parent, it is definitely a merge commit.
	if len(mergeCommit.Parents) > 1 {
		logrus.Info(fmt.Sprintf("PR #%d merged via a merge commit", pr.Number))
		return MERGE, nil
	}

	// A special case: if the PR only has one commit, we cannot tell if it was rebased or
	// squashed. We return "squash" preemptibly to avoid recomputing trees unnecessarily.
	if len(commits) == 1 {
		logrus.Info(fmt.Sprintf("Considering PR #%d as squash as it only has one commit", pr.Number))
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
	mergeTree := mergeCommit.TreeSHA
	prTree := commits[len(commits)-1].TreeSHA

	logrus.Info(fmt.Sprintf("Merge tree: %s - PR tree: %s", mergeTree, prTree))

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

// getCommits returns the commits of the PR
func (impl *defaultPRImplementation) getCommits(ctx context.Context, pr *PullRequest) ([]*Commit, error) {
	// Fixme read response and add retries
	commitList, _, err := impl.githubAPIUser.GitHubClient().PullRequests.ListCommits(
		ctx, pr.RepoOwner, pr.RepoName, pr.Number, &gogithub.ListOptions{},
	)
	if err != nil {
		return nil, errors.Wrapf(err, "querying GitHub for commits in PR %d", pr.Number)
	}

	list := []*Commit{}
	for _, ghCommit := range commitList {
		list = append(list, impl.githubAPIUser.NewCommit(ghCommit.Commit))
	}

	logrus.Info(fmt.Sprintf("Read %d commits from PR %d", len(commitList), pr.Number))
	return list, nil
}
