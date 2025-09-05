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

	content := fmt.Sprintf(`
bazel_dep(name = "rules_rust", version = "%s")

crate = use_extension("@rules_rust//crate_universe:extensions.bzl", "crate")
crate.from_cargo(
    name = "crates",
    manifests = ["//:Cargo.toml"],
)
use_repo(crate, "crates")
`, rulesRustVersion)
	if _, err := f.WriteString(content); err != nil {
		return fmt.Errorf("error writing to MODULE.bazel: %w", err)
	}
	log.Printf("Added rules_rust dependency and crate_universe extension to %s", moduleFilePath)
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
	return commitModuleFiles(dir, fmt.Sprintf("migration: add rules_rust@%s to MODULE.bazel", rulesRustVersion))
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

// hasBazelBuildTargets checks if there are any bazel build targets by running 'bazel query //...'.
func hasBazelBuildTargets(dir string) (bool, error) {
	queryCmd := exec.Command("bazel", "query", "//...")
	queryCmd.Dir = dir // Set the working directory for the command
	log.Printf("running command: %s %s", queryCmd.Path, queryCmd.Args)
	queryOutput, err := queryCmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Bazel query returns non-zero exit code if no targets are found.
			// We check stderr to differentiate between no targets and other errors.
			if bytes.Contains(exitErr.Stderr, []byte("no targets found")) {
				log.Printf("command %s completed successfully. found 0 targets.\n", queryCmd.Path)
				return false, nil
			}
		}
		log.Printf("command %s failed: %v\n", queryCmd.Path, err)
		return false, fmt.Errorf("'bazel query //...' failed: %w", err)
	}
	numTargets := 0
	if len(queryOutput) > 0 {
		numTargets = len(bytes.Split(queryOutput, []byte("\n")))
		if len(queryOutput) > 0 && queryOutput[len(queryOutput)-1] == '\n' {
			numTargets--
		}
	}
	log.Printf("command %s completed successfully. found %d targets.\n", queryCmd.Path, numTargets)
	return numTargets > 0, nil
}

// getRustCrateNames returns a list of Rust crate names by running 'cargo metadata' and parsing its output.
func getRustCrateNames(dir string) ([]string, error) {
	cargoCmd := exec.Command("cargo", "metadata", "--format-version", "1", "--no-deps")
	cargoCmd.Dir = dir
	log.Printf("running command: %s %s", cargoCmd.Path, cargoCmd.Args)
	cargoOutput, err := cargoCmd.Output()
	if err != nil {
		log.Printf("command %s failed: %v\n", cargoCmd.Path, err)
		return nil, fmt.Errorf("'cargo metadata' failed: %w", err)
	}
	log.Printf("command %s completed successfully.", cargoCmd.Path)

	jqCmd := exec.Command("jq", "-r", ".packages[].name")
	jqCmd.Stdin = bytes.NewReader(cargoOutput)
	log.Printf("running command: %s %s", jqCmd.Path, jqCmd.Args)
	jqOutput, err := jqCmd.Output()
	if err != nil {
		log.Printf("command %s failed: %v\n", jqCmd.Path, err)
		return nil, fmt.Errorf("'jq' failed: %w", err)
	}
	log.Printf("command %s completed successfully.", jqCmd.Path)

	names := bytes.Split(bytes.TrimSpace(jqOutput), []byte("\n"))
	result := make([]string, 0, len(names))
	for _, name := range names {
		if len(name) > 0 {
			result = append(result, string(name))
		}
	}
	return result, nil
}

// getRustCrateDependencies returns a list of dependency names for a given crate by running 'cargo tree'.
func getRustCrateDependencies(dir string, crateName string) ([]string, error) {
	cargoCmd := exec.Command("cargo", "tree", "--package", crateName, "--prefix", "none")
	cargoCmd.Dir = dir
	log.Printf("running command: %s %s", cargoCmd.Path, cargoCmd.Args)
	cargoOutput, err := cargoCmd.Output()
	if err != nil {
		log.Printf("command %s failed: %v\n", cargoCmd.Path, err)
		return nil, fmt.Errorf("'cargo tree' failed for crate %s: %w", crateName, err)
	}
	log.Printf("command %s completed successfully.", cargoCmd.Path)

	// The output of `cargo tree --prefix none` lists each dependency on a new line.
	lines := bytes.Split(bytes.TrimSpace(cargoOutput), []byte("\n"))
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if len(line) > 0 {
			result = append(result, string(line))
		}
	}
	return result, nil
}

// getCrateWithFewestDependencies returns the name of the crate with the fewest dependencies.
func getCrateWithFewestDependencies(dir string) (string, error) {
	crateNames, err := getRustCrateNames(dir)
	if err != nil {
		return "", fmt.Errorf("error getting Rust crate names: %w", err)
	}

	if len(crateNames) == 0 {
		return "", nil // No crates found
	}

	minDependencies := -1
	crateWithFewestDependencies := ""

	for _, crateName := range crateNames {
		dependencies, err := getRustCrateDependencies(dir, crateName)
		if err != nil {
			return "", fmt.Errorf("error getting dependencies for crate %s: %w", crateName, err)
		}

		numDependencies := len(dependencies)
		if minDependencies == -1 || numDependencies < minDependencies {
			minDependencies = numDependencies
			crateWithFewestDependencies = crateName
		}
	}
	return crateWithFewestDependencies, nil
}

