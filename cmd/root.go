/*
Copyright 2022 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package cmd defines command line utilities for ghpc
package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/spf13/cobra"
)

// Git references when use Makefile
var (
	GitTagVersion  string
	GitBranch      string
	GitCommitInfo  string
	GitCommitHash  string
	GitInitialHash string
)

var (
	annotation = make(map[string]string)
	rootCmd    = &cobra.Command{
		Use:   "ghpc",
		Short: "A blueprint and deployment engine for HPC clusters in GCP.",
		Long: `gHPC provides a flexible and simple to use interface to accelerate
HPC deployments on the Google Cloud Platform.`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Help(); err != nil {
				log.Fatalf("cmd.Help function failed: %s", err)
			}
		},
		Version:     "v1.24.0",
		Annotations: annotation,
	}
)

// Execute the root command
func Execute() error {
	// Don't prefix messages with data & time to improve readability.
	// See https://pkg.go.dev/log#pkg-constants
	log.SetFlags(0)

	mismatch, branch, hash, dir := checkGitHashMismatch()
	if mismatch {
		fmt.Fprintf(os.Stderr, "WARNING: ghpc binary was built from a different commit (%s/%s) than the current git branch in %s (%s/%s). You can rebuild the binary by running 'make'\n",
			GitBranch, GitCommitHash[0:7], dir, branch, hash[0:7])
	}

	if len(GitCommitInfo) > 0 {
		if len(GitTagVersion) == 0 {
			GitTagVersion = "- not built from official release"
		}
		if len(GitBranch) == 0 {
			GitBranch = "detached HEAD"
		}
		annotation["version"] = GitTagVersion
		annotation["branch"] = GitBranch
		annotation["commitInfo"] = GitCommitInfo
		rootCmd.SetVersionTemplate(`ghpc version {{index .Annotations "version"}}
Built from '{{index .Annotations "branch"}}' branch.
Commit info: {{index .Annotations "commitInfo"}}
`)
	}
	return rootCmd.Execute()
}

func init() {}

// checkGitHashMismatch will compare the hash of the git repository vs the git
// hash the ghpc binary was compiled against, if the git repository if found and
// a mismatch is identified, then the function returns a positive bool along with
// the branch details, and false for all other cases.
func checkGitHashMismatch() (mismatch bool, branch, hash, dir string) {
	// binary does not contain build-time git info
	if len(GitCommitHash) == 0 {
		return false, "", "", ""
	}

	// could not find hpcToolkitRepo
	repo, dir, err := hpcToolkitRepo()
	if err != nil {
		return false, "", "", ""
	}

	// failed to open git
	head, err := repo.Head()
	if err != nil {
		return false, "", "", ""
	}

	// found hpc-toolkit git repo and hash does not match
	if GitCommitHash != head.Hash().String() {
		mismatch = true
		branch = head.Name().Short()
		hash = head.Hash().String()
		return
	}
	return false, "", "", ""
}

// hpcToolkitRepo will find the path of the directory containing the hpc-toolkit
// starting with the working directory and evaluating the parent directories until
// the toolkit repository is found. If the HPC Toolkit repository is not found by
// traversing the path, then the executable directory is checked.
func hpcToolkitRepo() (repo *git.Repository, dir string, err error) {
	// first look in the working directory and it's parents until a git repo is
	// found. If it's the hpc-toolkit repo, return it.
	// repo := new(git.Repository)
	dir, err = os.Getwd()
	if err != nil {
		return nil, "", err
	}
	subdir := filepath.Dir(dir)
	o := git.PlainOpenOptions{DetectDotGit: true}
	repo, err = git.PlainOpenWithOptions(dir, &o)
	if err == nil && isHpcToolkitRepo(*repo) {
		return
	} else if err == nil && !isHpcToolkitRepo(*repo) {
		// found a repo that is not the hpc-toolkit repo. likely a submodule
		// or another git repo checked out under ./hpc-toolkit. Keep walking
		// the parents' path to find the hpc-toolkit repo until we hit root of
		// filesystem
		for dir != subdir {
			dir = filepath.Dir(dir)
			subdir = filepath.Dir(dir)
			repo, err = git.PlainOpen(dir)

			if err == nil && isHpcToolkitRepo(*repo) {
				return repo, dir, nil
			}
		}
	}

	// fall back to the executable's directory
	e, err := os.Executable()
	if err != nil {
		return nil, "", err
	}
	dir = filepath.Dir(e)

	repo, err = git.PlainOpen(dir)
	if err != nil {
		return nil, "", err
	}
	if isHpcToolkitRepo(*repo) {
		return repo, dir, nil
	}
	return nil, "", errors.New("ghpc executable found in a git repo other than the hpc-toolkit git repo")
}

// isHpcToolkitRepo will verify that the found git repository has a commit with
// the known hash of the initial commit of the HPC Toolkit repository
func isHpcToolkitRepo(r git.Repository) bool {
	h := plumbing.NewHash(GitInitialHash)
	_, err := r.CommitObject(h)
	return err == nil
}
