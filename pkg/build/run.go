// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	v02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	"github.com/mattermost/cicd-sdk/pkg/build/runners"
	"github.com/mattermost/cicd-sdk/pkg/object"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/command"
	"sigs.k8s.io/release-utils/hash"
	"sigs.k8s.io/release-utils/util"
)

var (
	RUNSUCCESS = true
	RUNFAIL    = false
)

// Run asbtracts a build run
type Run struct {
	impl      runImplementation
	id        int
	opts      *RunOptions
	Created   time.Time
	StartTime time.Time
	EndTime   time.Time
	runner    runners.Runner
	output    string
	isSuccess *bool
}

// RunOptions control specific bits of a build run
type RunOptions struct {
	BuildPoint string
	Transfers  []TransferConfig // Artifacts to transfer out
}

var DefaultRunOptions = &RunOptions{}

// NewRun creates a new running specified an options set
func NewRun(runner runners.Runner) *Run {
	return &Run{
		impl:    &defaultRunImplementation{},
		runner:  runner,
		opts:    DefaultRunOptions,
		Created: time.Now(),
	}
}

func (r *Run) ID() string {
	return fmt.Sprintf("%s-%04d", r.runner.ID(), r.id)
}

func (r *Run) Output() string {
	return r.output
}

func (r *Run) setRunnerOptions() {
	r.runner.Options().BuildPoint = r.opts.BuildPoint
}

// Execute executes the run
func (r *Run) Execute() error {
	if r.isSuccess != nil {
		logrus.Warnf("Run #%s already ran", r.ID())
		return nil
	}

	// Record the start time
	r.StartTime = time.Now()

	// Defer setting the status and endtime
	defer func() {
		r.EndTime = time.Now()
		if r.isSuccess == nil {
			r.isSuccess = &RUNFAIL
		}
	}()

	r.setRunnerOptions()

	// Checkout the build point
	if err := r.impl.checkoutBuildPoint(r); err != nil {
		return errors.Wrapf(err, "checking out build point %s", r.runner.Options().BuildPoint)
	}

	// Process the run replacements
	if err := r.impl.processReplacements(r.runner.Options()); err != nil {
		logrus.Error("Error applying replacement data")
		return errors.Wrap(err, "applying run replacement data")
	}

	// Call the runner Run method to execute the build
	err := r.runner.Run()
	r.output = r.runner.Output()
	if err != nil {
		logrus.Errorf("[exec error in run #%s] %s", r.ID(), err)
		return errors.Wrapf(err, "[exec error in run #%s]", r.ID())
	}

	if err := r.impl.checkExpectedArtifacts(r.runner.Options()); err != nil {
		logrus.Error("Error verifying expected artifacts")
		return errors.Wrap(err, "verifying artifacts")
	}

	r.isSuccess = &RUNSUCCESS

	if r.impl.sendTransfers(r) != nil {
		return errors.Wrap(err, "processing artifact transfers")
	}

	// TODO(@puerco): normalize provenance artifacts to their
	// transferred locations
	if r.impl.writeProvenance(r) != nil {
		return errors.Wrap(err, "writing provenance metadata")
	}

	return nil
}

func (r *Run) Provenance() (*intoto.ProvenanceStatement, error) {
	return r.impl.provenance(r)
}

type runImplementation interface {
	processReplacements(*runners.Options) error
	checkExpectedArtifacts(opts *runners.Options) error
	provenance(*Run) (*intoto.ProvenanceStatement, error)
	writeProvenance(*Run) error
	checkoutBuildPoint(*Run) error
	sendTransfers(*Run) error
}

type defaultRunImplementation struct{}

// processReplacements applies all replacements defined for the run
func (dri *defaultRunImplementation) processReplacements(opts *runners.Options) error {
	if opts.Replacements == nil || len(opts.Replacements) == 0 {
		logrus.Info("Run has no replacements defined")
		return nil
	}
	for i, r := range opts.Replacements {
		if err := r.Apply(); err != nil {
			return errors.Wrapf(err, "applying replacement #%d", i)
		}
	}
	return nil
}

// checkExpectedArtifacts verifies a list of expected artifacts
func (dri *defaultRunImplementation) checkExpectedArtifacts(opts *runners.Options) error {
	if opts.ExpectedArtifacts == nil {
		logrus.Info("Run has no expected artifacts")
		return nil
	}
	for _, path := range opts.ExpectedArtifacts {
		if !util.Exists(filepath.Join(opts.Workdir, path)) {
			return errors.Errorf("expected artifact not found: %s", path)
		}
	}
	logrus.Infof("Successfully confirmed %d expected artifacts", len(opts.ExpectedArtifacts))
	return nil
}

