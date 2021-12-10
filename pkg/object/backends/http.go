package backends

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"sigs.k8s.io/release-utils/hash"
	"sigs.k8s.io/release-utils/util"
)

const (
	URLPrefixHTTP  = "http://"
	URLPrefixHTTPS = "https://"
)

type ObjectBackendHTTP struct{}

func NewHTTPWithOptions(opts *Options) *ObjectBackendHTTP {
	// Create the new configuration for the client
	return &ObjectBackendHTTP{}
}

func (h *ObjectBackendHTTP) Prefixes() []string {
	return []string{
		URLPrefixHTTP,
		URLPrefixHTTPS,
	}
}

func (h *ObjectBackendHTTP) URLPrefix() string {
	return URLPrefixHTTPS
}

func (h *ObjectBackendHTTP) CopyObject(srcURL, destURL string) (err error) {
	if strings.HasPrefix(srcURL, URLPrefixFilesystem) {
		return errors.New("unable to upload to http server")
	}
	if strings.HasPrefix(destURL, URLPrefixFilesystem) {
		// Read the path from the URL
		path := "/" + strings.TrimPrefix(destURL, URLPrefixFilesystem)
		var localFile *os.File
		if util.Exists(path) {
			s, err := os.Stat(path)
			if err != nil {
				return errors.Wrap(err, "checking destination path")
			}

			if s.IsDir() {
				u, err := url.Parse(srcURL)
				if err != nil {
					return errors.Wrap(err, "parsing source URL")
				}
				filename := filepath.Base(u.Path)
				if filename == "" {
					filename = "index"
				}
				path = filepath.Join(path, filename)
			}
			localFile, err = os.Create(path)
			if err != nil {
				return errors.Wrap(err, "creating destination file")
			}
		} else {
			// Create the file
			localFile, err = os.Create(path)
			if err != nil {
				return err
			}
		}
		defer localFile.Close()

		// Fetch the URL
		resp, err := http.Get(srcURL) //nolint:gosec // This is supposed to be variable
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Write the body to file
		if _, err = io.Copy(localFile, resp.Body); err != nil {
			return errors.Wrap(err, "writing data to local file")
		}
		return nil
	}
	return errors.New("Cloud to cloud copy is not supported yet")
}

func (h *ObjectBackendHTTP) PathExists(objectURL string) (bool, error) {
	resp, err := http.Head(objectURL) //nolint:gosec // This is supposed to be variable
	if err != nil {
		return false, errors.Wrap(err, "checking if remote URL exists")
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotFound:
		return false, nil
	case http.StatusOK:
		return true, nil
	}

	return false, errors.Errorf("unable to interpret HTTP response code %d", resp.StatusCode)
}

func (h *ObjectBackendHTTP) GetObjectHash(objectURL string) (hashes map[string]string, err error) {
	// Download to a temporary directory to check
	f, err := os.CreateTemp("", "temp-downloader-")
	if err != nil {
		return nil, errors.Wrap(err, "creating temp file")
	}

	if err := h.CopyObject(objectURL, URLPrefixFilesystem+objectURL[1:]); err != nil {
		return nil, errors.Wrap(err, "downloading temporary file")
	}

	fs := map[string]func(string) (string, error){
		"sha1":   hash.SHA1ForFile,
		"sha256": hash.SHA256ForFile,
		"sha512": hash.SHA512ForFile,
	}

	hashes = map[string]string{}
	for algo, fn := range fs {
		h, err := fn(f.Name())
		if err != nil {
			return nil, errors.Wrapf(err, "generating %s for object", objectURL)
		}
		hashes[algo] = h
	}
	return hashes, nil
}
