package build

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	testfile := `---
runner:
  id: make
  params: ["-v"]
secrets:
  - name: TEST_SECRET
env:     
  - var: COMMIT_SHA
    value: b739074e0260def700eb13b2aa6091cae9366327
  - var: COMMIT_WITHOUT_SHA
replacements:
  - paths: [code.go]
    tag: placeholder
    valueFrom:
      secret: TEST_SECRET
artifacts:
  files: ["README.md", "release-notes.md", "LICENSE", "go.mod", "go.sum"]    
  images: ["index.docker.io/mattermost/mm-te-test:test"]
transfers:
  - source: ["mattermost-webapp.tar.gz"]
    destination: s3://bucket1/dir/subdir/
  - source: ["mmctl", "mmctl.sha512"]
    destination: s3://bucket2/projectname/dir/
materials:
  - source: "git+https://github.com/foo/bar.git"
    digest:
      sha1: e97447134cd650ee9f9da5d705a06d3c548d3d6c
`
	f, err := os.CreateTemp("", "yaml-test-")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	require.NoError(t, os.WriteFile(f.Name(), []byte(testfile), os.FileMode(0o644)))

	// Load the testfile
	conf, err := LoadConfig(f.Name())
	require.NoError(t, err)

	// Test the expected values:
	require.Len(t, conf.Secrets, 1)
	require.Len(t, conf.Env, 2)
	require.Len(t, conf.Replacements, 1)

	require.Equal(t, conf.Runner.ID, "make")
	require.NotNil(t, conf.Runner.Parameters)
	require.Len(t, conf.Runner.Parameters, 1)
	require.Equal(t, conf.Runner.Parameters[0], "-v")

	require.Equal(t, conf.Secrets[0].Name, "TEST_SECRET")

	require.Equal(t, conf.Env[0].Var, "COMMIT_SHA")
	require.Equal(t, conf.Env[0].Value, "b739074e0260def700eb13b2aa6091cae9366327")
	require.Equal(t, conf.Env[1].Var, "COMMIT_WITHOUT_SHA")
	require.Equal(t, conf.Env[1].Value, "")

	require.Equal(t, conf.Replacements[0].Paths[0], "code.go")
	require.Equal(t, conf.Replacements[0].Tag, "placeholder")
	require.Equal(t, conf.Replacements[0].ValueFrom.Secret, "TEST_SECRET")
	require.Equal(t, conf.Replacements[0].ValueFrom.Env, "")

	require.Len(t, conf.Artifacts.Files, 5)
	require.ElementsMatch(t,
		[]string{"README.md", "release-notes.md", "LICENSE", "go.mod", "go.sum"},
		conf.Artifacts.Files,
	)
	require.Len(t, conf.Artifacts.Images, 1)
	require.ElementsMatch(t, []string{"index.docker.io/mattermost/mm-te-test:test"}, conf.Artifacts.Images)

	require.Len(t, conf.Transfers, 2)
	require.Equal(t, conf.Transfers[0].Destination, "s3://bucket1/dir/subdir/")
	require.Equal(t, conf.Transfers[0].Source, []string{"mattermost-webapp.tar.gz"})
	require.Equal(t, conf.Transfers[1].Destination, "s3://bucket2/projectname/dir/")
	require.Equal(t, conf.Transfers[1].Source, []string{"mmctl", "mmctl.sha512"})

	require.Equal(t, conf.Materials[0].URI, "")
	require.Len(t, conf.Materials, 1)
	require.Len(t, conf.Materials[0].Digest, 1)
	require.Equal(t, conf.Materials[0].Digest["sha1"], "e97447134cd650ee9f9da5d705a06d3c548d3d6c")
}

