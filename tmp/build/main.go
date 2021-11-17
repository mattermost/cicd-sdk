package main

import (
	"github.com/mattermost/cicd-sdk/pkg/build"
	"github.com/mattermost/cicd-sdk/pkg/build/runners"
)

func main() {
	b := build.NewWithOptions(runners.NewMake("compile"), &build.Options{
		Workdir:           "/home/urbano/Projects/Mattermost/cicd-sdk/tmp/build-sample",
		ExpectedArtifacts: []string{"binary"},
	})

	b.Run().Execute()
}
