// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import "github.com/sirupsen/logrus"

func NewCommit() *Commit {
	return &Commit{
		impl:    &defaultCommitImplementation{},
		Parents: []string{},
		Files:   []CommitFile{},
	}
}

type Commit struct {
	impl    CommitImplementation
	SHA     string       // SHA sum of the commit
	TreeSHA string       // SHA of the commmit's tree
	Parents []string     // SHAs of parent commits
	Files   []CommitFile // List of files modified in this commit
}

// CommitFile abstracts a file changed in a commit
type CommitFile struct {
	Filename string
	SHA      string
}

// ChangeTree creates a sha1 sum of the changed files
func (c *Commit) ChangeTree() string {
	logrus.Infof("Checksumming %d files in commit %s", len(c.Files), c.SHA)
	return c.impl.ChangeTree(c.Files)
}

type CommitImplementation interface {
	ChangeTree([]CommitFile) string
}
