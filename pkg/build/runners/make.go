// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package runners

import (
	"fmt"

	"sigs.k8s.io/release-utils/command"
)

// https://git.internal.mattermost.com/mattermost/ci/mattermost-server/-/blob/master/master/te.yml

const (
	makeCmd     = "make"
	makeMoniker = "make"
)

func init() {
	Catalog[makeMoniker] = NewMake
}

type Make struct {
	baseRunner
	args []string
}

func NewMake(args ...string) Runner {
	return &Make{
		baseRunner: baseRunner{
			id:     makeMoniker,
			output: "",
			opts:   DefaultOptions,
			args:   args,
		},
	}
}

// Run executes make
func (m *Make) Run() error {
	envStr := []string{}
	for v, val := range m.Options().EnvVars {
		envStr = append(envStr, fmt.Sprintf("%s=%s", v, val))
	}

	cmd := command.NewWithWorkDir(m.Options().Workdir, makeCmd, m.args...).Env(envStr...)

	output, err := cmd.RunSuccessOutput()
	if err != nil {
		return err
	}

	m.output = output.Output()

	return nil
}
