// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package runners

import (
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

	// Write a simple make file
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, "Makefile"),
		[]byte(".PHONY: test-target\ntest-target: # Just echo a string\n\t echo \"Hola amigos\"\n"), os.FileMode(0o644)),
	)

	//
	m := NewMake("test-target")
	m.Options().Workdir = dir
	require.NoError(t, m.Run())
	// Verify the output.
	require.Equal(t, "echo \"Hola amigos\"\nHola amigos\n", m.Output())
}
