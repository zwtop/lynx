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
	"io"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog"
)

func main() {
	if err := rootCommand().Execute(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}

func rootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "e2e-reporter",
		Short: "run lynx e2e on tower elf environment",
		RunE:  reporter,
	}

	rootCmd.PersistentFlags().String("mount-dir", "/mnt/e2e-log", "E2e log mount dir")
	rootCmd.PersistentFlags().String("hook-url", "", "Slack hook url, must not empty")
	rootCmd.PersistentFlags().String("expose-url", "", "Expose url for look for log")
	rootCmd.PersistentFlags().String("remote-repo", "https://github.com/smartxworks/lynx.git", "Remote lynx repo")
	rootCmd.PersistentFlags().String("refspec", "main", "Checkout refspec. Could be branch, commit or pr. E.g. main, c151ee84, pull/12/head")

	rootCmd.MarkPersistentFlagRequired("hook-url")
	rootCmd.MarkPersistentFlagRequired("expose-url")

	rootCmd.Root().SilenceUsage = true
	rootCmd.Root().SetHelpCommand(&cobra.Command{Hidden: true})

	return rootCmd
}

func reporter(cmd *cobra.Command, args []string) error {
	var mountDir = cmd.Flag("mount-dir").Value.String()
	var hookUrl = cmd.Flag("hook-url").Value.String()
	var exposeUrl = cmd.Flag("expose-url").Value.String()
	var remoteRepo = cmd.Flag("remote-repo").Value.String()
	var refspec = cmd.Flag("refspec").Value.String()

	var errList []error
	startTime := time.Now()
	mountDir = path.Join(mountDir, time.Now().Format(time.RFC3339))
	logWriter := newLogWriter(mountDir)

	// init
	commitHash := mustStartInitRunner(logWriter, remoteRepo, refspec, 40*time.Minute)

	// run e2e
	errList = append(errList, startE2eRunner(logWriter))

	// fetch log
	errList = append(errList, startColectRunner(logWriter, mountDir))

	// send message
	slackMessage := message{
		startTime:  startTime,
		expostUrl:  exposeUrl,
		mountDir:   mountDir,
		err:        errors.NewAggregate(errList),
		remoteRepo: remoteRepo,
		refspec:    refspec,
		commitSha:  commitHash,
	}
	mustSendMsg(hookUrl, slackMessage, 10*time.Minute)

	return errors.NewAggregate(errList)
}

func newLogWriter(mountDir string) io.Writer {
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		klog.Fatalf("unable create dir %s, %s", mountDir, err)
	}
	runnerLog := path.Join(mountDir, "e2e-runner.log")
	f, openErr := os.OpenFile(runnerLog, os.O_CREATE|os.O_RDWR, 0644)
	if openErr != nil {
		klog.Fatalf("unable open e2e-runner log file: %s", openErr)
	}
	return io.MultiWriter(os.Stdout, f)
}
