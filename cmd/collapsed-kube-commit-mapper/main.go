/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/golang/glog"

	"k8s.io/publishing-bot/pkg/cache"
	"k8s.io/publishing-bot/pkg/git"
)

func Usage() {
	fmt.Fprintf(os.Stderr, `Print a lookup table by printing each mainline k8s.io/kubernetes
commit hash with its corresponding commit hash in the current branch
(which is the result of a "git filter-branch --sub-directory"). It is
expected that the commit messages on the current branch contain a
"Kubernetes-commit: <upstream commit>" line for the directly corresponding
commit. Note, that a number of k8s.io/kubernetes mainline commits might
be collapsed during filtering:

    HEAD <source-branch>
     |          |
     H'<--------H
     z          |
     y         ,G
     F'<------*-F
     |        ,-E
     x       / ,D
     |      / / |
     C'<----**--C
     j          |
     i <----*   |
             \--B
              '-A

The sorted output looks like this:

    <sha of A> <sha of i>
    <sha of B> <sha of j>
    <sha of C> <sha of C'>
    <sha of D> <sha of C'>
    <sha of E> <sha of C'>
    <sha of F> <sha of F'>
    <sha of G> <sha of F'>
    <sha of H> <sha of H'>
    ...

Usage: %s --source-branch <source-branch> [-l] [--commit-message-tag <Commit-message-tag>]
`, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	commitMsgTag := flag.String("commit-message-tag", "Kubernetes-commit", "the git commit message tag used to point back to source commits")
	sourceBranch := flag.String("source-branch", "", "the source branch (fully qualified e.g. refs/remotes/origin/master) used as the filter-branch basis")
	showMessage := flag.Bool("l", false, "list the commit message after the two hashes")

	flag.Usage = Usage
	flag.Parse()

	if *sourceBranch == "" {
		glog.Fatalf("source-branch cannot be empty")
	}

	// open repo at "."
	r, err := gogit.PlainOpen(".")
	if err != nil {
		glog.Fatalf("Failed to open repo at .: %v", err)
	}

	// get HEAD
	dstRef, err := r.Head()
	if err != nil {
		glog.Fatalf("Failed to open HEAD: %v", err)
	}
	dstHead, err := cache.CommitObject(r, dstRef.Hash())
	if err != nil {
		glog.Fatalf("Failed to resolve HEAD: %v", err)
	}

	// get first-parent commit list of upstream branch
	srcUpstreamBranch, err := r.ResolveRevision(plumbing.Revision(*sourceBranch))
	if err != nil {
		glog.Fatalf("Failed to open upstream branch %s: %v", *sourceBranch, err)
	}
	srcHead, err := cache.CommitObject(r, *srcUpstreamBranch)
	if err != nil {
		glog.Fatalf("Failed to open upstream branch %s head: %v", *sourceBranch, err)
	}
	srcFirstParents, err := git.FirstParentList(r, srcHead)
	if err != nil {
		glog.Fatalf("Failed to get upstream branch %s first-parent list: %v", *sourceBranch, err)
	}

	// get first-parent commit list of HEAD
	dstFirstParents, err := git.FirstParentList(r, dstHead)
	if err != nil {
		glog.Fatalf("Failed to get first-parent commit list for %s: %v", dstHead.Hash, err)
	}

	sourceCommitToDstCommits, err := git.SourceCommitToDstCommits(r, *commitMsgTag, dstFirstParents, srcFirstParents)
	if err != nil {
		glog.Fatalf("Failed to map upstream branch %s to HEAD: %v", *sourceBranch, err)
	}

	// print out a look-up table
	// <kube sha> <dst sha>
	var lines []string
	for kh, dh := range sourceCommitToDstCommits {
		if *showMessage {
			c, err := cache.CommitObject(r, kh)
			if err != nil {
				// if this happen something above in the algorithm is broken
				glog.Fatalf("Failed to find k8s.io/kubernetes commit %s", kh)
			}
			lines = append(lines, fmt.Sprintf("%s %s %s", kh, dh, strings.SplitN(c.Message, "\n", 2)[0]))
		} else {
			lines = append(lines, fmt.Sprintf("%s %s", kh, dh))
		}
	}
	sort.Strings(lines) // sort to allow binary search
	for _, l := range lines {
		fmt.Println(l)
	}
}
