// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import "context"

type Repository struct{}

func (r *Repository) CreatePullRequest() {
}

func (r *Repository) GetCommit(ctx context.Context, sha string) (c *Commit, err error) {
	return c, nil
}
