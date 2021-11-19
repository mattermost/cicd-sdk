// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

import (
	"path/filepath"
	"time"

	intoto "github.com/in-toto/in-toto-golang/in_toto"
	v02 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v0.2"
	"github.com/mattermost/cicd-sdk/pkg/build/runners"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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
	ID        int
	Created   time.Time
	StartTime time.Time
	EndTime   time.Time
	runner    runners.Runner
	output    string
	isSuccess *bool
}

// NewRun creates a new running specified an options set
func NewRun(runner runners.Runner) *Run {
	return &Run{
		impl:    &defaultRunImplementation{},
		runner:  runner,
		Created: time.Now(),
	}
}

func (r *Run) Output() string {
	return r.output
}

// Execute executes the run
func (r *Run) Execute() error {
	if r.isSuccess != nil {
		logrus.Warnf("Run #%d already run", r.ID)
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

	// Process the run replacements
	if err := r.impl.processReplacements(r.runner.Options()); err != nil {
		logrus.Error("Error applying replacement data")
		return errors.Wrap(err, "applying run replacement data")
	}

	// Call the runner Run method to execute the build
	err := r.runner.Run()
	r.output = r.runner.Output()
	if err != nil {
		logrus.Errorf("[exec error in run #%d] %s", r.ID, err)
		return errors.Wrapf(err, "[exec error in run #%d]", r.ID)
	}

	if err := r.impl.checkExpectedArtifacts(r.runner.Options()); err != nil {
		logrus.Error("Error verifying expected artifacts")
		return errors.Wrap(err, "verifying artifacts")
	}

	r.isSuccess = &RUNSUCCESS
	return nil
}

func (r *Run) Provenance() (*intoto.ProvenanceStatement, error) {
	return r.impl.provenance(r)
}

type runImplementation interface {
	processReplacements(*runners.Options) error
	checkExpectedArtifacts(opts *runners.Options) error
	provenance(*Run) (*intoto.ProvenanceStatement, error)
}

type defaultRunImplementation struct{}

// processReplacements applies all replacements defined for the run
func (dri *defaultRunImplementation) processReplacements(opts *runners.Options) error {
	if opts.Replacements == nil || len(*opts.Replacements) == 0 {
		logrus.Info("Run has no replacements defined")
		return nil
	}
	for i, r := range *opts.Replacements {
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
	var envData = map[string]string{}
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
			Materials: []v02.ProvenanceMaterial{},
		},
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

	return &statement, nil
}
