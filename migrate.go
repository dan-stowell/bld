package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	wd := flag.String("wd", ".", "working directory")
	flag.Parse()

	moduleFilePath := filepath.Join(*wd, "MODULE.bazel")
	moduleFileCreated := false

	// Check if MODULE.bazel exists
	if _, err := os.Stat(moduleFilePath); os.IsNotExist(err) {
		fmt.Printf("MODULE.bazel not found in %s. Creating an empty %s\n", *wd, moduleFilePath)
		file, err := os.Create(moduleFilePath)
		if err != nil {
			fmt.Printf("Error creating MODULE.bazel: %v\n", err)
			os.Exit(1)
		}
		file.Close()
		moduleFileCreated = true
	}

	// Invoke 'bazel mod graph'
	fmt.Println("Invoking 'bazel mod graph'...")
	cmd := exec.Command("bazel", "mod", "graph")
	cmd.Dir = *wd // Set the working directory for the command
	// We don't care about the output, just the success/failure
	err := cmd.Run()
	if err != nil {
		fmt.Printf("'bazel mod graph' failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("'bazel mod graph' succeeded.")

	// Invoke 'bazel query //...'
	fmt.Println("Invoking 'bazel query //...'...")
	queryCmd := exec.Command("bazel", "query", "//...")
	queryCmd.Dir = *wd // Set the working directory for the command
	queryOutput, err := queryCmd.Output()
	if err != nil {
		fmt.Printf("'bazel query //...' failed: %v\n", err)
	} else {
		// Count the number of lines (targets)
		numTargets := 0
		if len(queryOutput) > 0 {
			numTargets = len(bytes.Split(queryOutput, []byte("\n")))
			// Adjust for potential trailing newline creating an empty last element
			if len(queryOutput) > 0 && queryOutput[len(queryOutput)-1] == '\n' {
				numTargets--
			}
		}
		fmt.Printf("'bazel query //...' succeeded. Found %d targets.\n", numTargets)
	}

	if moduleFileCreated {
		moduleLockFilePath := filepath.Join(*wd, "MODULE.bazel.lock")
		fmt.Printf("MODULE.bazel created and 'bazel mod graph' succeeded. Committing %s and %s...\n", moduleFilePath, moduleLockFilePath)

		gitAddCmd := exec.Command("git", "add", moduleFilePath, moduleLockFilePath)
		gitAddCmd.Dir = *wd // Set the working directory for the command
		if err := gitAddCmd.Run(); err != nil {
			fmt.Printf("Error adding %s and %s to git: %v\n", moduleFilePath, moduleLockFilePath, err)
			os.Exit(1)
		}
		gitCommitCmd := exec.Command("git", "commit", "-m", "feat: Add MODULE.bazel and MODULE.bazel.lock")
		gitCommitCmd.Dir = *wd // Set the working directory for the command
		if err := gitCommitCmd.Run(); err != nil {
			fmt.Printf("Error committing %s and %s: %v\n", moduleFilePath, moduleLockFilePath, err)
			os.Exit(1)
		}
		fmt.Printf("%s and %s committed successfully.\n", moduleFilePath, moduleLockFilePath)
	}

	os.Exit(0)
}
