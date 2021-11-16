// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

// githubAPIUser is a type menat to be embedded in all objects that need to
// perform calls to the GitHub API

package github

import (
	"context"
	"net/http"
	"os"

	gogithub "github.com/google/go-github/v39/github"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

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

func (gau *githubAPIUser) NewCommit(rcommit *gogithub.RepositoryCommit) *Commit {
	c := NewCommit()
	c.SHA = rcommit.GetSHA()
	c.TreeSHA = rcommit.Commit.GetTree().GetSHA()

	// Circle the commit's parents and record the hashes
	for _, parent := range rcommit.Parents {
		c.Parents = append(c.Parents, parent.GetSHA())
	}

	// Add the changed files to the commit
	for _, f := range rcommit.Files {
		c.Files = append(c.Files, struct {
			Filename string
			SHA      string
		}{*f.Filename, *f.SHA})
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
		Title:     ghissue.GetTitle(),
		Body:      ghissue.GetBody(),
		RepoOwner: ghissue.GetRepository().GetOwner().GetLogin(),
		RepoName:  ghissue.GetRepository().GetName(),
		Number:    ghissue.GetNumber(),
		Username:  ghissue.GetUser().GetLogin(),
		State:     ghissue.GetState(),
	}
}
