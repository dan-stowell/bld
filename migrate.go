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

const rulesRustVersion = "0.64.0"

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

// addRulesRustDependency adds the bazel_dep for rules_rust to MODULE.bazel.
func addRulesRustDependency(dir string) error {
	moduleFilePath := filepath.Join(dir, "MODULE.bazel")
	f, err := os.OpenFile(moduleFilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening MODULE.bazel: %w", err)
	}
	defer f.Close()

	dependency := fmt.Sprintf("\nbazel_dep(name = \"rules_rust\", version = \"%s\")\n", rulesRustVersion)
	if _, err := f.WriteString(dependency); err != nil {
		return fmt.Errorf("error writing to MODULE.bazel: %w", err)
	}
	log.Printf("Added rules_rust dependency to %s", moduleFilePath)
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

func addRulesRustDependencyIfNecessary(dir string) error {
	exists, err := rulesRustExists(dir)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := addRulesRustDependency(dir); err != nil {
		return err
	}
	added, err := rulesRustExists(dir)
	if err != nil {
		return err
	}
	if !added {
		return fmt.Errorf("adding rules_rust did not succeed")
	}
	return commitModuleFiles(dir)
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
	return commitModuleFiles(dir)
}

func main() {
	defaultWd := os.Getenv("PWD")
	if defaultWd == "" {
		defaultWd = "."
	}
	wd := flag.String("wd", defaultWd, "working directory")
	flag.Parse()

	if err := createModuleFileIfNecessary(*wd); err != nil {
		log.Fatalf("MODULE.bazel does not exist or could not be created: %s", err)
	}
	if err := addRulesRustDependencyIfNecessary(*wd); err != nil {
		log.Fatalf("rules_rust module not present or could not be added: %s", err)
	}
}
