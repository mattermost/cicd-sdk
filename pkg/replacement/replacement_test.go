package replacement

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

const replacementTestText string = `This is a sample text
we need to Test the TEST of replacers
used in replacements across builds
`

func TestReplacement(t *testing.T) {
	r := Replacement{
		Tag:   "TEST",
		Value: "modified",
	}

	for i, tc := range []struct {
		shouldError bool
		prepare     func() Replacement
	}{
		{
			// Replacements without tags must fail
			true,
			func() Replacement {
				r.Tag = ""
				return r
			},
		},
		{
			// Replacements not found but not required should not fail
			false,
			func() Replacement {
				r.Tag = "NOTFOUND"
				r.Required = false
				return r
			},
		},
		{
			// Replacements not found but required must fail
			true,
			func() Replacement {
				r.Tag = "NOTFOUND"
				r.Required = true
				return r
			},
		},
		{
			// Successful replacements should not fail
			false,
			func() Replacement {
				r.Tag = "TEST"
				r.Required = true
				return r
			},
		},
	} {
		file, err := os.CreateTemp("", "temp-replacer")
		require.NoError(t, err)
		defer os.Remove(file.Name())
		require.NoError(t, os.WriteFile(file.Name(), []byte(replacementTestText), os.FileMode(0o644)))

		sut := tc.prepare()
		sut.Paths = append(sut.Paths, file.Name())
		err = sut.Apply()
		if tc.shouldError {
			require.Error(t, err, fmt.Sprintf("test case #%d", i))
		} else {
			require.NoError(t, err, fmt.Sprintf("test case #%d", i))
			res, err := r.Check()
			require.NoError(t, err)
			require.True(t, res)
		}
	}
}

func TestIsPathReplaced(t *testing.T) {
	r := Replacement{}
	// Replacements without tags should fail
	_, err := r.IsPathReplaced("/tmp/jldsjjl")
	require.Error(t, err)

	// Create a file with a string
	f, err := os.CreateTemp("", "temp-replacer-test-")
	require.NoError(t, err, "creating test file")
	defer os.Remove(f.Name())
	r.Paths = append(r.Paths, f.Name())
	r.Tag = "WAY"

	// Create a file with a quote
	require.NoError(t, os.WriteFile(f.Name(), []byte("Do or do not\nThere is no WAY.\n"), os.FileMode(0o644)))
	res, err := r.IsPathReplaced(f.Name())
	require.NoError(t, err)
	require.False(t, res)

	// Adjust it to become awesome wisdom
	require.NoError(t, os.WriteFile(f.Name(), []byte("Do or do not\nThere is no try.\n"), os.FileMode(0o644)))
	res, err = r.IsPathReplaced(f.Name())
	require.NoError(t, err)
	require.True(t, res)
}

func TestCheck(t *testing.T) {
	r := Replacement{}
	// Replacements without tags should fail
	_, err := r.IsPathReplaced("/tmp/jldsjjl")
	require.Error(t, err)

	// Create a file with a string
	f, err := os.CreateTemp("", "temp-replacer-test-")
	require.NoError(t, err, "creating test file")
	defer os.Remove(f.Name())
	r.Paths = append(r.Paths, f.Name())
	r.Tag = "SOMETIMES"

	// Create a file with a quote
	require.NoError(t, os.WriteFile(f.Name(), []byte("The Force will be with you. SOMETIMES.\n"), os.FileMode(0o644)))
	res, err := r.Check()
	require.NoError(t, err)
	require.False(t, res)

	// Adjust it to become awesome wisdom
	require.NoError(t, os.WriteFile(f.Name(), []byte("The Force will be with you. Always.\n"), os.FileMode(0o644)))
	res, err = r.IsPathReplaced(f.Name())
	require.NoError(t, err)
	require.True(t, res)
}

func TestCorruption(t *testing.T) {
	// Create a file with a string
	f, err := os.CreateTemp("", "temp-replacer-test-")
	require.NoError(t, err, "creating test file")
	defer os.Remove(f.Name())
	r := Replacement{
		Paths: []string{f.Name()},
		Tag:   "FILECORRUPTION",
		Value: "luck",
	}

	// Create a file with a quote
	require.NoError(t, os.WriteFile(
		f.Name(), []byte("In my experience,\nthere's no such thing as FILECORRUPTION.\n"),
		os.FileMode(0o644),
	))
	require.NoError(t, r.Apply())

	// Now read back the file and check it looks as expected
	rdata, err := os.ReadFile(f.Name())
	require.NoError(t, err, "reading replaced data")
	require.Equal(t, []byte("In my experience,\nthere's no such thing as luck.\n"), rdata)
}
