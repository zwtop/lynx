/*
Copyright 2021 The Lynx Authors.

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
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

func startInitRunner(logWriter io.Writer, remoteRepo, refspec string) (string, error) {
	var outBuff bytes.Buffer

	runner := exec.Command("/usr/local/bin/init-runner.sh", remoteRepo, refspec)
	runner.Stdout = io.MultiWriter(&outBuff, logWriter)
	runner.Stderr = logWriter

	if err := runner.Run(); err != nil {
		return "", err
	}

	commitHash := regexp.MustCompile(`(?m)^GIT_COMMIT_ID=[\w]+$`).FindString(outBuff.String())
	commitHash = strings.Split(commitHash, "=")[1]
	return commitHash, nil
}

func mustStartInitRunner(logWriter io.Writer, remoteRepo, refspec string, timeout time.Duration) string {
	var commitHash string

	err := wait.PollImmediate(time.Second, timeout, func() (done bool, err error) {
		klog.Infof("try to start init runner")
		fmt.Fprintln(logWriter, "=======================\nstart new init runner\n=======================")

		if commitHash, err = startInitRunner(logWriter, remoteRepo, refspec); err != nil {
			klog.Errorf("failed complete init runner: %s", err)
			return false, nil
		} else {
			return true, nil
		}
	})
	if err != nil {
		klog.Fatalf("failed to complete init runner: %s", err)
	}

	return commitHash
}

func startE2eRunner(logWriter io.Writer) error {
	runner := exec.Command("/usr/local/bin/e2e.test", "--test.timeout", "2h", "--test.v")
	runner.Stdout = logWriter
	runner.Stderr = logWriter

	return runner.Run()
}

func startColectRunner(logWriter io.Writer, mountDir string) error {
	runner := exec.Command("/usr/local/bin/collect-runner.sh", mountDir)
	runner.Stdout = logWriter
	runner.Stderr = logWriter

	return runner.Run()
}
