// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package backends

type Options struct {
	ServiceOptions interface{}
}

type Backend interface {
	URLPrefix() string
	CopyObject(srcURL, destURL string) error
	Prefixes() []string
	PathExists(string) (bool, error)
	GetObjectHash(string) (map[string]string, error)
}
