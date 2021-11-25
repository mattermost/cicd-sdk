package git

import (
	"fmt"
	"os"

	gogit "github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"sigs.k8s.io/release-utils/util"
)

const (
	gitCommand       = "git"
	githubDefaultURL = "git@github.com:%s/%s"
)

type Git struct {
	opts *Options
	impl gitImplementation
}

type Options struct{}

var defaultGitOptions = &Options{}

// New returns a new Git object with the default options
func New() *Git {
	return NewWithOptions(defaultGitOptions)
}

// NewWithOptions returns a git object with specific options
func NewWithOptions(opts *Options) *Git {
	return &Git{
		opts: opts,
		impl: &defaultGitImpl{},
	}
}

type gitImplementation interface {
	openRepo(path string) (repo *Repository, err error)
	cloneRepo(url, path string) (repo *Repository, err error)
}

func (g *Git) OpenRepo(path string) (repo *Repository, err error) {
	return g.impl.openRepo(path)
}

func (g *Git) CloneRepo(url, path string) (repo *Repository, err error) {
	return g.impl.cloneRepo(url, path)
}

// OpenOrCloneRepo
func (g *Git) OpenOrCloneRepo(url, path string) (repo *Repository, err error) {
	// If we have no path, work in a temp directory
	if path == "" {
		path, err = os.MkdirTemp("", "repo-clone-")
		if err != nil {
			return nil, errors.Wrap(err, "creating temporary directory")
		}
	}

	if util.Exists(path) {
		// todo(@puerco): Check the directory actually is a fork of the repo
		return g.impl.openRepo(path)
	}
	return g.impl.cloneRepo(url, path)
}

// nolint:revive // I don't want to call this HubURL
func GitHubURL(repoOwner, repoName string) string {
	return fmt.Sprintf(githubDefaultURL, repoOwner, repoName)
}

type defaultGitImpl struct{}

func (di *defaultGitImpl) openRepo(path string) (repo *Repository, err error) {
	gogitrepo, err := gogit.PlainOpen(path)
	if err != nil {
		return nil, errors.Wrap(err, "opening repository")
	}
	opts := defaultRepositoryOptions
	opts.Path = path
	repo = NewRepositoryWithOptions(opts)
	repo.SetClient(gogitrepo)
	return repo, nil
}

// cloneRepo clones a repository to `path` and returns it
func (di *defaultGitImpl) cloneRepo(url, path string) (repo *Repository, err error) {
	gogitrepo, err := gogit.PlainClone(path, false, &gogit.CloneOptions{
		URL: url,
	})
	if err != nil {
		return nil, errors.Wrap(err, "cloning repository")
	}
	opts := defaultRepositoryOptions
	opts.Path = path
	repo = NewRepositoryWithOptions(opts)
	repo.SetClient(gogitrepo)
	return repo, nil
}
