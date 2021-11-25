package object

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/release-utils/hash"
)

func TestCopyLocal(t *testing.T) {
	om := NewManager()
	f, err := os.CreateTemp("", "test-copy-")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f2, err := os.CreateTemp("", "test-copy-")
	require.NoError(t, err)
	defer os.Remove(f2.Name())

	require.NoError(t, os.WriteFile(f.Name(), []byte("test data"), os.FileMode(0o644)))

	require.NoError(t, om.Copy("file:/"+f.Name(), "file:/"+f2.Name()))
	i, err := os.Stat(f2.Name())
	require.NoError(t, err)
	require.NotZero(t, i.Size())

	hash1, err := hash.SHA256ForFile(f.Name())
	require.NoError(t, err)
	hash2, err := hash.SHA256ForFile(f2.Name())
	require.NoError(t, err)
	require.Equal(t, hash1, hash2)
}

func TestCopyS3(t *testing.T) {
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	om := NewManager()

	source := "s3://pr-builds.mattermost.com/mattermost-cloud/account-alerts/commit/02913a9cf037c0c0eb3a31860dd6e99b2fc4c0d1/main.zip"
	destination, err := os.CreateTemp("", "test-copy-")
	require.NoError(t, err)
	defer os.Remove(destination.Name())

	require.NoError(t, om.Copy(source, "file:/"+destination.Name()))

	i, err := os.Stat(destination.Name())
	require.NoError(t, err)
	require.NotZero(t, i.Size())
	h256, err := hash.SHA256ForFile(destination.Name())
	require.NoError(t, err)
	require.Equal(t, "f105f49add53c638b5ba60f23c6bd39943251c05a33f18389f4a724a71e0e3a1", h256)
}
