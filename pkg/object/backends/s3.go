// Copyright (c) 2021-present Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package backends

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	s3go "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/release-utils/hash"
)

const URLPrefixS3 = "s3://"

type ObjectBackendS3 struct {
	session session.Session
}

func NewS3WithOptions(opts *Options) *ObjectBackendS3 {
	// Create the new configuration for the client
	conf := &aws.Config{
		Region: aws.String(os.Getenv("AWS_DEFAULT_REGION")),
	}

	if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		logrus.Infof("No AWS credentials found in the environment, using anonnymous client")
		conf.Credentials = credentials.AnonymousCredentials
	}
	sess := session.Must(session.NewSession(conf))
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

func (s3 *ObjectBackendS3) splitBucketPath(locationURL string) (bucket, path string, err error) {
	u, err := url.Parse(locationURL)
	if err != nil {
		return bucket, path, errors.Wrap(err, "parsing source URL")
	}
	return u.Host, u.Path, nil
}

// copyRemoteLocal downloads a file from a bucket to the local filesystem
func (s3 *ObjectBackendS3) copyRemoteToLocal(source, destURL string) error {
	destPath := filepath.Join(string(filepath.Separator), strings.TrimPrefix(destURL, URLPrefixFilesystem))
	bucket, path, err := s3.splitBucketPath(source)
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
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		return errors.Wrapf(err, "failed to download file %s from %s", path, bucket)
	}
	logrus.Infof("Downloaded %d bytes to %s", n, destURL)
	return nil
}

// copyLocalToRemote copies a localfile to an s3 bucket
func (s3 *ObjectBackendS3) copyLocalToRemote(sourceURL, destURL string) error {
	srcPath := filepath.Join(string(filepath.Separator), strings.TrimPrefix(sourceURL, URLPrefixFilesystem))
	uploader := s3manager.NewUploader(&s3.session)
	bucket, path, err := s3.splitBucketPath(destURL)
	if err != nil {
		return errors.Wrap(err, "parsing source URL")
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return errors.Wrap(err, "opening local file")
	}
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
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

// PathExists checks if a path exosts in the filesystem
func (s3 *ObjectBackendS3) PathExists(nodeURL string) (bool, error) {
	bucket, path, err := s3.splitBucketPath(nodeURL)
	if err != nil {
		return false, errors.Wrap(err, "parsing node URL")
	}
	client := s3go.New(&s3.session)
	logrus.Debugf("Checking if %s exists in %s", path, bucket)
	if _, err := client.HeadObject(&s3go.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NotFound":
				return false, nil
			default:
				return false, err
			}
		}
		return false, err
	}
	return true, nil
}

// GetObjectHash returns a hash of a remote object. In S3, there are no
// APIs to get the file hash so we have to download and sum.
func (s3 *ObjectBackendS3) GetObjectHash(objectURL string) (hashes map[string]string, err error) {
	// Create a temporary directory to store the file
	f, err := os.CreateTemp("", "object-hashing-")
	if err != nil {
		return nil, errors.Wrap(err, "creating temporary file")
	}
	defer os.Remove(f.Name())

	if err := s3.copyRemoteToLocal(objectURL, "file:/"+f.Name()); err != nil {
		return nil, errors.Wrap(err, "downloading obkect from s3")
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