// getCargoTomlPath returns the path to the Cargo.toml file for a given crate name.
func getCargoTomlPath(dir string, crateName string) (string, error) {
	cargoCmd := exec.Command("cargo", "metadata", "--format-version", "1")
	cargoCmd.Dir = dir
	log.Printf("running command: %s %s", cargoCmd.Path, cargoCmd.Args)
	cargoOutput, err := cargoCmd.Output()
	if err != nil {
		log.Printf("command %s failed: %v\n", cargoCmd.Path, err)
		return "", fmt.Errorf("'cargo metadata' failed: %w", err)
	}
	log.Printf("command %s completed successfully.", cargoCmd.Path)

	jqQuery := fmt.Sprintf(".packages[] | select(.name == \"%s\") | .manifest_path", crateName)
	jqCmd := exec.Command("jq", "-r", jqQuery)
	jqCmd.Stdin = bytes.NewReader(cargoOutput)
	log.Printf("running command: %s %s", jqCmd.Path, jqCmd.Args)
	jqOutput, err := jqCmd.Output()
	if err != nil {
		log.Printf("command %s failed: %v\n", jqCmd.Path, err)
		return "", fmt.Errorf("'jq' failed: %w", err)
	}
	log.Printf("command %s completed successfully.", jqCmd.Path)

	path := string(bytes.TrimSpace(jqOutput))
	if path == "" {
		return "", fmt.Errorf("Cargo.toml path not found for crate: %s", crateName)
	}
	relativePath, err := filepath.Rel(dir, path)
	if err != nil {
		return "", fmt.Errorf("error getting relative path for %s: %w", path, err)
	}
	return relativePath, nil
}

// commitModuleFiles adds and commits MODULE.bazel and MODULE.bazel.lock.
func commitModuleFiles(dir string, message string) error {
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

	gitCommitCmd := exec.Command("git", "commit", "-m", message)
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

// buildFileExists checks if a BUILD.bazel or BUILD file exists in the given directory.
func buildFileExists(dir string) (bool, error) {
	buildBazelPath := filepath.Join(dir, "BUILD.bazel")
	buildPath := filepath.Join(dir, "BUILD")

	_, errBazel := os.Stat(buildBazelPath)
	if errBazel == nil {
		log.Printf("Found BUILD.bazel at %s", buildBazelPath)
		return true, nil
	}
	if !os.IsNotExist(errBazel) {
		return false, fmt.Errorf("error checking for BUILD.bazel: %w", errBazel)
	}

	_, errBuild := os.Stat(buildPath)
	if errBuild == nil {
		log.Printf("Found BUILD at %s", buildPath)
		return true, nil
	}
	if !os.IsNotExist(errBuild) {
		return false, fmt.Errorf("error checking for BUILD: %w", errBuild)
	}

	log.Printf("No BUILD.bazel or BUILD file found in %s", dir)
	return false, nil
}

// createEmptyBuildFile creates an empty BUILD.bazel file in the given directory.
func createEmptyBuildFile(dir string) error {
	buildFilePath := filepath.Join(dir, "BUILD.bazel")
	log.Printf("running os.Create on %s", buildFilePath)
	file, err := os.Create(buildFilePath)
	if err != nil {
		return fmt.Errorf("error creating BUILD.bazel: %w", err)
	}
	file.Close()
	return nil
}

// commitBuildFile adds and commits BUILD.bazel.
func commitBuildFile(dir string, message string) error {
	buildFilePath := filepath.Join(dir, "BUILD.bazel")

	gitAddCmd := exec.Command("git", "add", buildFilePath)
	gitAddCmd.Dir = dir // Set the working directory for the command
	log.Printf("running command: %s %s", gitAddCmd.Path, gitAddCmd.Args)
	if err := gitAddCmd.Run(); err != nil {
		log.Printf("command %s failed: %v", gitAddCmd.Path, err)
		return fmt.Errorf("error adding %s to git: %w", buildFilePath, err)
	}
	log.Printf("command %s completed successfully.", gitAddCmd.Path)

	gitCommitCmd := exec.Command("git", "commit", "-m", message)
	gitCommitCmd.Dir = dir // Set the working directory for the command
	log.Printf("running command: %s %s", gitCommitCmd.Path, gitCommitCmd.Args)
	if err := gitCommitCmd.Run(); err != nil {
		log.Printf("command %s failed: %v", gitCommitCmd.Path, err)
		return fmt.Errorf("error committing %s: %w", buildFilePath, err)
	}
	log.Printf("command %s completed successfully.", gitCommitCmd.Path)
	log.Printf("%s committed successfully.\n", buildFilePath)
	return nil
}

func createBuildFileIfNecessary(dir string) error {
	exists, err := buildFileExists(dir)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if err := createEmptyBuildFile(dir); err != nil {
		return err
	}
	return commitBuildFile(dir, "migration: add BUILD.bazel")
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
	return commitModuleFiles(dir, "migration: add MODULE.bazel and MODULE.bazel.lock")
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
	if err := createBuildFileIfNecessary(*wd); err != nil {
		log.Fatalf("BUILD.bazel does not exist or could not be created: %s", err)
	}
	if err := addRulesRustDependencyIfNecessary(*wd); err != nil {
		log.Fatalf("rules_rust module not present or could not be added: %s", err)
	}

	crate, err := getCrateWithFewestDependencies(*wd)
	if err != nil {
		log.Fatalf("error getting crate with fewest dependencies: %s", err)
	}
	if crate == "" {
		fmt.Println("No Rust crates found in the project.")
		return
	}

	fmt.Printf("Crate with fewest dependencies: %s\n", crate)
	cargoTomlPath, err := getCargoTomlPath(*wd, crate)
	if err != nil {
		log.Fatalf("error getting Cargo.toml path for crate %s: %s", crate, err)
	}
	fmt.Printf("Relative path to Cargo.toml: %s\n", cargoTomlPath)
}
