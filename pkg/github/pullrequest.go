// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

const (
	REBASE = "rebase"
	MERGE  = "merge"
	SQUASH = "squash"
)

type PullRequest struct {
	impl                PRImplementation
	Merged              *bool
	MaintainerCanModify *bool
	MilestoneNumber     *int64
	MilestoneTitle      *string
	CreatedAt           time.Time
	RepoOwner           string
	RepoName            string
	FullName            string
	Username            string
	Ref                 string
	Sha                 string
	State               string
	BuildStatus         string
	BuildConclusion     string
	BuildLink           string
	URL                 string
	MergeCommitSHA      string `db:"-"`
	Labels              []string
	Number              int
	Repository          *Repository
}

type PRImplementation interface {
	loadRepository(*PullRequest) *Repository
	getMergeMode(ctx context.Context, pr *PullRequest, commits []*Commit) (mode string, err error)
	getCommits(ctx context.Context, pr *PullRequest) ([]*Commit, error)
}

// GetRepository returns the Repository object representing the
// repo where the PR was filed
func (pr *PullRequest) GetRepository() *Repository {
	if pr.Repository == nil {
		pr.impl.loadRepository(pr)
	}
	return pr.Repository
}

// GetMergeMode returns the way the pull request was merged
func (pr *PullRequest) GetMergeMode(ctx context.Context) (mode string, err error) {
	return pr.impl.getMergeMode(ctx, pr, []*Commit{})
}

// GetCommits returns the list of commits the pull request merged
// into its target branch
func (pr *PullRequest) GetCommits(ctx context.Context) ([]*Commit, error) {
	commits, err := pr.impl.getCommits(ctx, pr)
	if err != nil {
		return nil, errors.Wrapf(err, "reading commits from PR #%d", pr.Number)
	}
	return commits, nil
}
