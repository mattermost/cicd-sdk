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

// TestFileHash tests creating an object (a file) an returning its hashes
func TestFileHash(t *testing.T) {
	f, err := os.CreateTemp("", "object-hashing-")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	require.NoError(t, os.WriteFile(f.Name(), []byte("testing, 123"), os.FileMode(0o644)))

	fs := NewFilesystemWithOptions(&Options{})
	h, err := fs.GetObjectHash(f.Name())
	require.NoError(t, err)
	require.Len(t, h, 3)
	require.Equal(t, h, map[string]string{
		"sha1":   "0a0bc4f7c602c43b8ada179dc0e28e6ad703b966",
		"sha256": "dd86307859bd3a3b5a2d03540b9679d269a400af146798e179ae3171751511a9",
		"sha512": "39456c46b5bb4a2e764452241d4104e155fad4d98ccc3070baec57b6d7bc03a1ac081b6ab928f1719c7c7d81190da3ce5434466f71ee66887420c4406d68f7b9",
	})
}
