// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

func NewCommit() *Commit {
	return &Commit{
		impl: defaultCommitImplementation{},
	}
}

type Commit struct {
	impl    CommitImplementation
	SHA     string    // SHA sum of the commit
	Parents []*Commit // Parent commits
	TreeSHA string    // SHA of the commmit's tree
}

type CommitImplementation interface {
}
