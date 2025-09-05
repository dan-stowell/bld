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
		os.Exit(1)
	}

	if moduleFileCreated {
		fmt.Printf("MODULE.bazel created and 'bazel mod graph' succeeded. Committing %s...\n", moduleFilePath)
		gitAddCmd := exec.Command("git", "add", moduleFilePath)
		gitAddCmd.Dir = *wd // Set the working directory for the command
		if err := gitAddCmd.Run(); err != nil {
			fmt.Printf("Error adding %s to git: %v\n", moduleFilePath, err)
			os.Exit(1)
		}
		gitCommitCmd := exec.Command("git", "commit", "-m", "feat: Add MODULE.bazel")
		gitCommitCmd.Dir = *wd // Set the working directory for the command
		if err := gitCommitCmd.Run(); err != nil {
			fmt.Printf("Error committing %s: %v\n", moduleFilePath, err)
			os.Exit(1)
		}
		fmt.Printf("%s committed successfully.\n", moduleFilePath)
	}

	os.Exit(0)
}
