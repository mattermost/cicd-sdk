// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package runners

import "github.com/mattermost/cicd-sdk/pkg/build"

type baseRunner struct {
	output string
	opts   *build.RunnerOptions
}

func (br *baseRunner) Output() string {
	return br.output
}

func (br *baseRunner) Options() *build.RunnerOptions {
	return br.opts
}
