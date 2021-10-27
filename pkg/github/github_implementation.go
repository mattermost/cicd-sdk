// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"

	gogithub "github.com/google/go-github/v39/github"
	"github.com/pkg/errors"
)

type defaultGithubImplementation struct {
	githubAPIUser
	client *gogithub.Client
}

func (di *defaultGithubImplementation) getPullRequestFromAPI(
	ctx context.Context, owner, repo string, number int,
) (*PullRequest, error) {
	ghpr, _, err := di.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, errors.Wrap(err, "getting PR from GitHub API")
	}

	pr := &PullRequest{
		RepoOwner:           ghpr.GetBase().GetRepo().GetOwner().GetLogin(),
		RepoName:            ghpr.GetBase().GetRepo().GetName(),
		Number:              ghpr.GetNumber(),
		Username:            ghpr.GetUser().GetLogin(),
		FullName:            ghpr.GetHead().GetRepo().GetFullName(),
		Ref:                 ghpr.GetHead().GetRef(),
		Sha:                 ghpr.GetHead().GetSHA(),
		State:               ghpr.GetState(),
		URL:                 ghpr.GetURL(),
		CreatedAt:           ghpr.GetCreatedAt(),
		Merged:              gogithub.Bool(ghpr.GetMerged()),
		MergeCommitSHA:      ghpr.GetMergeCommitSHA(),
		MaintainerCanModify: gogithub.Bool(ghpr.GetMaintainerCanModify()),
		MilestoneNumber:     gogithub.Int64(int64(ghpr.GetMilestone().GetNumber())),
		MilestoneTitle:      gogithub.String(ghpr.GetMilestone().GetTitle()),
	}

	return pr, nil
}
