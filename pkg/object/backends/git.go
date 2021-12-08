// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package backends

import (
	"regexp"
	"strings"

	"github.com/mattermost/cicd-sdk/pkg/git"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const URLPrefixGit = "git+"

// TODO(@puerco) this regexp must be corrected, not necesarilly the hash is the end
var revRegex = regexp.MustCompile("@([a-f0-9]{40})$")

type ObjectBackendGit struct{}

func NewGitWithOptions(opts *Options) *ObjectBackendGit {
	return &ObjectBackendGit{}
}

func (g *ObjectBackendGit) Prefixes() []string {
	return []string{URLPrefixGit}
}

func (g *ObjectBackendGit) URLPrefix() string {
	return URLPrefixGit
}

// copyRemoteLocal downloads a file from a bucket to the local filesystem
func (g *ObjectBackendGit) copyRemoteToLocal(source, destURL string) error {
	// Parse the URL to get the parts

	gc := git.New()
	// TODO: We need an algo to determine if we want a repository file. For now, only
	// referencing the whole repo will work.
	// See https://spdx.github.io/spdx-spec/package-information/#771-description
	rev := ""
	m := revRegex.FindAllString(source, 1)
	if len(m) > 0 {
		source = source[:len(source)-41]
		rev = m[0][1:]
		logrus.Infof("Cloning at revision %s", rev)
	}
	logrus.Infof("Cloning %s to %s", source, destURL)
	repo, err := gc.CloneRepo(
		strings.TrimPrefix(source, "git+"), strings.TrimPrefix(destURL, "file:/"),
	)
	if err != nil {
		return errors.Wrap(err, "performing git clone")
	}

	// If we hava revision, clone it
	if rev != "" {
		if err := repo.Checkout(rev); err != nil {
			return errors.Wrapf(err, "checking out revision %s", rev)
		}
	}
	return nil
}

func (g *ObjectBackendGit) copyLocalToRemote(srcURL, destURL string) error {
	return errors.New("Git does not support copying foles to remote")
}

// PathExists checks if a path exosts in the filesystem
func (g *ObjectBackendGit) PathExists(nodeURL string) (bool, error) {
	return false, errors.New("Path exists not implemented yet")
}

func (g *ObjectBackendGit) CopyObject(srcURL, destURL string) error {
	if strings.HasPrefix(srcURL, URLPrefixFilesystem) {
		return g.copyLocalToRemote(srcURL, destURL)
	}
	if strings.HasPrefix(destURL, URLPrefixFilesystem) {
		return g.copyRemoteToLocal(srcURL, destURL)
	}
	return errors.New("CLoud to cloud copy is not supported yet")
}

// GetObjectHash returns the hash of an object. In the case of data stored
// in a git repo, all artifacts return the hash of the repo commit
func (g *ObjectBackendGit) GetObjectHash(objectURL string) (hashes map[string]string, err error) {
	// First, lets try to get the hash from the URL itself
	m := revRegex.FindAllString(objectURL, 1)
	if len(m) > 0 {
		return map[string]string{"sha1": m[0][1:]}, nil
	}

	// If we were unable to fetch it from the URL, we have to query the repo
	// TODO(@puerco): Trim the URL of hashes and refs, recognize branch if included
	gc := git.New()
	output, err := gc.LsRemote(objectURL, "HEAD")
	if err != nil {
		return nil, errors.Wrap(err, "querying remote for HEAD hash")
	}
	parts := strings.Fields(output)
	return map[string]string{"sha1": parts[0]}, nil
}
