package main

import (
	"github.com/mattermost/cicd-sdk/pkg/build"
	"github.com/mattermost/cicd-sdk/pkg/build/runners"
	"github.com/mattermost/cicd-sdk/pkg/replacement"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func main() {
	b := build.NewWithOptions(runners.NewMake("compile"), &build.Options{
		Workdir:           "/home/urbano/Projects/Mattermost/cicd-sdk/tmp/build-sample",
		ExpectedArtifacts: []string{"binary"},
		EnvVars:           map[string]string{"BRANCH": "main"},
		ProvenanceDir:     "/tmp/slsa",
	})

	// Add a sample replacement
	b.Replacements = append(b.Replacements, replacement.Replacement{
		Tag:   "%REPLACEME%",
		Value: ">> This text was successfully replaced <<",
		Paths: []string{
			"main.go",
		},
		PathsRequired: true,
		Required:      true,
	})

	r := b.Run()
	res := r.Execute()
	if res != nil {
		logrus.Fatal(errors.Wrap(res, "executing build"))
	}
	/*
		s, err := r.Provenance()
		if err != nil {
			logrus.Fatal(errors.Wrap(err, "generating provenance metadata"))
		}

		data, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			logrus.Fatal(errors.Wrap(err, "marshalling provenance statement"))
		}
		fmt.Println(string(data))
	*/
}
