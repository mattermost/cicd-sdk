// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"
	"net/http"
	"os"

	gogithub "github.com/google/go-github/v39/github"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

const GITHUB_TOKEN = "GITHUB_TOKEN"

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

type githubAPIUser struct {
	client *gogithub.Client
}

// getGoGitHubClient returns a go-github client. If the environment
// contains a GitHub token, the client will use it for authentication
func (gau *githubAPIUser) GitHubClient() *gogithub.Client {
	if gau.client == nil {
		httpClient := http.DefaultClient
		tkn := os.Getenv(GITHUB_TOKEN)
		if tkn == "" {
			logrus.Warn("Note: GitHub client will not be authenticated")
		} else {
			httpClient = oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: tkn},
			))
		}
		gau.client = gogithub.NewClient(httpClient)
	}
	return gau.client
}

func (gau *githubAPIUser) NewCommit(commit *gogithub.Commit) *Commit {
	c := &Commit{
		SHA:     commit.GetSHA(),
		Parents: []*Commit{},
		TreeSHA: commit.GetTree().GetSHA(),
	}

	for _, parent := range commit.Parents {
		c.Parents = append(c.Parents, gau.NewCommit(parent))
	}
	return c
}

type Options struct {
}

var defaultOptions = Options{}

type githubImplementation interface {
	getPullRequestFromAPI(ctx context.Context, owner, repo string, number int) (*PullRequest, error)
}

// GetPullRequest fetches a PR from github
func (gh *GitHub) GetPullRequest(ctx context.Context, owner, repo string, number int) (*PullRequest, error) {
	return gh.impl.getPullRequestFromAPI(ctx, owner, repo, number)
}
