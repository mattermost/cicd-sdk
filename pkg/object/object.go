// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.
package object

import (
	"strings"

	"github.com/mattermost/cicd-sdk/pkg/object/backends"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Manager
type Manager struct {
	impl     ManagerImplementation
	Backends []backends.Backend
}

const URLPrefixFilesystem = "file://"

// NewObjectManager returns a new object manager with default options
func NewManager() *Manager {
	// Return a new object manager. It always includesd a file handler
	om := &Manager{
		impl:     &defaultManagerImpl{},
		Backends: []backends.Backend{},
	}
	// Add the implemented backends
	om.Backends = append(om.Backends,
		backends.NewFilesystemWithOptions(&backends.Options{}),
		backends.NewS3WithOptions(&backends.Options{}),
		backends.NewGitWithOptions(&backends.Options{}),
		backends.NewHTTPWithOptions(&backends.Options{}),
	)
	return om
}

// PathExists returns a bool that indicates if a path exists or not
func (om *Manager) PathExists(path string) (bool, error) {
	pathBackend, err := om.impl.GetURLBackend(om.Backends, path)
	if err != nil {
		return false, errors.Wrap(err, "getting URL backend")
	}

	return pathBackend.PathExists(path)
}

// Copy copies an object from a srcURL to a destination URL
func (om *Manager) Copy(srcURL, destURL string) (err error) {
	if srcURL == "" {
		return errors.New("unable to transfer file, no src url defined")
	}
	logrus.Infof("Transferring data from %s to %s", srcURL, destURL)
	srcBackend, err := om.impl.GetURLBackend(om.Backends, srcURL)
	if err != nil {
		return errors.Wrap(err, "getting backend for destination URL")
	}
	if srcBackend == nil {
		return errors.Errorf("No backend enabled for URL %s", srcURL)
	}
	dstBackend, err := om.impl.GetURLBackend(om.Backends, destURL)
	if err != nil {
		return errors.Wrap(err, "getting backend for destination backend")
	}
	if dstBackend == nil {
		return errors.Errorf("No backend enabled for URL %s", destURL)
	}

	// For now, we err no cloud to cloud copy operations
	if (dstBackend).URLPrefix() != URLPrefixFilesystem && (srcBackend).URLPrefix() != URLPrefixFilesystem {
		return errors.New("cloud to cloud operations are not yet supported")
	}

	if (srcBackend).URLPrefix() != URLPrefixFilesystem {
		return (srcBackend).CopyObject(srcURL, destURL)
	}
	return (dstBackend).CopyObject(srcURL, destURL)
}

// GetObjectHash returns the available hashes for an object
func (om *Manager) GetObjectHash(objectURL string) (map[string]string, error) {
	be, err := om.impl.GetURLBackend(om.Backends, objectURL)
	if err != nil {
		return nil, errors.Wrap(err, "getting backend for URL")
	}
	return be.GetObjectHash(objectURL)
}

type ManagerImplementation interface {
	GetURLBackend([]backends.Backend, string) (backends.Backend, error)
}

type defaultManagerImpl struct{}

// GetURLBackend returns the bakcend that can handle a specific URL
func (di *defaultManagerImpl) GetURLBackend(bs []backends.Backend, testURL string) (backends.Backend, error) {
	for _, backend := range bs {
		for _, prefix := range backend.Prefixes() {
			if strings.HasPrefix(testURL, prefix) {
				return backend, nil
			}
		}
	}
	return nil, nil
}
