package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	moduleFilePath := "MODULE.bazel"

	// Check if MODULE.bazel exists
	if _, err := os.Stat(moduleFilePath); os.IsNotExist(err) {
		fmt.Printf("MODULE.bazel not found. Creating an empty %s\n", moduleFilePath)
		file, err := os.Create(moduleFilePath)
		if err != nil {
			fmt.Printf("Error creating MODULE.bazel: %v\n", err)
			os.Exit(1)
		}
		file.Close()
	}

	// Invoke 'bazel mod graph'
	fmt.Println("Invoking 'bazel mod graph'...")
	cmd := exec.Command("bazel", "mod", "graph")
	// We don't care about the output, just the success/failure
	err := cmd.Run()
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
