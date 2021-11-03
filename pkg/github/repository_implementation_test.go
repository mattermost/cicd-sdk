package github

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func getTestRepoImpl() repositoryImplementation {
	return &defaultRepoImplementation{
		githubAPIUser: githubAPIUser{},
	}
}

func TestGetIssue(t *testing.T) {
	impl := getTestRepoImpl()
	issue, err := impl.getIssue(context.Background(), "mattermost", "mattermost-server", 57)

	require.Nil(t, err)
	require.Equal(t, "Creating team ?", issue.Title)
	require.Equal(t, 57, issue.Number)
	require.Equal(t, "mattermost-server", issue.RepoName)
	require.Equal(t, "mattermost", issue.RepoOwner)

	require.Equal(t, "jeremy-flusin", issue.Username)
	// issue, err :=
}
