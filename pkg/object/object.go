// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.
package object

import (
	"strings"

	"github.com/mattermost/cicd-sdk/pkg/object/backends"
	"github.com/pkg/errors"
)

// Manager
type Manager struct {
	impl     ManagerImplementation
	Backends []backends.Backend
}

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
	)
	return om
}

// Copy copies an object from a srcURL to a destination URL
func (om *Manager) Copy(srcURL, destURL string) (err error) {
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
	if (dstBackend).URLPrefix() != "file://" && (srcBackend).URLPrefix() != "file://" {
		return errors.New("cloud to cloud operations are not yet supported")
	}

	return (srcBackend).CopyObject(srcURL, destURL)
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