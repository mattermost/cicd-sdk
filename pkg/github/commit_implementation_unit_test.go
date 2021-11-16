// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeTree(t *testing.T) {
	impl := defaultCommitImplementation{}

	// An empty file list should return an empty string
	require.Equal(t, impl.ChangeTree([]CommitFile{}), "")
	// One checksum
	require.Equal(t,
		"73177388d63ccb9c0821147d33e450f9d50771f45b67960d4d0ef033347e4de2",
		impl.ChangeTree([]CommitFile{{"file.txt", "e970302b4d2756c3e6133bde811c1cd25dd4936a"}}),
	)

	// Two elements
	require.Equal(t,
		"a757363387bfbcf8700c303809378f8fc9fcc0b868ce7c907527ef43762b946a",
		impl.ChangeTree([]CommitFile{
			{"file1.txt", "e970302b4d2756c3e6133bde811c1cd25dd4936a"},
			{"file2.txt", "69d69d92c2ac690c8de19365a46c9b4cb6ff3bf6"},
		}),
	)

	// Same, but inverted should yield same checksum
	require.Equal(t,
		"a757363387bfbcf8700c303809378f8fc9fcc0b868ce7c907527ef43762b946a",
		impl.ChangeTree([]CommitFile{
			{"file2.txt", "69d69d92c2ac690c8de19365a46c9b4cb6ff3bf6"},
			{"file1.txt", "e970302b4d2756c3e6133bde811c1cd25dd4936a"},
		}),
	)
}
