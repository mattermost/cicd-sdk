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

	source := "s3://devs.mattermost.com/index.html"
	destination, err := os.CreateTemp("", "test-copy-")
	require.NoError(t, err)
	defer os.Remove(destination.Name())

	require.NoError(t, om.Copy(source, "file:/"+destination.Name()))

	i, err := os.Stat(destination.Name())
	require.NoError(t, err)
	require.NotZero(t, i.Size())
	h256, err := hash.SHA256ForFile(destination.Name())
	require.NoError(t, err)
	require.Equal(t, "96010d7a5d77a839b14a82deb526c6ad638b0c16bca1cf12e47a9e3de47a385d", h256)
}
