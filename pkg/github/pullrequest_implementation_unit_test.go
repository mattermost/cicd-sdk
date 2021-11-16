// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.
package github

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetRebaseCommits(t *testing.T) {
	impl := defaultPRImplementation{}
	ctx := context.Background()

	pr := &PullRequest{
		RepoOwner:      "mattermost",
		RepoName:       "mattermost-server",
		Number:         18746,
		MergeCommitSHA: "f68ba02e325002d7982936860f202b0524ee33bb",
	}

	// Get the comits, they are required
	commits, err := impl.getRebaseCommits(ctx, pr)
	require.NoError(t, err, "fetching commits")
	require.Len(t, commits, 10)

	commitList, err := impl.getCommits(ctx, pr)
	require.Nil(t, err, "getting rebase commits")
	require.Len(t, commitList, 10)
	// These are the shas of commits in the PR
	require.Equal(t, "c0400f1a2d2b01227f91cd04654965b30c5e8857", commitList[9].SHA)
	require.Equal(t, "d3d12bbf9fca34851eae00af85fb103762bce267", commitList[8].SHA)
	require.Equal(t, "d0289943ff2b71e4e86d7db1268c5ad506634171", commitList[7].SHA)
	require.Equal(t, "b11e24dc8a54558af9e18640527d79548f610648", commitList[6].SHA)
	require.Equal(t, "58c664861a3facf6d6474af095ec5407f84ac899", commitList[5].SHA)
	require.Equal(t, "2a9a91e699ecb19242eb2e59a11b5eaeaa452ece", commitList[4].SHA)
	require.Equal(t, "2768ec1632b128bda9dbb9d65effc90c6d91da45", commitList[3].SHA)
	require.Equal(t, "f19820388dacc93e72adaeafa537b3a87a757121", commitList[2].SHA)
	require.Equal(t, "87bbd0dd662a9e4fa037994bf22ec8b60152f992", commitList[1].SHA)
	require.Equal(t, "2685dc20c46ac35fe809189bf94afc49026a86bc", commitList[0].SHA)

	// These are the commits in the branch. They are different
	require.Equal(t, "f68ba02e325002d7982936860f202b0524ee33bb", commits[9].SHA)
	require.Equal(t, "125767e905e06779c36dd97bc405fd73d1e18f5f", commits[8].SHA)
	require.Equal(t, "ca6e387e7eb7ee95d80c61540b5bf9840ee15255", commits[7].SHA)
	require.Equal(t, "2a18f5e31364faf48de617de2011c14124de90a1", commits[6].SHA)
	require.Equal(t, "e5caaf33c0c4c500308fbc3f8e803481c7494bad", commits[5].SHA)
	require.Equal(t, "676cebd459c7e30e9444e692693f44b483b6dc26", commits[4].SHA)
	require.Equal(t, "c3569b7c6b43a483a9910851afb36f44cbfdff28", commits[3].SHA)
	require.Equal(t, "e6528fdcc4af928407a96e83004bc4d19f1bc797", commits[2].SHA)
	require.Equal(t, "ecd49172414b819632dc59adcd5bb6e480ee759e", commits[1].SHA)
	require.Equal(t, "ec9f8df72de730cb3b61c72678cdc050e93f925d", commits[0].SHA)
}

func TestFindPatchTree(t *testing.T) {
	impl := defaultPRImplementation{}
	ctx := context.Background()
	pr := &PullRequest{
		impl:           &impl,
		RepoOwner:      "mattermost",
		RepoName:       "mattermost-server",
		Number:         18759,
		MergeCommitSHA: "bc19bb33b0590a7c5699d9a2618911adfd7c7d7c",
	}
	// Get the comits, they are required
	commits, err := impl.getCommits(ctx, pr)
	require.NoError(t, err, "fetching commits")
	require.Len(t, commits, 2)

	// In Github, generally parent #0 points to the branch history, while
	// parent #1 points to the commit list in the PR
	parentID, err := impl.findPatchTree(ctx, pr)
	require.NoError(t, err)
	require.Equal(t, 1, parentID)
}

func TestGetRepo(t *testing.T) {
	ctx := context.Background()
	pr := &PullRequest{
		impl:      &defaultPRImplementation{},
		RepoOwner: "mattermost",
		RepoName:  "mattermost-server",
		Number:    18759,
	}

	require.NotNil(t, pr.GetRepository(ctx))
	require.Equal(t, "mattermost", pr.GetRepository(ctx).Owner)
	require.Equal(t, "mattermost-server", pr.GetRepository(ctx).Name)
}

func TestGetMergeMethod(t *testing.T) {
	repo := NewRepository("mattermost", "mattermost-mobile")
	ctx := context.Background()
	pr, err := repo.GetPullRequest(ctx, 5830)
	require.NoError(t, err)

	// Check the PR data is sound
	require.Equal(t, "1501b6ec05947d308ad4125d762db3ecd625a826", pr.MergeCommitSHA)

	method, err := pr.GetMergeMode(ctx)
	require.NoError(t, err)
	require.Equal(t, MMREBASE, method)
}

func TestGetCommits(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository("mattermost", "mattermost-server")
	pr, err := repo.GetPullRequest(ctx, 18746)
	require.NoError(t, err)

	require.Equal(t, "f68ba02e325002d7982936860f202b0524ee33bb", pr.MergeCommitSHA)
	commits, err := pr.GetCommits(ctx)
	require.NoError(t, err)
	require.Len(t, commits, 10)

	require.Equal(t, commits[0].SHA, "2685dc20c46ac35fe809189bf94afc49026a86bc")
	require.Len(t, commits[0].Files, 1)
	require.Len(t, commits[0].Parents, 1)
	require.Equal(t, commits[0].Files, []CommitFile{{"i18n/fr.json", "0e11e46380c19a97f01bd72bfe8a516766f14436"}})
}

func TestMergeCommit(t *testing.T) {
	ctx := context.Background()
	repo := NewRepository("mattermost", "mattermost-server")
	pr, err := repo.GetPullRequest(ctx, 18746)
	require.NoError(t, err)

	require.Equal(t, "f68ba02e325002d7982936860f202b0524ee33bb", pr.MergeCommitSHA)
	require.NotNil(t, pr.GetRepository(ctx))
	mergeCommit, err := pr.GetRepository(ctx).GetCommit(ctx, pr.MergeCommitSHA)
	require.NoError(t, err)
	require.NotNil(t, mergeCommit)
	require.Equal(t, "f68ba02e325002d7982936860f202b0524ee33bb", mergeCommit.SHA)
	mergeCommit.Parents[0] = "125767e905e06779c36dd97bc405fd73d1e18f5f"
	require.Equal(t, "1a1ac59e2853132888f0a56c7bc07a23a0783401", mergeCommit.TreeSHA)
	require.Len(t, mergeCommit.Files, 1)
	require.Equal(t, []CommitFile{{"i18n/en_AU.json", "de948430eae8a079f7e875f9ea44d441a35a0029"}}, mergeCommit.Files)
	// TODO: Test dual parent mergeCommit (real merge commit)
}
