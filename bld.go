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
	// openrouter top 10 programming weekly as of 2025-09-08
	"x-ai/grok-code-fast-1",
	"anthropic/claude-sonnet-4",
	"google/gemini-2.5-flash",
	"openai/gpt-4.1-mini",
	"google/gemini-2.5-pro",
	"openai/gpt-5",
	"qwen/qwen3-coder",
	"openrouter/sonoma-sky-alpha",
	"deepseek/deepseek-chat-v3.1",
	"x-ai/grok-4",
}

var targets = []string{
	"//crates/matcher:grep_matcher",
	"//crates/matcher:integration_test",
	"//crates/globset:globset",
	"//crates/cli:grep_cli",
	"//crates/regex:grep_regex",
	"//crates/searcher:grep_searcher",
	"//crates/pcre2:grep_pcre2",
	"//crates/ignore:ignore",
	"//crates/printer:grep_printer",
	"//crates/grep:grep",
	"//:ripgrep",
	"//:integration_test",
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

// createGitBranchIfNotExists ensures the given branch exists in the repo at dir.
// If the branch does not exist it will be created. The function logs progress
// similarly to the previous inline behavior.
func createGitBranchIfNotExists(dir, branchName string) error {
	exists, err := gitBranchExists(dir, branchName)
	if err != nil {
		return fmt.Errorf("failed to check if branch %s exists: %w", branchName, err)
	}
	if exists {
		log.Printf("Branch %s already exists.", branchName)
		return nil
	}

	log.Printf("Branch %s does not exist, creating...", branchName)
	if err := createGitBranch(dir, branchName); err != nil {
		return fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}
	log.Printf("Branch %s created.", branchName)
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

// createGitWorktreeIfNotExists ensures the given worktree exists at worktreePath.
// If the worktree does not exist it will be created. The function logs progress
// similarly to the previous inline behavior.
func createGitWorktreeIfNotExists(repoDir, worktreePath, branchName string) error {
	exists, err := gitWorktreeExists(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to check if worktree %s exists: %w", worktreePath, err)
	}
	if exists {
		log.Printf("Worktree already exists at: %s", worktreePath)
		return nil
	}

	log.Printf("Worktree at %s does not exist, creating...", worktreePath)
	if err := addGitWorktree(repoDir, worktreePath, branchName); err != nil {
		return fmt.Errorf("failed to add worktree at %s for branch %s: %w", worktreePath, branchName, err)
	}
	log.Printf("Worktree created at: %s", worktreePath)
	return nil
}

func runLLM(model, targetDir string, stdin string) (string, error) {
	prompt := fmt.Sprintf(
		"Please write the minimal BUILD.bazel file with a single target for the crate under %s. Output just the BUILD.bazel contents. Including MODULE.bazel and the Cargo.toml for the crate.",
		targetDir,
	)
	cmd := exec.Command("llm", "-x", "-m", model, "-s", prompt)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("llm failed: %w\n%s", err, string(ee.Stderr))
		}
		return "", fmt.Errorf("llm failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runFilesToPrompt(worktreePath, targetDir string) (string, error) {
	cmd := exec.Command("files-to-prompt", "MODULE.bazel", filepath.Join(targetDir, "Cargo.toml"))
	cmd.Dir = worktreePath
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("files-to-prompt failed: %w\n%s", err, string(ee.Stderr))
		}
		return "", fmt.Errorf("files-to-prompt failed: %w", err)
	}
	return string(out), nil
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
		sanitizedModelName := sanitizePath("openrouter/" + model)
		modelBranch := branch + "-" + sanitizedModelName
		worktreePath := filepath.Join(worktreeBaseDir, modelBranch)

		// Ensure branch exists (create if needed)
		if err := createGitBranchIfNotExists(wd, modelBranch); err != nil {
			log.Fatalf("Error ensuring branch %s exists: %s", modelBranch, err)
		}

		// Ensure worktree exists (create if needed)
		if err := createGitWorktreeIfNotExists(wd, worktreePath, modelBranch); err != nil {
			log.Fatalf("Error ensuring worktree at %s exists: %s", worktreePath, err)
		}

		// Bazel query removed: no longer verifying //... in the worktree.

		// Invoke files-to-prompt and then LLM to generate BUILD.bazel contents for this model/worktree
		llmModel := "openrouter/" + model
		ftpOut, err := runFilesToPrompt(worktreePath, targetDir)
		if err != nil {
			log.Fatalf("files-to-prompt failed for %s: %s", worktreePath, err)
		}
		llmOut, err := runLLM(llmModel, targetDir, ftpOut)
		if err != nil {
			log.Fatalf("LLM invocation failed for model %s: %s", llmModel, err)
		}
		// Write LLM output to BUILD.bazel in the target directory of this worktree
		buildFilePath := filepath.Join(worktreePath, targetDir, "BUILD.bazel")
		if err := os.WriteFile(buildFilePath, []byte(llmOut+"\n"), 0644); err != nil {
			log.Fatalf("Failed writing BUILD.bazel for model %s to %s: %v", llmModel, buildFilePath, err)
		}
		log.Printf("Wrote BUILD.bazel for %s to %s", llmModel, buildFilePath)
	}
}
