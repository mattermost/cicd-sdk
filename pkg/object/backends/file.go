// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package backends

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/util"
)

const URLPrefixFilesystem = "file://"

type Filesystem struct{}

var filePrefixes = []string{URLPrefixFilesystem}

func NewFilesystemWithOptions(opts *Options) *Filesystem {
	return &Filesystem{}
}

func (fsb *Filesystem) URLPrefix() string {
	return URLPrefixFilesystem
}

func (fsb *Filesystem) Prefixes() []string {
	return filePrefixes
}

func (fsb *Filesystem) CopyObject(srcURL, destURL string) error {
	srcPath := filepath.Join(string(filepath.Separator), strings.TrimPrefix(srcURL, URLPrefixFilesystem))
	destPath := filepath.Join(string(filepath.Separator), strings.TrimPrefix(destURL, URLPrefixFilesystem))

	logrus.Infof("Copying %s to %s in local filesystem", srcPath, destPath)

	sourceFileStat, err := os.Stat(srcPath)
	if err != nil {
		return errors.Wrap(err, "reading source stat info")
	}

	if !sourceFileStat.Mode().IsRegular() {
		return errors.Errorf("%s is not a regular file.", srcURL)
	}

	source, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrap(err, "opening source file")
	}
	defer source.Close()

	destination, err := os.Create(destPath)
	if err != nil {
		return errors.Wrap(err, "creating destination file")
	}
	defer destination.Close()

	buf := make([]byte, 65536)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return errors.Wrap(err, "reading source file")
		}
		if n == 0 {
			break
		}
		if _, err := destination.Write(buf[:n]); err != nil {
			return errors.Wrap(err, "writing buffer to destination file")
		}
	}
	return err
}

func (fsb *Filesystem) PathExists(path string) (bool, error) {
	path = "/" + strings.TrimPrefix(path, URLPrefixFilesystem)
	return util.Exists(path), nil
}
