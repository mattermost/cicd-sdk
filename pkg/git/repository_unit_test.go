package git

import (
	"os"
	"testing"

	gogit "github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/release-utils/command"
)

func TestCreateBranch(t *testing.T) {
	repoDir := createTestRepo(t)
	defer os.RemoveAll(repoDir)
	opts := defaultRepositoryOptions
	opts.Path = repoDir

	impl := defaultRepositoryImpl{}
	gogitrepo, err := gogit.PlainOpen(repoDir)
	require.NoError(t, err)
	branchName := "test-branch"
	// Create the branch
	require.NoError(t, impl.createBranch(gogitrepo, opts, branchName))

	// Ensure the branch was created
	cmd := command.NewWithWorkDir(repoDir, "git", "branch")
	output, err := cmd.RunSuccessOutput()
	require.Nil(t, err)

	require.Contains(t, output.Output(), branchName)
}

func TestCheckout(t *testing.T) {
	repoDir := createTestRepo(t)
	defer os.RemoveAll(repoDir)
	opts := defaultRepositoryOptions
	opts.Path = repoDir

	gogitrepo, err := gogit.PlainOpen(repoDir)
	require.NoError(t, err)

	impl := defaultRepositoryImpl{}
	require.NoError(t, impl.createBranch(gogitrepo, opts, "test"))

	cmd := command.NewWithWorkDir(repoDir, "git", "branch")
	output, err := cmd.RunSuccessOutput()
	require.Nil(t, err)

	require.Contains(t, output.Output(), "* main")
	require.NotContains(t, output.Output(), "* test")

	require.NoError(t, impl.checkout(gogitrepo, opts, "test"))

	cmd2 := command.NewWithWorkDir(repoDir, "git", "branch")
	output, err = cmd2.RunSuccessOutput()
	require.Nil(t, err)

	require.Contains(t, output.Output(), "* test")
	require.NotContains(t, output.Output(), "* main")
}
