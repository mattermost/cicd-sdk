package build

import (
	"os"
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
  - path: code.go
    tag: placeholder
    valueFrom:
      secret: TEST_SECRET
`
	f, err := os.CreateTemp("", "yaml-test-")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	require.NoError(t, os.WriteFile(f.Name(), []byte(testfile), os.FileMode(0o644)))

	// Load the testfile
	conf, err := Load(f.Name())
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

	require.Equal(t, conf.Replacements[0].Path, "code.go")
	require.Equal(t, conf.Replacements[0].Tag, "placeholder")
	require.Equal(t, conf.Replacements[0].ValueFrom.Secret, "TEST_SECRET")
	require.Equal(t, conf.Replacements[0].ValueFrom.Env, "")
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
				Path: "test.go",
				Tag:  "target",
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
		{func(c *Config) { c.Replacements[0].Path = "" }, true},                                                     // Blank replacement path
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
