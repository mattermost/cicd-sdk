// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

func NewPullRequest() *PullRequest {
	return &PullRequest{
		impl: &defaultPRImplementation{},
	}
}

// GetRepository returns the Repository object representing the
// repo where the PR was filed
func (pr *PullRequest) GetRepository(ctx context.Context) *Repository {
	if pr.Repository == nil {
		pr.impl.loadRepository(ctx, pr)
	}
	return pr.Repository
}

// GetMergeMode returns a string describing the way the pull request was merged
func (pr *PullRequest) GetMergeMode(ctx context.Context) (mode string, err error) {
	// Get the commits merged by the pull request
	commits, err := pr.impl.getRebaseCommits(ctx, pr)
	if err != nil {
		return "", errors.Wrapf(err, "getting commits from pull request #%d", pr.Number)
	}
	return pr.impl.getMergeMode(ctx, pr, commits)
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

// GetRebaseCommits returns the sequence of commits created when the PR
// was merged. It should only be used by rebased PRs.
func (pr *PullRequest) GetRebaseCommits(ctx context.Context) (commitSHAs []string, err error) {
	if pr.GetRepository(ctx) == nil {
		return nil, errors.New("unable to get rebase commits, pr repository is nil")
	}

	// First, the merge_commit_sha commit:
	branchCommit, err := pr.GetRepository(ctx).GetCommit(ctx, pr.MergeCommitSHA)
	if err != nil {
		return nil, errors.Wrap(err, "getting branch commit")
	}

	prCommits, err := pr.impl.getCommits(ctx, pr)
	if err != nil {
		return nil, errors.Wrap(err, "getting commits from PR")
	}

	commitSHAs = []string{}

	// Now, lets cycle and make sure we have the right SHAs
	for i := len(prCommits); i > 0; i-- {
		// Get the shas from the trees. They should match
		prTreeSHA := prCommits[i-1].ChangeTree()
		branchTreeSha := branchCommit.ChangeTree()
		if prTreeSHA != branchTreeSha {
			return nil, errors.Errorf(
				"Mismatch in PR and branch hashes in commit #%d PR:%s vs Branch:%s",
				i, prTreeSHA, branchTreeSha,
			)
		}

		logrus.Infof("Match #%d PR:%s vs Branch:%s", i, prTreeSHA, branchTreeSha)

		// Append the commit sha to the list (note not to use the *tree hash* here)
		commitSHAs = append(commitSHAs, branchCommit.SHA)

		// While we traverse the PR commits linearly, we follow
		// the git graph to get the neext commit int th branch
		branchCommit, err = pr.Repository.GetCommit(ctx, branchCommit.Parents[0])
		if err != nil {
			return nil, errors.Wrapf(
				err, "while fetching branch commit #%d - %s", i, branchCommit.Parents[0],
			)
		}
	}
	return commitSHAs, nil
}

// PatchTreeID return the parent ID of the pull request merge commit
func (pr *PullRequest) PatchTreeID(ctx context.Context) (parentNr int, err error) {
	return pr.impl.findPatchTree(ctx, pr)
}
