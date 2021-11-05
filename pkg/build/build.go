// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

// New returns a new build with the default options
func New(runner Runner) *Build {
	return NewWithOptions(runner, DefaultOptions)
}

func NewWithOptions(runner Runner, opts *Options) *Build {
	return &Build{
		runner: runner,
		opts:   opts,
	}
}

type Build struct {
	runner       Runner
	opts         *Options
	Runs         []*Run
	Replacements *[]Replacement
}

type Options struct {
	Workdir string
}

var DefaultOptions = &Options{
	Workdir: ".", // Working directory where the build runs
}

// Options returns the build's option set
func (b *Build) Options() *Options {
	return b.opts
}

// Run creates a new run
func (b *Build) Run() *Run {
	// Set the runner options
	b.runner.Options().Workdir = b.Options().Workdir

	// Create the new run
	run := NewRun(b.runner)

	// The ID is the new run position in the run array:
	run.ID = len(b.Runs)
	b.Runs = append(b.Runs, run)

	return run
}
