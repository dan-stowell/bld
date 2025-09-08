package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var models = []string{
	// openrouter top 10 programming weekly as of 2025-09-08
	"anthropic/claude-sonnet-4",
	"google/gemini-2.5-flash",
	"openai/gpt-4.1-mini",
	"google/gemini-2.5-pro",
	"openai/gpt-5",
	"qwen/qwen3-coder",
	"openrouter/sonoma-sky-alpha",
	"deepseek/deepseek-chat-v3.1",
	"x-ai/grok-code-fast-1",
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

func humanLogInvoke(model, target string, attempt int, cmd *exec.Cmd) {
	bin := cmd.Args[0]
	fmt.Fprintf(os.Stderr, "%s model=%s target=%s attempt=%d invoked=%s\n", time.Now().Format(time.RFC3339Nano), model, target, attempt, filepath.Base(bin))
}

func humanLogComplete(model, target string, attempt int, cmd *exec.Cmd, err error) {
	bin := cmd.Args[0]
	status := "ok"
	if err != nil {
		status = "err"
	}
	fmt.Fprintf(os.Stderr, "%s model=%s target=%s attempt=%d completed=%s status=%s\n", time.Now().Format(time.RFC3339Nano), model, target, attempt, filepath.Base(bin), status)
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

func ensureBuildBazelExists(worktreePath, target string) error {
	// Parse target like //path/to/pkg:target or //:target
	if !strings.HasPrefix(target, "//") {
		// not a package-style target; nothing to do
		return nil
	}
	s := strings.TrimPrefix(target, "//")
	pkg := s
	if idx := strings.Index(s, ":"); idx != -1 {
		pkg = s[:idx]
	}
	var pkgPath string
	if pkg == "" {
		pkgPath = ""
	} else {
		pkgPath = pkg
	}
	buildPath := filepath.Join(worktreePath, pkgPath, "BUILD.bazel")
	if _, err := os.Stat(buildPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat %s: %w", buildPath, err)
	}
	dir := filepath.Dir(buildPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create dir %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(buildPath, []byte("# created by bld.go\n"), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", buildPath, err)
	}
	log.Printf("Created %s", buildPath)
	return nil
}

func gitStashAll(worktreePath, model, target string, attempt int) error {
	// Stash untracked and dirty files so the next aider invocation starts clean.
	stashMsg := fmt.Sprintf("aider-temp-stash target %s", target)
	stashCmd := exec.Command("git", "stash", "push", "-u", "-m", stashMsg)
	stashCmd.Dir = worktreePath
	humanLogInvoke(model, target, attempt, stashCmd)
	out, err := stashCmd.CombinedOutput()
	humanLogComplete(model, target, attempt, stashCmd, err)
	if err != nil {
		return fmt.Errorf("git stash failed in %s: %v\n%s", worktreePath, err, string(out))
	}
	// git stash prints a message even when there is nothing to stash;
	// log the output for debugging but don't treat it as fatal.
	log.Printf("git stash output in %s: %s", worktreePath, strings.TrimSpace(string(out)))
	return nil
}

func makeTargetBuild(worktreePath, llmModel, target string) (bool, error) {
	// Ensure BUILD.bazel exists for the target package.
	if err := ensureBuildBazelExists(worktreePath, target); err != nil {
		return false, fmt.Errorf("ensuring BUILD.bazel for target %s: %w", target, err)
	}

	// Determine the BUILD.bazel path for the target to pass to aider.
	pkg := strings.TrimPrefix(target, "//")
	if idx := strings.Index(pkg, ":"); idx != -1 {
		pkg = pkg[:idx]
	}
	var buildArg string
	if pkg == "" {
		buildArg = "BUILD.bazel"
	} else {
		buildArg = filepath.Join(pkg, "BUILD.bazel")
	}

	// Pre-check: If bazel query then bazel build succeed without changes, return success.
	queryCmd := exec.Command("bazel", "query", target)
	queryCmd.Dir = worktreePath
	humanLogInvoke(llmModel, target, 0, queryCmd)
	queryOut, queryErr := queryCmd.CombinedOutput()
	humanLogComplete(llmModel, target, 0, queryCmd, queryErr)
	if queryErr == nil {
		// Query succeeded; try building directly.
		bazelCmd := exec.Command("bazel", "build", target)
		bazelCmd.Dir = worktreePath
		humanLogInvoke(llmModel, target, 0, bazelCmd)
		bazelOut, bazelErr := bazelCmd.CombinedOutput()
		humanLogComplete(llmModel, target, 0, bazelCmd, bazelErr)
		if bazelErr == nil {
			log.Printf("bazel query and build succeeded for model %s target %s; skipping aider", llmModel, target)
			return true, nil
		}
		log.Printf("Pre-check bazel build failed for model %s target %s: %v\n%s", llmModel, target, bazelErr, string(bazelOut))
	} else {
		log.Printf("Pre-check bazel query failed for model %s target %s: %v\n%s", llmModel, target, queryErr, string(queryOut))
	}

	// Try up to N attempts per model/target using aider to produce Bazel changes.
	const maxAttempts = 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		aiderCmd := exec.Command(
			"aider",
			"--no-auto-commits",
			"--disable-playwright",
			"--yes-always",
			"--model", llmModel,
			"--edit-format", "diff",
			"--auto-test",
			"--test-cmd", "bazel build "+target,
			"--message", "Please make the minimal Bazel file changes necessary to build "+target+". Do not touch non-Bazel files.",
			"MODULE.bazel",
			buildArg,
		)
		aiderCmd.Dir = worktreePath
		aiderCmd.Stdout = io.Discard
		aiderCmd.Stderr = io.Discard
		humanLogInvoke(llmModel, target, attempt, aiderCmd)
		if err := aiderCmd.Run(); err != nil {
			humanLogComplete(llmModel, target, attempt, aiderCmd, err)
			return false, fmt.Errorf("aider failed for model %s target %s: %w", llmModel, target, err)
		}
		humanLogComplete(llmModel, target, attempt, aiderCmd, nil)
		log.Printf("aider completed for model %s target %s (attempt %d/%d)", llmModel, target, attempt, maxAttempts)

		// After aider, first run 'bazel query' to check target visibility/resolution.
		queryCmd := exec.Command("bazel", "query", target)
		queryCmd.Dir = worktreePath
		humanLogInvoke(llmModel, target, attempt, queryCmd)
		queryOut, queryErr := queryCmd.CombinedOutput()
		humanLogComplete(llmModel, target, attempt, queryCmd, queryErr)
		if queryErr != nil {
			log.Printf("bazel query failed for model %s target %s: %v\n%s", llmModel, target, queryErr, string(queryOut))
			// Stash any untracked or dirty files and retry with aider.
			if err := gitStashAll(worktreePath, llmModel, target, attempt); err != nil {
				return false, fmt.Errorf("git stash failed in %s: %w", worktreePath, err)
			}
			log.Printf("Re-invoking aider for model %s target %s after failed bazel query (attempt %d/%d)", llmModel, target, attempt, maxAttempts)
			continue
		}

		// Query succeeded; attempt to build the target.
		bazelCmd := exec.Command("bazel", "build", target)
		bazelCmd.Dir = worktreePath
		humanLogInvoke(llmModel, target, attempt, bazelCmd)
		bazelOut, bazelErr := bazelCmd.CombinedOutput()
		humanLogComplete(llmModel, target, attempt, bazelCmd, bazelErr)
		if bazelErr != nil {
			log.Printf("bazel build failed for model %s target %s: %v\n%s", llmModel, target, bazelErr, string(bazelOut))
			// Stash any untracked or dirty files and retry with aider.
			if err := gitStashAll(worktreePath, llmModel, target, attempt); err != nil {
				return false, fmt.Errorf("git stash failed in %s: %w", worktreePath, err)
			}
			log.Printf("Re-invoking aider for model %s target %s after failed bazel build (attempt %d/%d)", llmModel, target, attempt, maxAttempts)
			continue
		}

		// Bazel build succeeded. Commit any untracked or dirty files and return success.
		addCmd := exec.Command("git", "add", "-A")
		addCmd.Dir = worktreePath
		humanLogInvoke(llmModel, target, attempt, addCmd)
		out, err := addCmd.CombinedOutput()
		humanLogComplete(llmModel, target, attempt, addCmd, err)
		if err != nil {
			return false, fmt.Errorf("git add failed in %s: %v\n%s", worktreePath, err, string(out))
		}

		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = worktreePath
		humanLogInvoke(llmModel, target, attempt, statusCmd)
		statusOut, err := statusCmd.Output()
		humanLogComplete(llmModel, target, attempt, statusCmd, err)
		if err != nil {
			return false, fmt.Errorf("git status failed in %s: %w", worktreePath, err)
		}
		if strings.TrimSpace(string(statusOut)) == "" {
			log.Printf("No changes to commit in %s for model %s target %s", worktreePath, llmModel, target)
		} else {
			commitMsg := fmt.Sprintf("aider: model %s target %s", llmModel, target)
			commitCmd := exec.Command("git", "commit", "-m", commitMsg)
			commitCmd.Dir = worktreePath
			commitCmd.Stdout = io.Discard
			commitCmd.Stderr = io.Discard
			humanLogInvoke(llmModel, target, attempt, commitCmd)
			if err := commitCmd.Run(); err != nil {
				humanLogComplete(llmModel, target, attempt, commitCmd, err)
				return false, fmt.Errorf("git commit failed in %s: %w", worktreePath, err)
			}
			humanLogComplete(llmModel, target, attempt, commitCmd, nil)
			log.Printf("Committed changes in %s: %s", worktreePath, commitMsg)
		}

		log.Printf("bazel build succeeded for model %s target %s", llmModel, target)
		return true, nil
	}

	return false, fmt.Errorf("maximum attempts (%d) reached for model %s target %s", maxAttempts, llmModel, target)
}

func main() {
	logPath := flag.String("log", "bld.log", "path to detailed log file")
	flag.Parse()

	f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		defer f.Close()
	} else {
		log.Printf("Warning: could not open log file %s: %v; logging to stderr", *logPath, err)
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting working directory: %s", err)
	}

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

		// For each target, invoke aider in the worktree so the model can make
		// minimal Bazel changes to build the target.
		llmModel := "openrouter/" + model
		for _, target := range targets {
			success, err := makeTargetBuild(worktreePath, llmModel, target)
			if err != nil || !success {
				if err != nil {
					log.Printf("makeTargetBuild failed for model %s target %s: %v; moving to next model/worktree", llmModel, target, err)
				} else {
					log.Printf("makeTargetBuild did not succeed for model %s target %s; moving to next model/worktree", llmModel, target)
				}
				break
			}
		}
	}
}
