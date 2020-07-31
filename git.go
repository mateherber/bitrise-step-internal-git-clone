package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/command/git"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-utils/retry"
)

func isOriginPresent(gitCmd git.Git, dir, repoURL string) (bool, error) {
	absDir, err := pathutil.AbsPath(dir)
	if err != nil {
		return false, err
	}

	gitDir := filepath.Join(absDir, ".git")
	if exist, err := pathutil.IsDirExists(gitDir); err != nil {
		return false, err
	} else if exist {
		remotes, err := output(gitCmd.RemoteList())
		if err != nil {
			return false, err
		}

		if !strings.Contains(remotes, repoURL) {
			return false, fmt.Errorf(".git folder exists in the directory (%s), but using a different remote", dir)
		}
		return true, nil
	}

	return false, nil
}

func resetRepo(gitCmd git.Git) error {
	if err := run(gitCmd.Reset("--hard", "HEAD")); err != nil {
		return err
	}
	if err := run(gitCmd.Clean("-x", "-d", "-f")); err != nil {
		return err
	}
	if err := run(gitCmd.SubmoduleForeach(gitCmd.Reset("--hard", "HEAD"))); err != nil {
		return err
	}
	return run(gitCmd.SubmoduleForeach(gitCmd.Clean("-x", "-d", "-f")))
}

func getCheckoutArg(commit, tag, branch string) string {
	switch {
	case commit != "":
		return commit
	case tag != "":
		return tag
	case branch != "":
		return branch
	default:
		return ""
	}
}

func run(c *command.Model) error {
	log.Infof(c.PrintableCommandArgs())
	return c.SetStdout(os.Stdout).SetStderr(os.Stderr).Run()
}

func output(c *command.Model) (string, error) {
	return c.RunAndReturnTrimmedCombinedOutput()
}

func runWithRetry(f func() *command.Model) error {
	return retry.Times(2).Wait(5).Try(func(attempt uint) error {
		if attempt > 0 {
			log.Warnf("Retrying...")
		}

		err := run(f())
		if err != nil {
			log.Warnf("Attempt %d failed:", attempt+1)
			fmt.Println(err.Error())
		}

		return err
	})
}

func checkout(gitCmd git.Git, arg, branch string, depth int, isTag bool) error {
	if err := runWithRetry(func() *command.Model {
		var opts []string
		if depth != 0 {
			opts = append(opts, "--depth="+strconv.Itoa(depth))
		}
		if isTag {
			opts = append(opts, "--tags")
		}
		if branch == arg {
			opts = append(opts, "origin", branch)
		}
		return gitCmd.Fetch(opts...)
	}); err != nil {
		return fmt.Errorf("Fetch failed, error: %v", err)
	}

	if err := run(gitCmd.Checkout(arg)); err != nil {
		if depth == 0 {
			return fmt.Errorf("checkout failed (%s), error: %v", arg, err)
		}
		log.Warnf("Checkout failed, error: %v\nUnshallow...", err)
		if err := runWithRetry(func() *command.Model {
			return gitCmd.Fetch("--unshallow")
		}); err != nil {
			return fmt.Errorf("fetch failed, error: %v", err)
		}
		if err := run(gitCmd.Checkout(arg)); err != nil {
			return fmt.Errorf("checkout failed (%s), error: %v", arg, err)
		}
	}

	return nil
}
