package build

import (
	"os"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

// Load reads a config file and return a config object
func Load(path string) (*Config, error) {
	yamlData, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "reading build configuration file")
	}
	conf := &Config{}
	if err := yaml.Unmarshal(yamlData, conf); err != nil {
		return nil, errors.Wrap(err, "parsing config yaml data")
	}

	return conf, nil
}

type Config struct {
	Runner       RunnerConfig        `yaml:"runner"`       // Tag determining the runner to use
	Secrets      []SecretConfig      `yaml:"secrets"`      // Secrets required by the build
	Env          []EnvConfig         `yaml:"env"`          // Environment vars to require/set
	Replacements []ReplacementConfig `yaml:"replacements"` // Replacements to perform before the run
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
			if r.Path == "" {
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
	Path      string `yaml:"path"`
	Tag       string `yaml:"tag"`
	ValueFrom struct {
		Secret string `yaml:"secret"`
		Env    string `yaml:"env"`
	} `yaml:"valueFrom"`
}
