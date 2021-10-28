// Copyright (c) 2017-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package github

type Issue struct {
	impl      IssueImplementation
	RepoOwner string
	RepoName  string
	Username  string
	State     string
	Number    int
	Labels    []string
}

type IssueImplementation interface{}
