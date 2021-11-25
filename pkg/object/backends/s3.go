// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package backends

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	s3go "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const URLPrefixS3 = "s3://"

type ObjectBackendS3 struct {
	session session.Session
}

func NewS3WithOptions(opts *Options) *ObjectBackendS3 {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_DEFAULT_REGION")),
	},
	))
	return &ObjectBackendS3{
		session: *sess,
	}
}

func (s3 *ObjectBackendS3) Prefixes() []string {
	return []string{URLPrefixS3}
}

func (s3 *ObjectBackendS3) URLPrefix() string {
	return URLPrefixS3
}

// copyRemoteLocal downloads a file from a bucket to the local filesystem
func (s3 *ObjectBackendS3) copyRemoteToLocal(source, destURL string) error {
	destPath := filepath.Join(string(filepath.Separator), strings.TrimPrefix(destURL, URLPrefixFilesystem))
	u, err := url.Parse(source)
	if err != nil {
		return errors.Wrap(err, "parsing source URL")
	}
	downloader := s3manager.NewDownloader(&s3.session)

	f, err := os.Create(destPath)
	if err != nil {
		return errors.Wrap(err, "opening destination file")
	}

	// Write the contents of S3 Object to the file
	n, err := downloader.Download(f, &s3go.GetObjectInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(u.Path),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to download file %s from %s", u.Path, u.Host)
	}
	logrus.Infof("Downloaded %d bytes to %s", n, destURL)
	return nil
}

// copyLocalToRemote copies a localfile to an s3 bucket
func (s3 *ObjectBackendS3) copyLocalToRemote(sourceURL, destURL string) error {
	srcPath := filepath.Join(string(filepath.Separator), strings.TrimPrefix(sourceURL, URLPrefixFilesystem))
	uploader := s3manager.NewUploader(&s3.session)
	u, err := url.Parse(destURL)
	if err != nil {
		return errors.Wrap(err, "parsing source URL")
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrap(err, "opening local file")
	}
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(u.Host),
		Key:    aws.String(u.Path),
		Body:   f,
	})
	return errors.Wrap(err, "uploading file")
}

func (s3 *ObjectBackendS3) CopyObject(srcURL, destURL string) error {
	if strings.HasPrefix(srcURL, URLPrefixFilesystem) {
		return s3.copyLocalToRemote(srcURL, destURL)
	}
	if strings.HasPrefix(destURL, URLPrefixFilesystem) {
		return s3.copyRemoteToLocal(srcURL, destURL)
	}
	return errors.New("CLoud to cloud copy is not supported yet")
}
