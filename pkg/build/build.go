// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	"github.com/mattermost/cicd-sdk/pkg/build/runners"
	"github.com/mattermost/cicd-sdk/pkg/replacement"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/hash"
	"sigs.k8s.io/release-utils/util"
)

const (
	BuilderID      = "MatterBuild/v0.1"
	ConfigFileName = "matterbuild.yaml"
)

// New returns a new build with the default options
func New(runner runners.Runner) *Build {
	return NewWithOptions(runner, DefaultOptions)
}

func loadAttestation(path string) (*intoto.ProvenanceStatement, error) {
	attestationData, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "opening provenance attestation")
	}
	statement := &intoto.ProvenanceStatement{}
	if err := json.Unmarshal(attestationData, statement); err != nil {
		return nil, errors.Wrapf(err, "unmarshaling attestation from %s", path)
	}

	return statement, nil
}

func NewFromConfigFile(configPath string) (*Build, error) {
	b := &Build{opts: &Options{}}
	if err := b.Load(configPath); err != nil {
		return nil, errors.Wrap(err, "loading build config file")
	}
	return b, nil
}

// NewFromAttestation returns a build from the provenance attestation
func NewFromAttestation(provenancePath string, extraOpts *Options) (*Build, error) {
	statement, err := loadAttestation(provenancePath)
	if err != nil {
		return nil, errors.Wrap(err, "opening attestation metadata")
	}

	// Read the build parameters
	params := []string{}
	if ifs, ok := statement.Predicate.Invocation.Parameters.([]interface{}); ok {
		for _, i := range ifs {
			if v, ok := i.(string); ok {
				params = append(params, v)
			} else {
				logrus.Info("Unknown params")
			}
		}
	}

	// Get the runn from the attestation
	runner, err := runners.New(
		statement.Predicate.BuildType, params...,
	)
	if err != nil {
		return nil, errors.Wrap(err, "getting build runner")
	}

	b := &Build{
		runner: runner,
		opts:   extraOpts, // Options are what we got but ill be mostly overwritten
	}

	// If there is a config source, load the configuration file
	if statement.Predicate.Invocation.ConfigSource.URI != "" {
		// When done, build should checkout the config file at the specified commit
		// we need more test repos to implement and test this.
		logrus.Warn("ConfigSource commit digest not supported yet")
		if util.Exists(
			filepath.Join(extraOpts.Workdir, statement.Predicate.Invocation.ConfigSource.URI),
		) {
			if err := b.Load(
				filepath.Join(extraOpts.Workdir, statement.Predicate.Invocation.ConfigSource.URI),
			); err != nil {
				return nil, errors.Wrap(err, "loading configuration file")
			}
		} else {
			return nil, errors.Errorf(
				"unable to load config source from %s", statement.Predicate.Invocation.ConfigSource.URI,
			)
		}
	}

	// Add material #0 (which we always interpret as the main source)
	if len(statement.Predicate.Materials) > 0 && statement.Predicate.Materials[0].URI != "" {
		b.Options().Source = strings.TrimPrefix(statement.Predicate.Materials[0].URI, "git+")
	} else {
		logrus.Warn("Attestation does not have a materials entry for source code")
	}
	return b, nil
}

func (b *Build) RunAttestation(path string) error {
	statement, err := loadAttestation(path)
	if err != nil {
		return errors.Wrap(err, "opening attestation metadata")
	}
	ropts := &RunOptions{}
	if len(statement.Predicate.Materials) > 0 {
		ropts.BuildPoint = statement.Predicate.Materials[0].Digest["sha1"]
	}
	run := b.RunWithOptions(ropts)

	if err := run.Execute(); err != nil {
		return errors.Wrap(err, "running attestation")
	}

	// Check the expected artifacts
	if len(statement.Subject) == 0 {
		logrus.Warn("Provenance statement does not have any subjects, not verifying artifacts")
		return nil
	}

	logrus.Infof("Checking %d artifacts from the build", len(statement.Subject))
	for _, sub := range statement.Subject {
		s256, err := hash.SHA256ForFile(filepath.Join(b.opts.Workdir, sub.Name))
		if err != nil {
			return errors.Wrapf(err, "checking hash for %s ", sub.Name)
		}

		s512, err := hash.SHA512ForFile(filepath.Join(b.opts.Workdir, sub.Name))
		if err != nil {
			return errors.Wrapf(err, "checking hash for %s ", sub.Name)
		}

		if s256 != sub.Digest["sha256"] {
			return errors.Errorf("SHA256 for %s does not match", sub.Name)
		}

		if s512 != sub.Digest["sha512"] {
			return errors.Errorf("SHA512 for %s does not match", sub.Name)
		}
	}

	logrus.Info("🎉 Build has been reproduced successfully!")

	return nil
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
	Workdir       string
	Source        string            // Source is the URL for the code repository
	EnvVars       map[string]string // Variables to set when running
	ProvenanceDir string            // FIrectory to save the provenance attestations
	ConfigFile    string            // If the build was bootstarpped from a build, this is it
	ConfigPoint   string            // git ref of the config file
	Transfers     []TransferConfig  // List of artifacts to transfer
	Artifacts     ArtifactsConfig   // A list of expected artifacts to be produced by the build
	Materials     MaterialsConfig   // List of materials to use for the build
}

