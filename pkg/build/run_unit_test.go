// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStagingPath checks the hashing function to generate a path
func TestStagingPath(t *testing.T) {
	r := &Run{
		opts: &RunOptions{
			BuildPoint: "46305d50a15717e2d224e38f2f2bdc9027a7cbc7",
			Materials: MaterialsConfig{
				{
					URI:    "http://example.com/repo/go.mod",
					Digest: map[string]string{"sha1": "61a7663a7c0f46ab149ec2cadd44fc3cc30f9403"},
				},
				{
					URI:    "http://example.com/repo/go.sum",
					Digest: map[string]string{"sha1": "ac74142d9394dc40c046eadc99b19c95b6f8d5d3"},
				},
				{
					URI:    "http://example.com/repo/source.go",
					Digest: map[string]string{"sha512": "efbedc70276435eaf861152cb139dccc91c31c5955385b6797feaf36f3ad7a974b07aec012a135c2105aefcb606fffd50b261efa8be7f993f5c55cf7fba703e9"},
				},
			},
		},
	}

	ri := defaultRunImplementation{}
	firstHash := "9241fbc43a90babf28912d4662580f8740e709237c1797a29ea5ee64558c7b9f"

	path, err := ri.stagingPath(r)
	require.NoError(t, err)
	require.Equal(t, firstHash, path)

	// Adding a sha512 or sha256 to existint artifacts should not alter the path
	r.opts.Materials[0].Digest["sha512"] = "2f5ee12f90520edc83dde8d2600a536f05be208cb26be9fb239b8a5975f145c5c530cb7ec1ec9d3b4cf6a652253620182b73a799ba072798e5ae17d29e7857d5"
	path, err = ri.stagingPath(r)
	require.NoError(t, err)
	require.Equal(t, firstHash, path)

	r.opts.Materials[1].Digest["sha256"] = "f26b0d6be3a5ec8055e988424adb11a85f56294128d4d05d4c2fe53430d3055c"
	path, err = ri.stagingPath(r)
	require.NoError(t, err)
	require.Equal(t, firstHash, path)

	// Adding a sha1 to the third material (that does not have one) must produce
	// a new staging path:
	r.opts.Materials[2].Digest["sha1"] = "327c7a98c992fbcf7865066dec38408e91a6d998"
	path, err = ri.stagingPath(r)
	require.NoError(t, err)
	require.Equal(t, "9e9523d6de1d7fe54a300f631514c4a6960723482149e225a1dbca023efc050e", path)

	// Modifying the project build point will generate a new
	// staging path as the source is assumed to have changed:
	r.opts.BuildPoint = "c6ef45fb22ed03c6e458d8111b860e0c8914555e"
	path, err = ri.stagingPath(r)
	require.NoError(t, err)
	require.Equal(t, "82d771c189319ff60d207579bc9c0595c84d15de88327ab25f033d03b858585b", path)
}
