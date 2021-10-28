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

const githubTknVar = "GITHUB_TOKEN"

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
		tkn := os.Getenv(githubTknVar)
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

// NewPullRequest builds a PullRequest object from a gogithub PR object
func (gau *githubAPIUser) NewPullRequest(ghpr *gogithub.PullRequest) *PullRequest {
	return &PullRequest{
		impl:                &defaultPRImplementation{},
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
}

func (gau *githubAPIUser) NewRepository(ghrepo *gogithub.Repository) *Repository {
	return &Repository{
		impl:  &defaultRepoImplementation{},
		Owner: ghrepo.GetOwner().GetLogin(),
		Name:  ghrepo.GetName(),
	}
}

func (gau *githubAPIUser) NewIssue(ghissue *gogithub.Issue) *Issue {
	return &Issue{
		impl:      &defaultIssueImplementation{},
		RepoOwner: ghissue.GetRepository().GetOwner().GetLogin(),
		RepoName:  ghissue.GetRepository().GetName(),
		Number:    ghissue.GetNumber(),
		Username:  ghissue.GetUser().GetLogin(),
		State:     ghissue.GetState(),
	}
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
