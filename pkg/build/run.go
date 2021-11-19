// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

import (
	"path/filepath"
	"time"

	"github.com/mattermost/cicd-sdk/pkg/build/runners"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
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

type runImplementation interface {
	processReplacements(*runners.Options) error
	checkExpectedArtifacts(opts *runners.Options) error
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
