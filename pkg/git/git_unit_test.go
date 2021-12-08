package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/release-utils/command"
)

func createTestRepo(t *testing.T) string {
	dir, err := os.MkdirTemp("", "test-repo-")
	require.NoError(t, err)

	require.NoError(t, command.NewWithWorkDir(dir, gitCommand, "init", "--initial-branch=main").RunSuccess())
	require.NoError(t, command.NewWithWorkDir(dir, gitCommand, "config", "user.email", "user@example.com").RunSuccess())
	require.NoError(t, command.NewWithWorkDir(dir, gitCommand, "config", "user.name", "Example Users").RunSuccess())
	require.NoError(t, command.NewWithWorkDir(dir, gitCommand, "commit", "--allow-empty", "-m", "First Commit").RunSuccess())
	return dir
}

func TestCloneRepository(t *testing.T) {
	const testRepo = "https://github.com/mattermost/.github.git"
	impl := defaultGitImpl{}
	dir, err := os.MkdirTemp("", "test-git-clone-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	repo, err := impl.cloneRepo(testRepo, dir)
	require.NoError(t, err)

	r, err := repo.client.Remote("origin")
	require.NoError(t, err)
	require.Contains(t, r.String(), testRepo)
	require.DirExists(t, filepath.Join(dir, ".git"))
	require.FileExists(t, filepath.Join(dir, "README.md"))
}

func TestOpenRepo(t *testing.T) {
	dir := createTestRepo(t)
	defer os.RemoveAll(dir)
	impl := defaultGitImpl{}
	repo, err := impl.openRepo(dir)
	require.NoError(t, err)
	tree, err := repo.client.Worktree()
	require.NoError(t, err)
	status, err := tree.Status()
	require.NoError(t, err)
	require.True(t, status.IsClean())
	o, err := command.NewWithWorkDir(dir, gitCommand, "log").RunSuccessOutput()
	require.NoError(t, err)
	require.Contains(t, o.Output(), "First Commit")
}

func TestLSRemote(t *testing.T) {
	impl := defaultGitImpl{}
	res, err := impl.lsRemote("https://github.com/mattermost/mattermost-server", "release-6.1")
	require.NoError(t, err)
	require.Contains(t, res, "f39534a2a28036a249d8f584e6fd489b7535a610")
	require.Contains(t, res, "refs/heads/release-6.1")
}