func (dri *defaultRunImplementation) provenance(run *Run) (*intoto.ProvenanceStatement, error) {
	// Generate the environment struct
	envData := map[string]string{}
	for v, val := range run.runner.Options().EnvVars {
		envData[v] = val
	}

	// Add the parameters
	statement := intoto.ProvenanceStatement{
		StatementHeader: intoto.StatementHeader{
			Type:          intoto.StatementInTotoV01,
			PredicateType: v02.PredicateSLSAProvenance,
			Subject:       []intoto.Subject{},
		},
		Predicate: v02.ProvenancePredicate{
			Builder: v02.ProvenanceBuilder{
				ID: BuilderID,
			},
			BuildType: run.runner.ID(),
			Invocation: v02.ProvenanceInvocation{
				ConfigSource: v02.ConfigSource{},
				Parameters:   run.runner.Arguments(),
				Environment:  envData,
			},
			BuildConfig: nil,
			Metadata: &v02.ProvenanceMetadata{
				BuildInvocationID: "",
				BuildStartedOn:    &run.StartTime,
				BuildFinishedOn:   &run.EndTime,
				Completeness:      v02.ProvenanceComplete{},
				Reproducible:      false,
			},
			// The first material is the source code
			Materials: []v02.ProvenanceMaterial{},
		},
	}

	if run.runner.Options().Source != "" && run.runner.Options().BuildPoint != "" {
		statement.Predicate.Materials = append(statement.Predicate.Materials, v02.ProvenanceMaterial{
			URI: "git+" + run.runner.Options().Source,
			Digest: map[string]string{
				"sha1": run.runner.Options().BuildPoint,
			},
		})
	} else {
		logrus.Warn("Source code and/or buildpint not set. Not adding to predicate materials")
	}

	for _, path := range run.runner.Options().ExpectedArtifacts {
		ch256, err := hash.SHA256ForFile(filepath.Join(run.runner.Options().Workdir, path))
		if err != nil {
			return nil, errors.Wrap(err, "hashing expected artifacts to provenance subject")
		}

		ch512, err := hash.SHA512ForFile(filepath.Join(run.runner.Options().Workdir, path))
		if err != nil {
			return nil, errors.Wrap(err, "hashing expected artifacts to provenance subject")
		}

		sub := intoto.Subject{
			Name: path,
			Digest: map[string]string{
				"sha256": ch256,
				"sha512": ch512,
			},
		}

		statement.StatementHeader.Subject = append(
			statement.StatementHeader.Subject, sub,
		)
	}

	// Add the configuration file if we have one
	if run.runner.Options().ConfigFile != "" {
		statement.Predicate.Invocation.ConfigSource = v02.ConfigSource{
			URI: strings.TrimPrefix(run.runner.Options().ConfigFile, run.runner.Options().Workdir),
		}

		// If the rundata has the git config point, record it
		if run.runner.Options().ConfigPoint != "" {
			statement.Predicate.Invocation.ConfigSource.Digest = map[string]string{
				"sha1": run.runner.Options().ConfigPoint,
			}
		}
	}

	return &statement, nil
}

// writeProvenance outputs the provenance metadata to the
// specified directory.
func (dri *defaultRunImplementation) writeProvenance(r *Run) error {
	if r.runner.Options().ProvenanceDir == "" {
		logrus.Info("Not writing provenance metadata")
		return nil
	}

	// Generate the attestation
	statement, err := dri.provenance(r)
	if err != nil {
		return errors.Wrap(err, "generating provenance attestation")
	}
	data, err := json.MarshalIndent(statement, "", "  ")
	if err != nil {
		logrus.Fatal(errors.Wrap(err, "marshalling provenance attestation"))
	}

	filename := filepath.Join(
		r.runner.Options().ProvenanceDir,
		fmt.Sprintf("provenance-%d-%s.json", os.Getpid(), r.ID()),
	)
	if err := os.WriteFile(filename, data, os.FileMode(0o644)); err != nil {
		return errors.Wrap(err, "writing provenance metadata to file")
	}
	logrus.Infof("Provenance metadata written to %s", filename)
	return nil
}

func (dri *defaultRunImplementation) checkoutBuildPoint(r *Run) error {
	// If buildpoint is blank, we assume we are about to run the
	// build at HEAD. Here, we get the HEAD commit sha to record
	// it in the provenance attestation.
	if r.runner.Options().BuildPoint == "" {
		logrus.Info("BuildPoint not set, building at HEAD")

		// Get the current build point:
		output, err := command.NewWithWorkDir(
			r.runner.Options().Workdir,
			"git", "log", "--pretty=format:%H", "-n1",
		).RunSilentSuccessOutput()
		if err != nil {
			return errors.Wrap(err, "getting HEAD commit for build point")
		}
		commitSha := output.OutputTrimNL()
		r.runner.Options().BuildPoint = commitSha
		logrus.Infof("HEAD commit is %s", commitSha)
		return nil
	}

	// Otherwise, we checkout the commit specified by BuildPoint
	// to run the build at that point in the GIT history.
	// Get the current build point:
	if err := command.NewWithWorkDir(
		r.runner.Options().Workdir,
		"git", "checkout", r.runner.Options().BuildPoint,
	).RunSilentSuccess(); err != nil {
		return errors.Wrapf(err, "checking out build point (commit %s)", r.runner.Options().BuildPoint)
	}

	return nil
}

// sendTransfers copy the specified artifacts to their destinations
func (dri *defaultRunImplementation) sendTransfers(r *Run) error {
	if r.opts.Transfers == nil || len(r.opts.Transfers) == 0 {
		logrus.Info("No artifact transfers defined in run")
		return nil
	}

	// Create a new object manager to transfer the artifacts
	manager := object.NewManager()

	for _, td := range r.opts.Transfers {
		for _, f := range td.Source {
			if err := manager.Copy(
				"file://"+filepath.Join(r.runner.Options().Workdir, f),
				td.Destination,
			); err != nil {
				return errors.Wrap(err, "processing transfer")
			}
		}
	}
	return nil
}
