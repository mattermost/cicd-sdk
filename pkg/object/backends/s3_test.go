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
