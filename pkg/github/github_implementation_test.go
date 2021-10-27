// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

import (
	"context"
	"net/http"
	"testing"

	gogithub "github.com/google/go-github/v39/github"
	"github.com/stretchr/testify/require"
)

func getTestImplementation(t *testing.T) *defaultGithubImplementation {
	return &defaultGithubImplementation{
		client: gogithub.NewClient(http.DefaultClient),
	}
}

func TestGetPullRequestFromAPI(t *testing.T) {
	// Getch a commit from GH and check the variable assignments
	gh := getTestImplementation(t)
	pr, err := gh.getPullRequestFromAPI(context.Background(), "mattermost", "mattermost-server", 1)
	require.Nil(t, err)
	require.NotNil(t, pr)
	require.Equal(t, 1, pr.Number)
	require.Equal(t, "jwilander", pr.Username)
	require.Equal(t, "mm-1223", pr.Ref)
	require.Equal(t, "753b952bde9ee28311ca49c2ec0113e06a40bd4f", pr.Sha)
	require.Equal(t, "closed", pr.State)
	require.Equal(t, "https://api.github.com/repos/mattermost/mattermost-server/pulls/1", pr.URL)
	require.Equal(t, "f86a6578ff3110b65bc5ff28e0e58358bd13d9e2", pr.MergeCommitSHA)
}
