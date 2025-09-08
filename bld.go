package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var models = []string{
	"openrouter/anthropic/claude-2",
	"openrouter/anthropic/claude-3-5-haiku",
	"openrouter/anthropic/claude-3-5-haiku-20241022",
	"openrouter/anthropic/claude-3-haiku",
	"openrouter/anthropic/claude-3-haiku-20240307",
	"openrouter/anthropic/claude-3-opus",
	"openrouter/anthropic/claude-3-sonnet",
	"openrouter/anthropic/claude-3.5-sonnet",
	"openrouter/anthropic/claude-3.5-sonnet:beta",
	"openrouter/anthropic/claude-3.7-sonnet",
	"openrouter/anthropic/claude-3.7-sonnet:beta",
	"openrouter/anthropic/claude-instant-v1",
	"openrouter/anthropic/claude-opus-4",
	"openrouter/anthropic/claude-opus-4.1",
	"openrouter/anthropic/claude-sonnet-4",
}

// sanitizePath replaces characters that are unsafe in file paths with hyphens.
func sanitizePath(s string) string {
	s = strings.ReplaceAll(s, "/", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

// getGitBranch returns the current git branch name for a given directory.
func getGitBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get git branch: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// gitBranchExists checks if a git branch exists.
func gitBranchExists(dir, branchName string) (bool, error) {
	cmd := exec.Command("git", "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	cmd.Dir = dir
	err := cmd.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			return false, nil // Branch does not exist
		}
		return false, fmt.Errorf("failed to check if branch %s exists: %w", branchName, err)
	}
	return true, nil // Branch exists
}

// createGitBranch creates a new git branch.
func createGitBranch(dir, branchName string) error {
	cmd := exec.Command("git", "branch", branchName)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}
	return nil
}

// gitWorktreeExists checks if a git worktree exists at the given path.
func gitWorktreeExists(worktreePath string) (bool, error) {
	_, err := os.Stat(worktreePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check worktree existence at %s: %w", worktreePath, err)
}

// addGitWorktree adds a new git worktree.
func addGitWorktree(repoDir, worktreePath, branchName string) error {
	cmd := exec.Command("git", "worktree", "add", worktreePath, branchName)
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add worktree at %s for branch %s: %w", worktreePath, branchName, err)
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: bld <target_directory>")
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %s", err)
	}

	targetDir := os.Args[1]

	cargoTomlPath := filepath.Join(wd, targetDir, "Cargo.toml")
	if _, err = os.Stat(cargoTomlPath); err != nil {
		log.Fatalf("Error checking for Cargo.toml: %s", err)
	}
	log.Printf("Cargo.toml found in %s", targetDir)


	branch, err := getGitBranch(wd)
	if err != nil {
		log.Printf("Error getting git branch: %v", err)
		os.Exit(1)
	}
	log.Printf("Current git branch: %s\n", branch)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting user home directory: %s", err)
	}
	worktreeBaseDir := filepath.Join(homeDir, "worktree")

	for _, model := range models {
		sanitizedModelName := sanitizePath(model)
		modelBranch := branch + "-" + sanitizedModelName
		worktreePath := filepath.Join(worktreeBaseDir, modelBranch)

		// Check and create branch
		exists, err := gitBranchExists(wd, modelBranch)
		if err != nil {
			log.Fatalf("Error checking branch %s: %s", modelBranch, err)
		}
		if !exists {
			log.Printf("Branch %s does not exist, creating...", modelBranch)
			if err := createGitBranch(wd, modelBranch); err != nil {
				log.Fatalf("Error creating branch %s: %s", modelBranch, err)
			}
			log.Printf("Branch %s created.", modelBranch)
		} else {
			log.Printf("Branch %s already exists.", modelBranch)
		}

		// Check and create worktree
		wtExists, err := gitWorktreeExists(worktreePath)
		if err != nil {
			log.Fatalf("Error checking worktree at %s: %s", worktreePath, err)
		}
		if !wtExists {
			log.Printf("Worktree at %s does not exist, creating...", worktreePath)
			if err := addGitWorktree(wd, worktreePath, modelBranch); err != nil {
				log.Fatalf("Error adding worktree at %s: %s", worktreePath, err)
			}
			log.Printf("Worktree created at: %s", worktreePath)
		} else {
			log.Printf("Worktree already exists at: %s", worktreePath)
		}
	}
}
