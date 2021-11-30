package build

import (
	"bytes"
	"fmt"
	"os"
	"regexp"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var varRegexp = regexp.MustCompile(`\$\{([_A-Z0-9]+)\}`)

// replaceVariables replaces the yaml configuration variables
func replaceVariables(yamlData []byte) ([]byte, error) {
	vars := extractConfigVariables(yamlData)
	if len(vars) == 0 {
		logrus.Info("No configuration variables found in YAML code")
		return yamlData, nil
	}

	logrus.Infof("Replacing %d configuration variables in YAML code (%v)", len(vars), vars)
	valueVals := map[string]string{}

	// First, we do a first pass at parsing the config data to see if
	// the replacements are defined inside of the conf itself (in env vars for example)
	c, err := parseConf(yamlData)
	if err != nil {
		return nil, errors.Wrap(err, "parsing yaml configuration")
	}

	// Cycle all vars from the YAML conf and try to get a value for them
	for _, yamlVariable := range vars {
		valueVals[yamlVariable] = ""
		for _, envConf := range c.Env {
			// If there is a predefined environment var, use that value
			if envConf.Var == yamlVariable {
				valueVals[yamlVariable] = envConf.Value
				logrus.Infof(
					"YAML conf variable %s set to value '%s' from predefined environment",
					yamlVariable, envConf.Value,
				)
				break
			}
		}

		if valueVals[yamlVariable] != "" {
			continue
		}

		// If not, check if the value is defined in the system env
		if v := os.Getenv(yamlVariable); v != "" {
			valueVals[yamlVariable] = v
			logrus.Infof(
				"YAML conf variable %s set to value '%s' from system environment",
				yamlVariable, v,
			)
			continue
		}

		return nil, errors.Wrapf(
			err, "unable to find a value for yaml config variable $%s", yamlVariable,
		)
	}

	// Replace the values in the yaml data
	for vr, vl := range valueVals {
		yamlData = bytes.ReplaceAll(yamlData, []byte(fmt.Sprintf("${%s}", vr)), []byte(vl))
	}

	return yamlData, nil
}

// Load reads a config file and return a config object
func LoadConfig(path string) (*Config, error) {
	yamlData, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading build configuration file")
	}

	yamlData, err = replaceVariables(yamlData)
	if err != nil {
		return nil, errors.Wrap(err, "replacing configuration variables")
	}

	conf, err := parseConf(yamlData)
	if err != nil {
		return nil, errors.Wrap(err, "parsing config yaml data")
	}

	return conf, nil
}

// extractConfigVariables scans configuration data to search for variables
func extractConfigVariables(yamlData []byte) []string {
	matches := varRegexp.FindAllSubmatch(yamlData, -1)
	vars := []string{}
	foundVars := map[string]struct{}{}
	for _, match := range matches {
		foundVars[string(match[1])] = struct{}{}
	}
	for v := range foundVars {
		vars = append(vars, v)
	}
	return vars
}

func parseConf(yamlData []byte) (*Config, error) {
	conf := &Config{
		Secrets:      []SecretConfig{},
		Env:          []EnvConfig{},
		Replacements: []ReplacementConfig{},
		Transfers:    []TransferConfig{},
	}
	if err := yaml.Unmarshal(yamlData, conf); err != nil {
		return nil, errors.Wrap(err, "parsing config yaml data")
	}
	return conf, nil
}

type Config struct {
	Runner        RunnerConfig        `yaml:"runner"`       // Tag determining the runner to use
	Secrets       []SecretConfig      `yaml:"secrets"`      // Secrets required by the build
	Env           []EnvConfig         `yaml:"env"`          // Environment vars to require/set
	Replacements  []ReplacementConfig `yaml:"replacements"` // Replacements to perform before the run
	Artifacts     ArtifactsConfig     `yaml:"artifacts"`    // Data about artifacts expected to be built
	ProvenanceDir string              `yaml:"provenance"`   // Directory to write provenance data
	Transfers     []TransferConfig    `yaml:"transfers"`    // List of artifacts to be transferred out after the build is done
}

// Validate checks the configuration values to make sure they are complete
func (conf *Config) Validate() error {
	// Check we have a runner
	if conf.Runner.ID == "" {
		return errors.New("runner ID is missing")
	}

	// Check all secrets have names
	if conf.Secrets != nil {
		for i, s := range conf.Secrets {
			if s.Name == "" {
				return errors.Errorf("secret #%d name is blank", i)
			}
		}
	}
	// Check all environmen vars have names
	if conf.Env != nil {
		for i, v := range conf.Env {
			if v.Var == "" {
				return errors.Errorf("envvar #%d name is blank", i)
			}
		}
		// TODO: Check var name syntax
	}

	// Check replacement configuration
	if conf.Replacements != nil {
		for i, r := range conf.Replacements {
			if r.Paths == nil {
				return errors.Errorf("replacement #%d path is blank", i)
			}
			if r.Tag == "" {
				return errors.Errorf("replacement #%d tag is blank", i)
			}

			if r.ValueFrom.Env == "" && r.ValueFrom.Secret == "" {
				return errors.Errorf("replacement #%d has no secret or env source ", i)
			}

			if r.ValueFrom.Env != "" && r.ValueFrom.Secret != "" {
				return errors.Errorf("replacement #%d has set sources from env and secret", i)
			}

			if r.ValueFrom.Secret != "" {
				found := false
				for _, s := range conf.Secrets {
					if s.Name == r.ValueFrom.Secret {
						found = true
						break
					}
					if !found {
						return errors.Errorf("replacement #%d has secret source %s but it is not defined", i, r.ValueFrom.Secret)
					}
				}
			}

			if r.ValueFrom.Env != "" {
				found := false
				for _, s := range conf.Env {
					if s.Var == r.ValueFrom.Env {
						found = true
						break
					}
					if !found {
						return errors.Errorf("replacement #%d has env source %s but it is not defined", i, r.ValueFrom.Env)
					}
				}
			}
		}
	}

	if conf.Transfers != nil {
		for i, t := range conf.Transfers {
			if t.Destination == "" {
				return errors.Errorf("transfer #%d config has no destination URL", i)
			}
			if len(t.Source) == 0 {
				return errors.Errorf("transfer #%d config has empty list of artifacts", i)
			}
		}
	}
	logrus.Info("Build configuration is valid")
	return nil
}

type RunnerConfig struct {
	ID         string   `yaml:"id"`
	Parameters []string `yaml:"params"`
}

type SecretConfig struct {
	Name string `yaml:"name"` // Name of the secret
}

type EnvConfig struct {
	Var   string `yaml:"var"`   // Env var name. Will be required
	Value string `yaml:"value"` // Value. If set, the build system will set it before starting
}

type ReplacementConfig struct {
	Required      bool     `yaml:"required"`
	RequiredPaths bool     `yaml:"requiredPaths"`
	Tag           string   `yaml:"tag"`
	Value         string   `yaml:"value"`
	Paths         []string `yaml:"paths"`
	ValueFrom     struct {
		Secret string `yaml:"secret"`
		Env    string `yaml:"env"`
	} `yaml:"valueFrom"`
}

type ArtifactsConfig struct {
	Files  []string `yaml:"files"` // List of files expected from the build
	Images []string `yaml:"images"`
}

type TransferConfig struct {
	Source      []string `yaml:"source"`      // List if files to transfer out
	Destination string   `yaml:"destination"` // An object URL where files will be copied to
}
