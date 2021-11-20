// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package runners

import (
	"github.com/mattermost/cicd-sdk/pkg/replacement"
	"github.com/pkg/errors"
)

type Runner interface {
	ID() string
	Run() error
	Output() string
	Options() *Options
	Arguments() []string
}

type Options struct {
	Workdir           string
	ProvenanceDir     string
	EnvVars           map[string]string
	ExpectedArtifacts []string
	Replacements      []replacement.Replacement
}

var DefaultOptions = &Options{
	Workdir: ".",
}

var Catalog = make(map[string]func(args ...string) Runner)

func New(builderID string, args ...string) (Runner, error) {
	if _, ok := Catalog[builderID]; !ok {
		return nil, errors.Errorf("no runner with id '%s' found", builderID)
	}
	runner := Catalog[builderID](args...)
	if runner == nil {
		return nil, errors.Errorf("unable to initialize new runner")
	}

	return runner, nil
}

type baseRunner struct {
	id     string
	output string
	args   []string
	opts   *Options
}

func (br *baseRunner) ID() string {
	return br.id
}

func (br *baseRunner) Output() string {
	return br.output
}

func (br *baseRunner) Options() *Options {
	return br.opts
}

func (br *baseRunner) Arguments() []string {
	return br.args
}
