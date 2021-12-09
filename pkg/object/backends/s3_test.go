package backends

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestS3PathExists(t *testing.T) {
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	s3 := NewS3WithOptions(&Options{})

	// File exists:
	e, err := s3.PathExists("s3://devs.mattermost.com/index.html")
	require.NoError(t, err)
	require.True(t, e)

	// File does not exist:
	e2, err2 := s3.PathExists("s3://devs.mattermost.com/nonexistent-index.html")
	require.NoError(t, err2)
	require.False(t, e2)
}

func TestS3Hash(t *testing.T) {
	os.Setenv("AWS_DEFAULT_REGION", "us-east-1")
	s3 := NewS3WithOptions(&Options{})

	// File exists:
	h, err := s3.GetObjectHash("s3://devs.mattermost.com/index.html")
	require.NoError(t, err)

	require.Len(t, h, 3)
	require.Equal(t, h, map[string]string{
		"sha1":   "27c744bd079754498e078e830b07cbcdb9a3eb8e",
		"sha256": "96010d7a5d77a839b14a82deb526c6ad638b0c16bca1cf12e47a9e3de47a385d",
		"sha512": "1d5fe438ec97daf208d9e34cb9814834d40c540f65096e6ff5fcc19ac1c3084bfc05bbc911ceb32271089a8f71c8dc9eabf2c7b8146f79a6596e38ff8ee36f2a",
	})
}
