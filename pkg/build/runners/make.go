// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package runners

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
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
}

func NewMake(args ...string) Runner {
	return &Make{
		baseRunner: baseRunner{
			id:   makeMoniker,
			opts: DefaultOptions,
			args: args,
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

	if m.Options().Log != "" {
		oLog, err := os.Create(m.Options().Log)
		if err != nil {
			return errors.Wrap(err, "opening output log")
		}
		cmd.AddOutputWriter(oLog)
	}

	if m.Options().ErrorLog != "" {
		eLog, err := os.Create(m.Options().ErrorLog)
		if err != nil {
			return errors.Wrap(err, "opening error log")
		}
		cmd.AddOutputWriter(eLog)
	}

	if err := cmd.RunSuccess(); err != nil {
		return err
	}

	return nil
}
