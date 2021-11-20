// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

import (
	"github.com/mattermost/cicd-sdk/pkg/build/runners"
	"github.com/mattermost/cicd-sdk/pkg/replacement"
	"github.com/pkg/errors"
)

const (
	BuilderID = "MatterBuild/v0.1"
)

// New returns a new build with the default options
func New(runner runners.Runner) *Build {
	return NewWithOptions(runner, DefaultOptions)
}

func NewWithOptions(runner runners.Runner, opts *Options) *Build {
	return &Build{
		runner: runner,
		opts:   opts,
	}
}

type Build struct {
	runner       runners.Runner
	opts         *Options
	Runs         []*Run
	Replacements []replacement.Replacement
}

type Options struct {
	Workdir           string
	ExpectedArtifacts []string
	EnvVars           map[string]string
	ProvenanceDir     string
}

var DefaultOptions = &Options{
	Workdir:           ".", // Working directory where the build runs
	ExpectedArtifacts: []string{},
	EnvVars:           map[string]string{},
}

// Options returns the build's option set
func (b *Build) Options() *Options {
	return b.opts
}

// Run creates a new run
func (b *Build) Run() *Run {
	// Set the runner options
	b.runner.Options().Workdir = b.Options().Workdir
	b.runner.Options().EnvVars = b.Options().EnvVars
	b.runner.Options().ProvenanceDir = b.Options().ProvenanceDir
	b.runner.Options().ExpectedArtifacts = b.Options().ExpectedArtifacts
	b.runner.Options().Replacements = b.Replacements
	for i := range b.runner.Options().Replacements {
		b.runner.Options().Replacements[i].Workdir = b.Options().Workdir
	}

	// Create the new run
	run := NewRun(b.runner)

	// The ID is the new run position in the run array:
	run.id = len(b.Runs)
	b.Runs = append(b.Runs, run)

	return run
}

// LoadConfig loads the build configuration from a file
func (b *Build) Load(path string) error {
	conf, err := LoadConfig(path)
	if err != nil {
		return errors.Wrap(err, "opening config")
	}

	// Initialize the runner from the configuration:
	runner, err := runners.New(conf.Runner.ID, conf.Runner.Parameters...)
	if err != nil {
		return errors.Wrap(err, "initializing runner from config file")
	}
	b.runner = runner

	// Load the secrets, we do this before replacements
	// because we are going to need them

	// TODO: Merge secrets from branch
	// Secrets      []SecretConfig      `yaml:"secrets"`      // Secrets required by the build

	// Build the replacement set:
	if b.Replacements == nil {
		b.Replacements = []replacement.Replacement{}
	}
	reps := []replacement.Replacement{}
	for _, rdata := range conf.Replacements {
		rep := replacement.Replacement{
			Tag:           rdata.Tag,
			Paths:         rdata.Paths,
			PathsRequired: true,
			// Required:      rdata.Required,
		}
		reps = append(reps, rep)
	}
	b.Replacements = reps

	for _, e := range conf.Env {
		b.runner.Options().EnvVars[e.Var] = e.Value
	}

	return nil
}
