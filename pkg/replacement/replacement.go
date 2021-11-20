package replacement

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	maxScanSize = 3145728
)

var errNoTag = errors.New("the replacement has no tag defined")

// Replacements
type Replacement struct {
	Tag           string
	Value         string
	Paths         []string
	PathsRequired bool // If true, the replacement will fail if path is not found
	Required      bool
	Workdir       string
}

type ReplacementSet []Replacement

func (r *Replacement) Apply() (err error) {
	if r.Tag == "" {
		return errNoTag
	}

	for _, path := range r.Paths {
		logrus.Infof("Replacing tags in %s", path)
		if r.Workdir != "" {
			path = filepath.Join(r.Workdir, path)
		}
		fileData, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if r.PathsRequired {
					return errors.Errorf("required path %s not found", path)
				}
				continue
			} else {
				return errors.Wrapf(err, "while checking path %s", path)
			}
		}

		// Should skip maybe
		if fileData.Size() > maxScanSize {
			logrus.Warnf("File %s is too big to replace in memory", path)
		}

		fileContents, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrap(err, "opening file to replace tags")
		}
		originalSum := sha256.Sum256(fileContents)

		newData := bytes.ReplaceAll(fileContents, []byte(r.Tag), []byte(r.Value))
		newSum := sha256.Sum256(newData)

		// Check if anything was modified
		if newSum == originalSum {
			if r.Required {
				return errors.New("replacement is required, but no data was modified")
			}
			logrus.Debugf("No data modified for tag '%s' in path %s", r.Tag, path)
			continue
		}

		// Write the modified data
		if err := os.WriteFile(path, newData, fileData.Mode()); err != nil {
			return errors.Wrap(err, "writing replaced file")
		}
	}
	return nil
}

// IsPathReplaced checks an arbitrary path to see if the tag is found
func (r *Replacement) IsPathReplaced(path string) (bool, error) {
	if r.Tag == "" {
		return false, errNoTag
	}

	fileContents, err := os.ReadFile(path)
	if err != nil {
		return false, errors.Wrap(err, "opening file to replace tags")
	}

	return !bytes.Contains(fileContents, []byte(r.Tag)), nil
}

// Check checks if all paths have been replaced
func (r *Replacement) Check() (bool, error) {
	if r.Tag == "" {
		return false, errNoTag
	}

	// Range al paths to check
	for _, path := range r.Paths {
		fileData, err := os.Stat(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				if r.PathsRequired {
					return false, errors.Errorf("required path %s not found", path)
				}
				continue
			} else {
				return false, errors.Wrapf(err, "while checking path %s", path)
			}
		}

		// Should skip maybe
		if fileData.Size() > maxScanSize {
			logrus.Warnf("File %s is too big to replace in memory", path)
		}

		isr, err := r.IsPathReplaced(path)
		if err != nil || !isr {
			return false, err
		}
	}

	return true, nil
}
