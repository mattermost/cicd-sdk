// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"

	gogithub "github.com/google/go-github/v39/github"
	"github.com/pkg/errors"
)

type defaultRepoImplementation struct {
	githubAPIUser
}

func (di *defaultRepoImplementation) getCommit(ctx context.Context, owner, repo, sha string) (*Commit, error) {
	repoCommit, _, err := di.githubAPIUser.GitHubClient().Repositories.GetCommit(ctx, owner, repo, sha, &gogithub.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "fetching commit from github API")
	}
	return di.githubAPIUser.NewCommitFromRepoCommit(repoCommit), nil
}

// getPullRequest pulls a PR from the GitHub API and return a PullRequest object
func (di *defaultRepoImplementation) getPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	ghPr, _, err := di.githubAPIUser.GitHubClient().PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching PR #%d from github api", number)
	}

	return di.githubAPIUser.NewPullRequest(ghPr), nil
}

// getIssue queries github for an issue and return the
func (di *defaultRepoImplementation) getIssue(ctx context.Context, owner, repo string, number int) (*Issue, error) {
	ghIssue, _, err := di.githubAPIUser.GitHubClient().Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching issue #%d from github api", number)
	}

	i := di.githubAPIUser.NewIssue(ghIssue)
	i.RepoName = repo
	i.RepoOwner = owner
	return i, nil
}

func (di *defaultRepoImplementation) createPullRequest(
	ctx context.Context, owner, repo, head, base, title, body string, opts *NewPullRequestOptions,
) (*PullRequest, error) {
	newPullRequest := &gogithub.NewPullRequest{
		Head:                &head,
		Base:                &base,
		Body:                &body,
		Title:               &title,
		MaintainerCanModify: &opts.MaintainerCanModify,
	}
	pullrequest, _, err := di.githubAPIUser.GitHubClient().PullRequests.Create(ctx, owner, repo, newPullRequest)
	if err != nil {
		return nil, errors.Wrap(err, "creating pull request")
	}

	return di.githubAPIUser.NewPullRequest(pullrequest), nil
}
