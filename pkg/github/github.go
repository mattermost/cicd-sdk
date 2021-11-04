// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"
)

const (
	githubTknVar = "GITHUB_TOKEN"
	MMREBASE     = "rebase"
	MMMERGE      = "merge"
	MMSQUASH     = "squash"
)

type GitHub struct {
	impl    githubImplementation
	options *Options
}

// New returns a new GitHub client
func New() *GitHub {
	return NewWithOptions(&defaultOptions)
}

func NewWithOptions(opts *Options) *GitHub {
	gh := &GitHub{
		impl:    &defaultGithubImplementation{},
		options: opts,
	}
	return gh
}

type Options struct{}

var defaultOptions = Options{}

type githubImplementation interface {
	getPullRequestFromAPI(ctx context.Context, owner, repo string, number int) (*PullRequest, error)
}

// GetPullRequest fetches a PR from github
func (gh *GitHub) GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	return gh.impl.getPullRequestFromAPI(ctx, owner, repo, number)
}
