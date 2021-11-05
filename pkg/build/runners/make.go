// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package runners

import (
	"github.com/mattermost/cicd-sdk/pkg/build"
	"sigs.k8s.io/release-utils/command"
)

// https://git.internal.mattermost.com/mattermost/ci/mattermost-server/-/blob/master/master/te.yml

const makeCmd = "make"

type Make struct {
	baseRunner
	args []string
}

func NewMake(args ...string) *Make {
	return &Make{
		baseRunner: baseRunner{
			output: "",
			opts:   build.DefaultRunnerOptions,
		},
		args: args,
	}
}

// Run executes make
func (m *Make) Run() error {
	output, err := command.NewWithWorkDir(m.Options().Workdir, makeCmd, m.args...).RunSuccessOutput()
	if err != nil {
		return err
	}

	m.output = output.Output()

	return nil
}