var DefaultOptions = &Options{
	Workdir: ".", // Working directory where the build runs
	Artifacts: ArtifactsConfig{
		Files:  []string{},
		Images: []string{},
	},
	EnvVars: map[string]string{},
}

// Options returns the build's option set
func (b *Build) Options() *Options {
	return b.opts
}

// setRunnerOptions sets the runner options
func (b *Build) setRunnerOptions() {
	b.runner.Options().Workdir = b.Options().Workdir
	b.runner.Options().EnvVars = b.Options().EnvVars
	b.runner.Options().ProvenanceDir = b.Options().ProvenanceDir
	b.runner.Options().Replacements = b.Replacements
	b.runner.Options().Source = b.Options().Source
	b.runner.Options().ConfigFile = b.Options().ConfigFile
	b.runner.Options().ConfigPoint = b.Options().ConfigPoint
	for i := range b.runner.Options().Replacements {
		b.runner.Options().Replacements[i].Workdir = b.Options().Workdir
	}
}

// Run creates a new run
func (b *Build) Run() *Run {
	opts := DefaultRunOptions
	opts.Transfers = b.Options().Transfers
	opts.Materials = b.Options().Materials
	opts.Artifacts = b.opts.Artifacts
	opts.ForceBuild = true
	return b.RunWithOptions(opts)
}

func (b *Build) RunWithOptions(opts *RunOptions) *Run {
	// Set the runner options
	b.setRunnerOptions()

	// Create the new run
	run := NewRun(b.runner)
	run.opts = opts

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
	logrus.Infof("Runner (%s) parameters: %+v", conf.Runner.ID, conf.Runner.Parameters)
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
			Value:         rdata.Value,
			Paths:         rdata.Paths,
			PathsRequired: rdata.RequiredPaths,
			Required:      rdata.Required,
		}
		reps = append(reps, rep)
	}
	b.Replacements = reps

	for _, e := range conf.Env {
		b.runner.Options().EnvVars[e.Var] = e.Value
	}

	b.Options().ProvenanceDir = conf.ProvenanceDir
	b.Options().ConfigFile = path          // Check if its normalized to the repo dir
	b.Options().Transfers = conf.Transfers // Artifacts to transfer out
	b.Options().Materials = conf.Materials // List of the build materials

	// Assign the env variables found in the config
	b.Options().EnvVars = map[string]string{}
	for _, e := range conf.Env {
		b.Options().EnvVars[e.Var] = e.Value
	}

	if conf.Artifacts.Files != nil {
		if conf.Artifacts.Files != nil {
			b.Options().Artifacts = conf.Artifacts
		}
	}

	// Check if the file lives in a git repo and read the commit
	if err := command.NewWithWorkDir(
		filepath.Dir(path), "git", "remote",
	).RunSilentSuccess(); err == nil {
		// Get the current build point:
		output, err := command.NewWithWorkDir(
			filepath.Dir(path), "git", "log", "--pretty=format:%H", "-n1",
		).RunSilentSuccessOutput()
		if err != nil {
			return errors.Wrap(err, "getting commit for configuration version")
		}
		b.Options().ConfigPoint = output.OutputTrimNL()
		logrus.Infof("Recording build configuration at %s", b.Options().ConfigPoint)
	} else {
		logrus.Info("Build config not in a git repo. Not reading commit.")
	}

	return nil
}

// digestSetForFile reads a file and produces a digestSet
// for subjects and material attestations
func digestSetForFile(filePath string) (hashes map[string]string, err error) {
	// Creat the function set to iterate
	fs := map[string]func(string) (string, error){
		"sha1":   hash.SHA1ForFile,
		"sha256": hash.SHA256ForFile,
		"sha512": hash.SHA512ForFile,
	}

	hashes = map[string]string{}
	for algo, fn := range fs {
		h, err := fn(filePath)
		if err != nil {
			return nil, errors.Wrapf(err, "generating digestset for %s", filePath)
		}
		hashes[algo] = h
	}
	return hashes, err
}
