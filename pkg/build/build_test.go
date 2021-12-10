// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See LICENSE for license information.

package build

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDigestSetForFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "test-")
	require.NoError(t, err)
	defer os.Remove(tmp.Name())
	require.NoError(t, os.WriteFile(tmp.Name(), []byte("test 12323837465876 test ------"), os.FileMode(0o644)))
	set, err := digestSetForFile(tmp.Name())
	require.NoError(t, err)
	require.Len(t, set, 3)
	require.Equal(t, set, map[string]string{
		"sha1":   "9aadf0f50c0b95df4a89b526b9977dd895dd8df1",
		"sha256": "308b4dc8285a00822ceb5e207e4c7dbe22459b4883651605c0f4b281af44c946",
		"sha512": "5ec43dbc82add923c5eaa1e3dac6eda3faddb66f27d90fecf47b302586e24a2920d8349cb98a62dd82d4876c4364f74fd31176a6f81f94ad5f8fdfa49b584317",
	})

	// Non existent file should fail
	_, err = digestSetForFile("lskjdflskdjflkjs")
	require.Error(t, err)
}
