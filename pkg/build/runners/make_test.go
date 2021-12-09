// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package runners

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMakeRun(t *testing.T) {
	// create a testdir
	dir, err := os.MkdirTemp("", "make-test-")
	require.NoError(t, err)
	defer os.Remove(dir)

	tmpfile, err := os.CreateTemp("", "make-test-")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	// Write a simple make file
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Makefile"),
		[]byte(
			fmt.Sprintf(
				".PHONY: test-target\ntest-target: # Just echo a string\n\t echo \"Hola amigos\" > %s\n",
				tmpfile.Name(),
			),
		),
		os.FileMode(0o644)),
	)

	//
	m := NewMake("test-target")
	m.Options().Workdir = dir
	require.NoError(t, m.Run())
	// Verify the output.
	require.FileExists(t, tmpfile.Name())
	data, err := os.ReadFile(tmpfile.Name())
	require.NoError(t, err)
	require.Equal(t, "Hola amigos\n", string(data))
}
