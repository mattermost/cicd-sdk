// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package backends

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/release-utils/util"
)

func TestGitRemoteCopy(t *testing.T) {
	g := NewGitWithOptions(&Options{})

	// "Copy" a git backend to a temporary directory
	dir, err := os.MkdirTemp("", "git-backend-test-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	require.NoError(t, g.copyRemoteToLocal("git+https://github.com/mattermost/cicd-sdk.git", "file:/"+dir))
	require.NoError(t, err)

	// Commit 61781b88e2aa98de64860ac2fd14384bf0224f53 was the last point where
	// the replacement code was still in pkg/build. If we check that specific version,
	// we should find the go code:

	// "Copy" a git backend to a temporary directory
	dir2, err := os.MkdirTemp("", "git-backend-test-")
	require.NoError(t, err)
	defer os.RemoveAll(dir2)
	require.NoError(t, g.copyRemoteToLocal("git+https://github.com/mattermost/cicd-sdk.git@61781b88e2aa98de64860ac2fd14384bf0224f53", "file:/"+dir2))
	require.NoError(t, err)
	require.True(t, util.Exists(filepath.Join(dir2, "pkg/build/replacement.go")))
}
