package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// bzlmodExists checks if MODULE.bazel exists in the given directory.
func bzlmodExists(dir string) (bool, error) {
	moduleFilePath := filepath.Join(dir, "MODULE.bazel")
	log.Printf("running os.Stat on %s", moduleFilePath)
	_, err := os.Stat(moduleFilePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("error checking for MODULE.bazel: %w", err)
}

// createEmptyModuleFile creates an empty MODULE.bazel file in the given directory.
func createEmptyModuleFile(dir string) error {
	moduleFilePath := filepath.Join(dir, "MODULE.bazel")
	log.Printf("running os.Create on %s", moduleFilePath)
	file, err := os.Create(moduleFilePath)
	if err != nil {
		return fmt.Errorf("error creating MODULE.bazel: %w", err)
	}
	file.Close()
	return nil
}

// runBazelModExplain executes 'bazel mod explain' in the given directory.
func runBazelModExplain(dir string) ([]byte, error) {
	cmd := exec.Command("bazel", "mod", "explain")
	cmd.Dir = dir // Set the working directory for the command
	log.Printf("running command: %s %s", cmd.Path, cmd.Args)
	output, err := cmd.Output()
	if err != nil {
		log.Printf("command %s failed: %v", cmd.Path, err)
		return nil, fmt.Errorf("'bazel mod explain' failed: %w", err)
	}
	log.Printf("command %s completed successfully.", cmd.Path)
	return output, nil
}

// rulesRustExists checks if the rules_rust module is present by running 'bazel mod explain'.
func rulesRustExists(dir string) (bool, error) {
	output, err := runBazelModExplain(dir)
	if err != nil {
		return false, err
	}
	return bytes.Contains(output, []byte("rules_rust")), nil
}

// runBazelQuery executes 'bazel query //...' and logs the number of targets.
func runBazelQuery(dir string) {
	queryCmd := exec.Command("bazel", "query", "//...")
	queryCmd.Dir = dir // Set the working directory for the command
	log.Printf("running command: %s %s", queryCmd.Path, queryCmd.Args)
	queryOutput, err := queryCmd.Output()
	if err != nil {
		log.Printf("command %s failed: %v\n", queryCmd.Path, err)
	} else {
		numTargets := 0
		if len(queryOutput) > 0 {
			numTargets = len(bytes.Split(queryOutput, []byte("\n")))
			if len(queryOutput) > 0 && queryOutput[len(queryOutput)-1] == '\n' {
				numTargets--
			}
		}
		log.Printf("command %s completed successfully. found %d targets.\n", queryCmd.Path, numTargets)
	}
}

// commitModuleFiles adds and commits MODULE.bazel and MODULE.bazel.lock.
func commitModuleFiles(dir string) error {
	moduleFilePath := filepath.Join(dir, "MODULE.bazel")
	moduleLockFilePath := filepath.Join(dir, "MODULE.bazel.lock")

	gitAddCmd := exec.Command("git", "add", moduleFilePath, moduleLockFilePath)
	gitAddCmd.Dir = dir // Set the working directory for the command
	log.Printf("running command: %s %s", gitAddCmd.Path, gitAddCmd.Args)
	if err := gitAddCmd.Run(); err != nil {
		log.Printf("command %s failed: %v", gitAddCmd.Path, err)
		return fmt.Errorf("error adding %s and %s to git: %w", moduleFilePath, moduleLockFilePath, err)
	}
	log.Printf("command %s completed successfully.", gitAddCmd.Path)

	gitCommitCmd := exec.Command("git", "commit", "-m", "migration: add MODULE.bazel and MODULE.bazel.lock")
	gitCommitCmd.Dir = dir // Set the working directory for the command
	log.Printf("running command: %s %s", gitCommitCmd.Path, gitCommitCmd.Args)
	if err := gitCommitCmd.Run(); err != nil {
		log.Printf("command %s failed: %v", gitCommitCmd.Path, err)
		return fmt.Errorf("error committing %s and %s: %w", moduleFilePath, moduleLockFilePath, err)
	}
	log.Printf("command %s completed successfully.", gitCommitCmd.Path)
	log.Printf("%s and %s committed successfully.\n", moduleFilePath, moduleLockFilePath)
	return nil
}

func createModuleFileIfNecessary(dir string) error {
	exists, err := bzlmodExists(dir)
	if err != nil {
		return err
	}
	if exists {
		if _, err := runBazelModExplain(dir); err != nil {
			return err
		}
		return nil
	}
	if err := createEmptyModuleFile(dir); err != nil {
		return err
	}
	if _, err := runBazelModExplain(dir); err != nil {
		return err
	}
	if err := commitModuleFiles(dir); err != nil {
		return err
	}
	return nil
}

func main() {
	defaultWd := os.Getenv("PWD")
	if defaultWd == "" {
		defaultWd = "."
	}
	wd := flag.String("wd", defaultWd, "working directory")
	flag.Parse()

	if err := createModuleFileIfNecessary(*wd); err != nil {
		log.Fatalf("error creating MODULE.bazel file if necessary: %s", err)
	}
}
