// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
)

type defaultCommitImplementation struct {
	githubAPIUser
}

// ChangeTree creates a checksum of the changes in the commit
func (di *defaultCommitImplementation) ChangeTree(files []CommitFile) string {
	if len(files) == 0 {
		return ""
	}
	hashes := []string{}

	for _, f := range files {
		hashes = append(hashes, f.SHA)
	}
	sort.Strings(hashes)
	h := sha256.New()
	h.Write([]byte(strings.Join(hashes, ":")))
	return fmt.Sprintf("%x", h.Sum(nil))
}