func TestConfigValidate(t *testing.T) {
	config := &Config{
		Runner: RunnerConfig{
			ID:         "make",
			Parameters: []string{},
		},
		Secrets: []SecretConfig{
			{
				Name: "TEST_SECRET",
			},
		},
		Env: []EnvConfig{
			{
				Var:   "TEST_ENV",
				Value: "",
			},
		},
		Replacements: []ReplacementConfig{
			{
				Paths: []string{"test.go"},
				Tag:   "target",
				ValueFrom: struct {
					Secret string "yaml:\"secret\""
					Env    string "yaml:\"env\""
				}{"TEST_SECRET", ""},
			},
		},
	}
	const TEST = "TEST"
	tests := []struct {
		Setup       func(*Config)
		ShouldError bool
	}{
		{func(c *Config) {}, false},                                                                                 // No error
		{func(c *Config) { c.Runner.ID = "" }, true},                                                                // Lacks runner ID
		{func(c *Config) { c.Secrets[0].Name = "" }, true},                                                          // Blank secret name
		{func(c *Config) { c.Env[0].Var = "" }, true},                                                               // Blank Env name
		{func(c *Config) { c.Replacements[0].Paths = nil }, true},                                                   // Blank replacement path
		{func(c *Config) { c.Replacements[0].Tag = "" }, true},                                                      // Blank replacement Tag
		{func(c *Config) { c.Replacements[0].ValueFrom.Secret = "" }, true},                                         // Both replacement sources blank
		{func(c *Config) { c.Replacements[0].ValueFrom.Env = TEST }, true},                                          // Both replacement sources not-blank
		{func(c *Config) { c.Replacements[0].ValueFrom.Secret = TEST }, true},                                       // Replacement secret not defined
		{func(c *Config) { c.Replacements[0].ValueFrom.Secret = ""; c.Replacements[0].ValueFrom.Env = TEST }, true}, // Replacement env not defined
	}

	for _, tc := range tests {
		sut := config
		tc.Setup(sut)
		if tc.ShouldError {
			require.Error(t, sut.Validate())
		} else {
			require.NoError(t, sut.Validate())
		}
	}
}

var sampleConfWithVars = `transfers:
  - source: ["mattermost-webapp.tar.gz"]
    destination: s3://${BUCKET}/gitlab/${PROJECT_NAME}/ee/test/${COMMIT_SHA}
  - source: ["mattermost-webapp.tar.gz"]
    destination: s3://${BUCKET}/gitlab/${PROJECT_NAME}/te/${COMMIT_SHA}
`

func TestExtractConfigVariables(t *testing.T) {
	flags := extractConfigVariables([]byte(sampleConfWithVars))
	require.Len(t, flags, 3)
	require.ElementsMatch(t, flags, []string{"BUCKET", "PROJECT_NAME", "COMMIT_SHA"})
}

func TestReplaceVariables(t *testing.T) {
	envReplacements := `env:
  - var: BUCKET
    value: mattermost-release
  - var: PROJECT_NAME
    value: project
  - var: COMMIT_SHA
    value: d642f2cd18bf96a3da793d6e594da3b7029c6ca2
`

	// Test replacing data from env variables defined in the yaml itself:
	newYaml, err := replaceVariables([]byte(sampleConfWithVars + envReplacements))
	require.NoError(t, err)
	require.NotEqual(t, newYaml, []byte(sampleConfWithVars+envReplacements))
	require.True(t, strings.Contains(string(newYaml), "destination: s3://mattermost-release/gitlab/project/te/d642f2cd18bf96a3da793d6e594da3b7029c6ca2"))
	require.True(t, strings.Contains(string(newYaml), "destination: s3://mattermost-release/gitlab/project/ee/test/d642f2cd18bf96a3da793d6e594da3b7029c6ca2"))

	// Test replacing data from the system environment variables:

	// First. Without the defined values, this should throw an error
	_, err = replaceVariables([]byte(sampleConfWithVars))
	require.NoError(t, err)

	// Now set the environment vars and retest
	os.Setenv("BUCKET", "mattermost-release")
	os.Setenv("PROJECT_NAME", "project")
	os.Setenv("COMMIT_SHA", "d642f2cd18bf96a3da793d6e594da3b7029c6ca2")
	newYaml, err = replaceVariables([]byte(sampleConfWithVars))
	require.NoError(t, err)
	require.True(t, strings.Contains(string(newYaml), "destination: s3://mattermost-release/gitlab/project/te/d642f2cd18bf96a3da793d6e594da3b7029c6ca2"))
	require.True(t, strings.Contains(string(newYaml), "destination: s3://mattermost-release/gitlab/project/ee/test/d642f2cd18bf96a3da793d6e594da3b7029c6ca2"))
}
