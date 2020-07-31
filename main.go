package main

import (
	"fmt"
	"os"

	"github.com/bitrise-io/go-utils/command/git"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
)

type config struct {
	RepositoryURL       string `env:"repository_url,required"`
	CloneIntoDir        string `env:"clone_into_dir,required"`
	Commit              string `env:"commit"`
	Tag                 string `env:"tag"`
	Branch              string `env:"branch"`
	ResetRepository     bool   `env:"reset_repository,opt[Yes,No]"`
	CloneDepth          int    `env:"clone_depth"`
	UpdateSubmodules    bool   `env:"update_submodules,opt[Yes,No]"`
}

func mainE() error {
	var cfg config
	if err := stepconf.Parse(&cfg); err != nil {
		failf("Error: %s\n", err)
	}
	stepconf.Print(cfg)

	gitCmd, err := git.New(cfg.CloneIntoDir)
	if err != nil {
		return fmt.Errorf("create gitCmd project, error: %v", err)
	}
	checkoutArg := getCheckoutArg(cfg.Commit, cfg.Tag, cfg.Branch)

	originPresent, err := isOriginPresent(gitCmd, cfg.CloneIntoDir, cfg.RepositoryURL)
	if err != nil {
		return fmt.Errorf("check if origin is presented, error: %v", err)
	}

	if originPresent && cfg.ResetRepository {
		if err := resetRepo(gitCmd); err != nil {
			return fmt.Errorf("reset repository, error: %v", err)
		}
	}
	if err := run(gitCmd.Init()); err != nil {
		return fmt.Errorf("init repository, error: %v", err)
	}
	if !originPresent {
		if err := run(gitCmd.RemoteAdd("origin", cfg.RepositoryURL)); err != nil {
			return fmt.Errorf("add remote repository (%s), error: %v", cfg.RepositoryURL, err)
		}
	}

	if checkoutArg != "" {
		if err := checkout(gitCmd, checkoutArg, cfg.Branch, cfg.CloneDepth, cfg.Tag != ""); err != nil {
			return fmt.Errorf("checkout (%s): %v", checkoutArg, err)
		}
		// Update branch: 'git fetch' followed by a 'git merge' is the same as 'git pull'.
		if checkoutArg == cfg.Branch {
			if err := run(gitCmd.Merge("origin/" + cfg.Branch)); err != nil {
				return fmt.Errorf("merge %q: %v", cfg.Branch, err)
			}
		}
	}

	if cfg.UpdateSubmodules {
		if err := run(gitCmd.SubmoduleUpdate()); err != nil {
			return fmt.Errorf("submodule update: %v", err)
		}
	}

	return nil
}

func failf(format string, v ...interface{}) {
	log.Errorf(format, v...)
	os.Exit(1)
}

func main() {
	if err := mainE(); err != nil {
		failf("ERROR: %v", err)
	}
	log.Donef("\nSuccess")
}
