// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package build

type Runner interface {
	Run() error
	Output() string
	Options() *RunnerOptions
}

type RunnerOptions struct {
	Workdir           string
	EnvVars           map[string]string
	ExpectedArtifacts []string
	Replacements      *[]Replacement
}

var DefaultRunnerOptions = &RunnerOptions{
	Workdir: ".",
}
