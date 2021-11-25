// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package backends

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/release-utils/hash"
)

func TestFileCopy(t *testing.T) {
	fs := NewFilesystemWithOptions(&Options{})
	tmp1, err := os.CreateTemp("", "test-fs-copy-")
	require.NoError(t, err)
	tmp2, err := os.CreateTemp("", "test-fs-copy-")
	require.NoError(t, err)

	defer func() {
		os.Remove(tmp1.Name())
		os.Remove(tmp2.Name())
	}()

	require.NoError(t, os.WriteFile(tmp1.Name(), []byte("Hola, test"), os.FileMode(0o755)))

	require.NoError(t, fs.CopyObject(tmp1.Name(), tmp2.Name()))
	hashValue, err := hash.SHA256ForFile(tmp2.Name())
	require.NoError(t, err)

	require.Equal(t, "4f1c9c524e24694bbcaafb91ed55e504f29bd2b6df67cdfb481e412a3816bb46", hashValue)
}
