// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import "context"

type Repository struct {
	impl                       repositoryImplementation
	Owner                      string
	Name                       string
	BuildStatusContext         string
	InstanceSetupScript        string
	InstanceSetupUpgradeScript string
	JobName                    string
	GreetingTeam               string   // GreetingTeam is the GitHub team responsible for triaging non-member PRs for this repo.
	GreetingLabels             []string // GreetingLabels are the labels applied automatically to non-member PRs for this repo.

}

func NewRepository(owner, name string) *Repository {
	return &Repository{
		Owner: owner,
		Name:  name,
		impl:  &defaultRepoImplementation{},
	}
}

type repositoryImplementation interface {
	getPullRequest(ctx context.Context, owner, repo string, number int) (pr *PullRequest, err error)
	getCommit(ctx context.Context, owner string, repo string, sha string) (commit *Commit, err error)
	createPullRequest(
		ctx context.Context, owner, repo, head, base, title, body string, opts *NewPullRequestOptions,
	) (*PullRequest, error)
}

type NewPullRequestOptions struct {
	MaintainerCanModify bool
}

// CreatePullRequest creates a new pull request in the repository
func (repo *Repository) CreatePullRequest(
	ctx context.Context, head, base, title, body string, opts *NewPullRequestOptions,
) (*PullRequest, error) {
	// Call the create new PR function
	return repo.impl.createPullRequest(
		ctx, repo.Owner, repo.Name, head, base, title, body, opts,
	)
}

// GetCommit fteches from the repository the commit at sha
func (repo *Repository) GetCommit(ctx context.Context, sha string) (c *Commit, err error) {
	return repo.impl.getCommit(ctx, repo.Owner, repo.Name, sha)
}

func (repo *Repository) GetPullRequest(ctx context.Context, number int) (pr *PullRequest, err error) {
	return repo.impl.getPullRequest(ctx, repo.Owner, repo.Name, number)
}
