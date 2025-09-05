package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// bzlmodExists checks if MODULE.bazel exists in the given directory.
func bzlmodExists(dir string) (bool, error) {
	moduleFilePath := filepath.Join(dir, "MODULE.bazel")
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
	fmt.Printf("MODULE.bazel not found in %s. Creating an empty %s\n", dir, moduleFilePath)
	file, err := os.Create(moduleFilePath)
	if err != nil {
		return fmt.Errorf("error creating MODULE.bazel: %w", err)
	}
	file.Close()
	return nil
}

// runBazelModExplain executes 'bazel mod explain' in the given directory.
func runBazelModExplain(dir string) error {
	fmt.Println("Invoking 'bazel mod explain'...")
	cmd := exec.Command("bazel", "mod", "explain")
	cmd.Dir = dir // Set the working directory for the command
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("'bazel mod explain' failed: %w", err)
	}
	fmt.Println("'bazel mod explain' succeeded.")
	return nil
}

// runBazelQuery executes 'bazel query //...' and logs the number of targets.
func runBazelQuery(dir string) {
	fmt.Println("Invoking 'bazel query //...'...")
	queryCmd := exec.Command("bazel", "query", "//...")
	queryCmd.Dir = dir // Set the working directory for the command
	queryOutput, err := queryCmd.Output()
	if err != nil {
		fmt.Printf("'bazel query //...' failed: %v\n", err)
	} else {
		numTargets := 0
		if len(queryOutput) > 0 {
			numTargets = len(bytes.Split(queryOutput, []byte("\n")))
			if len(queryOutput) > 0 && queryOutput[len(queryOutput)-1] == '\n' {
				numTargets--
			}
		}
		fmt.Printf("'bazel query //...' succeeded. Found %d targets.\n", numTargets)
	}
}

// commitModuleFiles adds and commits MODULE.bazel and MODULE.bazel.lock.
func commitModuleFiles(dir string) error {
	moduleFilePath := filepath.Join(dir, "MODULE.bazel")
	moduleLockFilePath := filepath.Join(dir, "MODULE.bazel.lock")
	fmt.Printf("MODULE.bazel created and 'bazel mod graph' succeeded. Committing %s and %s...\n", moduleFilePath, moduleLockFilePath)

	gitAddCmd := exec.Command("git", "add", moduleFilePath, moduleLockFilePath)
	gitAddCmd.Dir = dir // Set the working directory for the command
	if err := gitAddCmd.Run(); err != nil {
		return fmt.Errorf("error adding %s and %s to git: %w", moduleFilePath, moduleLockFilePath, err)
	}
	gitCommitCmd := exec.Command("git", "commit", "-m", "feat: Add MODULE.bazel and MODULE.bazel.lock")
	gitCommitCmd.Dir = dir // Set the working directory for the command
	if err := gitCommitCmd.Run(); err != nil {
		return fmt.Errorf("error committing %s and %s: %w", moduleFilePath, moduleLockFilePath, err)
	}
	fmt.Printf("%s and %s committed successfully.\n", moduleFilePath, moduleLockFilePath)
	return nil
}

func main() {
	wd := flag.String("wd", ".", "working directory")
	flag.Parse()

	moduleFileCreated := false
	exists, err := bzlmodExists(*wd)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if !exists {
		if err := createEmptyModuleFile(*wd); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		moduleFileCreated = true
	}

	if err := runBazelModExplain(*wd); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	runBazelQuery(*wd)

	if moduleFileCreated {
		if err := commitModuleFiles(*wd); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}
