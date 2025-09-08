package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var systemPackagesBoilerplate = `
{ config, pkgs, ... }:
{
  environment.systemPackages = [

  ];
}

`

var homeManagerBoilerplate = `

{ config, pkgs, ... }:

{
  home.packages = [ 
    
  ];
  
}
`

var flatpakPackagesBoilerplate = `
{ config, pkgs, ... }:

{
  services.flatpak.packages = [

  ];
}



`

// Check if a package configuration already exists in any .nix file
func packageConfigExists(flakeDir, configType string) bool {
	err := filepath.WalkDir(flakeDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".nix") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil // Skip files we can't read
		}

		if strings.Contains(string(content), configType) {
			return fmt.Errorf("found")
		}

		return nil
	})

	return err != nil && err.Error() == "found"
}

// Create package configuration file if it doesn't exist
func createPackageFile(flakeDir, filename, configType, boilerplate, modulePath string) {
	// Check if package config already exists
	if packageConfigExists(flakeDir, configType) {
		fmt.Printf("%s already exists in configuration, skipping creation\n", configType)
		return
	}

	// Ask for confirmation
	fmt.Printf("About to create file '%s' and add module '%s'\n", filename, modulePath)
	fmt.Print("Proceed? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Operation cancelled.")
		return
	}

	// Create packages file
	file, err := os.Create(filepath.Join(flakeDir, "packages", filename))
	if err != nil {
		log.Printf("Error creating %s: %v", filename, err)
		return
	}
	defer file.Close()

	// Write boilerplate content
	_, err = file.WriteString(boilerplate)
	if err != nil {
		log.Printf("Error writing to %s: %v", filename, err)
		return
	}

	// Add module to flake
	err = addModule(filepath.Join(flakeDir, "flake.nix"), modulePath)
	if err != nil {
		log.Printf("Error adding module to flake: %v", err)
	}
}

func setupHomeManagerPackages() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Error getting home directory: %v", err)
		return
	}

	flakeLocationPath := filepath.Join(homedir, ".config", "apm", "flakelocation.txt")

	flakeDir, err := readFlakeLocation(flakeLocationPath)
	if err != nil {
		log.Printf("Error reading flake location: %v", err)
		return
	}

	// Add home-manager input to flake
	err = addInput(filepath.Join(flakeDir, "flake.nix"), "home-manager", "")
	if err != nil {
		log.Printf("Error adding home-manager input to flake: %v", err)
		return
	}

	// Add home-manager module to flake
	addModule(filepath.Join(flakeDir, "flake.nix"), "inputs.home-manager.nixosModules.home-manager")
}

func makeNixEnv() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Error getting home directory: %v", err)
		return
	}

	flakeLocationPath := filepath.Join(homedir, ".config", "apm", "flakelocation.txt")

	flakeDir, err := readFlakeLocation(flakeLocationPath)
	if err != nil {
		log.Printf("Error reading flake location: %v", err)
		return
	}

	err = os.MkdirAll(filepath.Join(flakeDir, "packages"), 0o755)
	if err != nil {
		log.Printf("Error creating directory %s: %v", filepath.Join(flakeDir, "packages"), err)
		return
	}

	// Check if flake.nix exists
	_, err = os.ReadFile(filepath.Join(flakeDir, "flake.nix"))
	if err != nil {
		log.Printf("Error reading flake.nix: %v (is your system flaked?)", err)
		return
	}

	// Create system packages file
	createPackageFile(flakeDir, "environment-packages.nix", "environment.systemPackages", systemPackagesBoilerplate, "./packages/environment-packages.nix")
}

func addModule(flakePath, modulePath string) error {
	// Read flake.nix
	content, err := os.ReadFile(flakePath)
	if err != nil {
		return fmt.Errorf("error reading flake.nix: %v", err)
	}

	// Check if module already exists
	if strings.Contains(string(content), modulePath) {
		fmt.Printf("Module '%s' already exists in flake\n", modulePath)
		return nil
	}

	// Ask for confirmation
	fmt.Printf("About to add module '%s' to flake\n", modulePath)
	fmt.Print("Proceed? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// Find modules array
	contentStr := string(content)
	modulesIndex := strings.Index(contentStr, "modules = [")
	if modulesIndex == -1 {
		return fmt.Errorf("modules array not found in flake.nix")
	}

	// Find closing bracket
	closeIndex := strings.Index(contentStr[modulesIndex:], "]")
	if closeIndex == -1 {
		return fmt.Errorf("closing bracket not found for modules array")
	}
	closeIndex += modulesIndex

	// Insert module before closing bracket
	newContent := contentStr[:closeIndex] + "    " + modulePath + "\n" + contentStr[closeIndex:]

	// Write back
	err = os.WriteFile(flakePath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing flake.nix: %v", err)
	}

	fmt.Printf("Added module '%s' to flake\n", modulePath)
	return nil
}

// Create home manager packages file
func makeHomeEnv() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Error getting home directory: %v", err)
		return
	}

	flakeLocationPath := filepath.Join(homedir, ".config", "apm", "flakelocation.txt")

	flakeDir, err := readFlakeLocation(flakeLocationPath)
	if err != nil {
		log.Printf("Error reading flake location: %v", err)
		return
	}

	err = os.MkdirAll(filepath.Join(flakeDir, "packages"), 0o755)
	if err != nil {
		log.Printf("Error creating directory %s: %v", filepath.Join(flakeDir, "packages"), err)
		return
	}

	// Setup home-manager input and module
	setupHomeManagerPackages()

	// Create home manager packages file
	createPackageFile(flakeDir, "home-packages.nix", "home.packages", homeManagerBoilerplate, "./packages/home-packages.nix")
}

// Setup Flatpak module
func setupFlatpak() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Printf("Error getting home directory: %v", err)
		return
	}

	flakeLocationPath := filepath.Join(homedir, ".config", "apm", "flakelocation.txt")

	flakeDir, err := readFlakeLocation(flakeLocationPath)
	if err != nil {
		log.Printf("Error reading flake location: %v", err)
		return
	}

	// Add Flatpak module to flake
	err = addModule(filepath.Join(flakeDir, "flake.nix"), "flatpaks.nixosModules.nix-flatpak")
	if err != nil {
		log.Printf("Error adding Flatpak module to flake: %v", err)
	}
}

// Extract nixpkgs version from flake
func getNixpkgsVersion(flakePath string) (string, error) {
	// Read flake.nix
	content, err := os.ReadFile(flakePath)
	if err != nil {
		return "", fmt.Errorf("error reading flake.nix: %v", err)
	}

	contentStr := string(content)

	// Find nixpkgs.url line
	lines := strings.Split(contentStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "nixpkgs.url =") && !strings.HasPrefix(line, "#") {
			// Extract version from URL
			if strings.Contains(line, "nixos-") {
				parts := strings.Split(line, "nixos-")
				if len(parts) == 2 {
					version := strings.Split(parts[1], "\"")[0]
					return version, nil
				}
			}
		}
	}

	return "", fmt.Errorf("nixpkgs version not found in flake")
}

func addInput(flakePath, inputName, inputURL string) error {
	// Read flake.nix
	content, err := os.ReadFile(flakePath)
	if err != nil {
		return fmt.Errorf("error reading flake.nix: %v", err)
	}

	contentStr := string(content)

	// Check if input already exists
	if strings.Contains(contentStr, inputName+".url") {
		fmt.Printf("Input '%s' already exists in flake\n", inputName)
		return nil
	}

	// Handle special cases
	var finalURL string
	var additionalLines []string

	switch inputName {
	case "home-manager":
		// Get nixpkgs version and create matching home-manager URL
		nixpkgsVersion, err := getNixpkgsVersion(flakePath)
		if err != nil {
			// Fallback to a default version
			finalURL = "github:nix-community/home-manager/release-24.11"
			fmt.Printf("Could not determine nixpkgs version, using default home-manager version\n")
		} else {
			finalURL = fmt.Sprintf("github:nix-community/home-manager/release-%s", nixpkgsVersion)
		}
		// Add follows relationship
		additionalLines = append(additionalLines, fmt.Sprintf("    %s.inputs.nixpkgs.follows = \"nixpkgs\";", inputName))

	case "flatpaks", "flatpak":
		finalURL = "github:gmodena/nix-flatpak/?ref=latest"

	default:
		finalURL = inputURL
	}

	// Show what will be added
	fmt.Printf("About to add input '%s' with URL '%s'\n", inputName, finalURL)
	for _, line := range additionalLines {
		fmt.Printf("Will also add: %s\n", strings.TrimSpace(line))
	}

	// Ask for confirmation
	fmt.Print("Proceed? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Operation cancelled.")
		return nil
	}

	// Find inputs section
	inputsIndex := strings.Index(contentStr, "inputs = {")
	if inputsIndex == -1 {
		return fmt.Errorf("inputs section not found in flake.nix")
	}

	// Find the closing brace of inputs
	braceCount := 0
	closeIndex := inputsIndex + 9 // Start after "inputs = {"
	for i := closeIndex; i < len(contentStr); i++ {
		if contentStr[i] == '{' {
			braceCount++
		} else if contentStr[i] == '}' {
			braceCount--
			if braceCount == 0 {
				closeIndex = i
				break
			}
		}
	}

	if braceCount != 0 {
		return fmt.Errorf("could not find closing brace for inputs section")
	}

	// Insert input before closing brace
	newContent := contentStr[:closeIndex] + fmt.Sprintf("    %s.url = \"%s\";\n", inputName, finalURL)

	// Add any additional lines (like follows)
	for _, line := range additionalLines {
		newContent += line + "\n"
	}

	newContent += contentStr[closeIndex:]

	// Write back
	err = os.WriteFile(flakePath, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing flake.nix: %v", err)
	}

	fmt.Printf("Added input '%s' with URL '%s' to flake\n", inputName, finalURL)
	for _, line := range additionalLines {
		fmt.Printf("Added: %s\n", line)
	}

	return nil
}

// Extract and list all inputs from flake.nix
func listInputs(flakePath string) error {
	// Read flake.nix
	content, err := os.ReadFile(flakePath)
	if err != nil {
		return fmt.Errorf("error reading flake.nix: %v", err)
	}

	contentStr := string(content)

	// Find inputs section
	inputsIndex := strings.Index(contentStr, "inputs = {")
	if inputsIndex == -1 {
		return fmt.Errorf("inputs section not found in flake.nix")
	}

	// Find the closing brace of inputs
	braceCount := 0
	closeIndex := inputsIndex + 9 // Start after "inputs = {"
	for i := closeIndex; i < len(contentStr); i++ {
		if contentStr[i] == '{' {
			braceCount++
		} else if contentStr[i] == '}' {
			braceCount--
			if braceCount == 0 {
				closeIndex = i
				break
			}
		}
	}

	if braceCount != 0 {
		return fmt.Errorf("could not find closing brace for inputs section")
	}

	// Extract inputs section
	inputsSection := contentStr[inputsIndex : closeIndex+1]

	fmt.Println("Flake Inputs:")
	fmt.Println("================")

	// Parse inputs
	lines := strings.Split(inputsSection, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ".url =") && !strings.HasPrefix(line, "#") {
			// Extract input name and URL
			parts := strings.Split(line, ".url =")
			if len(parts) == 2 {
				inputName := strings.TrimSpace(parts[0])
				inputURL := strings.Trim(strings.TrimSpace(parts[1]), "\";")
				fmt.Printf("- %s -> %s\n", inputName, inputURL)
			}
		} else if strings.Contains(line, ".follows =") && !strings.HasPrefix(line, "#") {
			// Handle follows
			parts := strings.Split(line, ".follows =")
			if len(parts) == 2 {
				inputName := strings.TrimSpace(parts[0])
				follows := strings.Trim(strings.TrimSpace(parts[1]), "\";")
				fmt.Printf("- %s -> follows %s\n", inputName, follows)
			}
		}
	}

	return nil
}

// Extract modules from inputs (for inputs that have modules)
func extractInputModules(flakePath string) error {
	// Read flake.nix
	content, err := os.ReadFile(flakePath)
	if err != nil {
		return fmt.Errorf("error reading flake.nix: %v", err)
	}

	contentStr := string(content)

	fmt.Println("Available Input Modules:")
	fmt.Println("===========================")

	// Find inputs section
	inputsIndex := strings.Index(contentStr, "inputs = {")
	if inputsIndex == -1 {
		return fmt.Errorf("inputs section not found in flake.nix")
	}

	// Find the closing brace of inputs
	braceCount := 0
	closeIndex := inputsIndex + 9 // Start after "inputs = {"
	for i := closeIndex; i < len(contentStr); i++ {
		if contentStr[i] == '{' {
			braceCount++
		} else if contentStr[i] == '}' {
			braceCount--
			if braceCount == 0 {
				closeIndex = i
				break
			}
		}
	}

	if braceCount != 0 {
		return fmt.Errorf("could not find closing brace for inputs section")
	}

	// Extract inputs section
	inputsSection := contentStr[inputsIndex : closeIndex+1]

	// Parse inputs and suggest modules
	lines := strings.Split(inputsSection, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, ".url =") && !strings.HasPrefix(line, "#") {
			parts := strings.Split(line, ".url =")
			if len(parts) == 2 {
				inputName := strings.TrimSpace(parts[0])
				inputURL := strings.Trim(strings.TrimSpace(parts[1]), "\";")

				// Suggest common module patterns
				if strings.Contains(inputURL, "home-manager") {
					fmt.Printf("- %s.nixosModules.home-manager\n", inputName)
					fmt.Printf("- %s.homeManagerModules.default\n", inputName)
				} else if strings.Contains(inputURL, "flatpak") || strings.Contains(inputURL, "nix-flatpak") {
					fmt.Printf("- %s.nixosModules.nix-flatpak\n", inputName)
					fmt.Printf("- %s.homeManagerModules.nix-flatpak\n", inputName)
				} else if strings.Contains(inputURL, "hyprland") {
					fmt.Printf("- %s.nixosModules.default\n", inputName)
					fmt.Printf("- %s.homeManagerModules.default\n", inputName)
				} else if strings.Contains(inputURL, "spicetify") {
					fmt.Printf("- %s.nixosModules.default\n", inputName)
					fmt.Printf("- %s.homeManagerModules.default\n", inputName)
				} else {
					// Generic suggestions
					fmt.Printf("- %s.nixosModules.default\n", inputName)
					fmt.Printf("- %s.homeManagerModules.default\n", inputName)
				}
			}
		}
	}

	return nil
}

// getLatestNixpkgsVersion fetches the latest nixpkgs version from multiple sources
func getLatestNixpkgsVersion() (string, error) {
	sources := []struct {
		name string
		url  string
	}{
		{"GitHub Branches", "https://api.github.com/repos/NixOS/nixpkgs/branches?per_page=100"},
		{"GitHub Releases", "https://api.github.com/repos/NixOS/nixpkgs/releases?per_page=100"},
		{"Nix Channels", "https://channels.nixos.org/"},
		{"Nix Homepage", "https://nixos.org/"},
	}

	for _, source := range sources {
		log.Printf("Trying to get version from %s: %s", source.name, source.url)
		version, err := parseVersionFromSource(source.url)
		if err != nil {
			log.Printf("Failed to parse version from %s: %v", source.name, err)
			continue
		}
		if version != "" {
			log.Printf("Successfully got version %s from %s", version, source.name)
			return version, nil
		}
	}

	return "", fmt.Errorf("failed to get version from all sources")
}

// Update nixpkgs version in flake.nix
func updateNixpkgsVersion(flakePath, newVersion string) error {
	// Read the flake file
	content, err := os.ReadFile(flakePath)
	if err != nil {
		return fmt.Errorf("error reading flake.nix: %v", err)
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")
	updated := false

	// Find and update nixpkgs.url line
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "nixpkgs.url =") && !strings.HasPrefix(line, "#") {
			// Replace the version in the URL
			if strings.Contains(lines[i], "nixos-") {
				// Replace existing version
				re := regexp.MustCompile(`nixos-[0-9]+\.[0-9]+`)
				lines[i] = re.ReplaceAllString(lines[i], "nixos-"+newVersion)
			} else {
				// Add version if not present
				re := regexp.MustCompile(`(nixpkgs\.url\s*=\s*".*github\.com/NixOS/nixpkgs)(.*")`)
				lines[i] = re.ReplaceAllString(lines[i], "${1}/nixos-"+newVersion+"${2}")
			}
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("nixpkgs.url not found in flake.nix")
	}

	// Write back to file
	err = os.WriteFile(flakePath, []byte(strings.Join(lines, "\n")), 0644)
	if err != nil {
		return fmt.Errorf("error writing flake.nix: %v", err)
	}

	return nil
}

// parseVersionFromSource attempts to parse version from a given URL
func parseVersionFromSource(url string) (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch %s: %v", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	// Try different parsing strategies based on the URL
	if strings.Contains(url, "api.github.com") && strings.Contains(url, "branches") {
		return parseGitHubBranches(body)
	} else if strings.Contains(url, "api.github.com") && strings.Contains(url, "releases") {
		return parseGitHubReleases(body)
	} else if strings.Contains(url, "channels.nixos.org") {
		return parseNixChannels(body)
	} else if strings.Contains(url, "nixos.org") {
		return parseNixHomepage(body)
	}

	return "", fmt.Errorf("no parser available for URL: %s", url)
}

func parseGitHubBranches(body []byte) (string, error) {
	var branches []struct {
		Name string `json:"name"`
	}

	err := json.Unmarshal(body, &branches)
	if err != nil {
		return "", fmt.Errorf("failed to parse branches JSON: %v", err)
	}

	var candidateVersions []string
	for _, branch := range branches {
		if strings.HasPrefix(branch.Name, "nixos-") && !strings.Contains(branch.Name, "-small") {
			version := strings.TrimPrefix(branch.Name, "nixos-")
			// Skip unstable/development branches
			if version != "unstable" {
				candidateVersions = append(candidateVersions, version)
			}
		}
	}

	if len(candidateVersions) == 0 {
		return "", fmt.Errorf("no valid nixos branches found")
	}

	// Sort versions and return the latest
	sort.Strings(candidateVersions)
	return candidateVersions[len(candidateVersions)-1], nil
}

// parseGitHubReleases parses version from GitHub releases API response
func parseGitHubReleases(body []byte) (string, error) {
	var releases []struct {
		TagName string `json:"tag_name"`
	}

	err := json.Unmarshal(body, &releases)
	if err != nil {
		return "", fmt.Errorf("failed to parse releases JSON: %v", err)
	}

	var candidateVersions []string
	for _, release := range releases {
		if strings.HasPrefix(release.TagName, "nixos-") {
			version := strings.TrimPrefix(release.TagName, "nixos-")
			candidateVersions = append(candidateVersions, version)
		}
	}

	if len(candidateVersions) == 0 {
		return "", fmt.Errorf("no valid nixos releases found")
	}

	// Sort and return the latest version
	sort.Strings(candidateVersions)
	return candidateVersions[len(candidateVersions)-1], nil
}

// parseNixChannels parses version from Nix channels HTML response
func parseNixChannels(body []byte) (string, error) {
	bodyStr := string(body)
	lines := strings.Split(bodyStr, "\n")
	var foundVersions []string

	for _, line := range lines {
		if strings.Contains(line, "nixos-") {
			re := regexp.MustCompile(`nixos-(\d+\.\d+)`)
			matches := re.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					version := match[1]
					// Avoid duplicates
					if !contains(foundVersions, version) {
						foundVersions = append(foundVersions, version)
					}
				}
			}
		}
	}

	if len(foundVersions) == 0 {
		return "", fmt.Errorf("no versions found in Nix channels")
	}

	// Sort and return the latest version
	sort.Strings(foundVersions)
	return foundVersions[len(foundVersions)-1], nil
}

// parseNixHomepage parses version from Nix homepage HTML response
func parseNixHomepage(body []byte) (string, error) {
	bodyStr := string(body)
	lines := strings.Split(bodyStr, "\n")
	var foundVersions []string

	// Look for version patterns in the homepage - be more specific for nixpkgs versions
	re := regexp.MustCompile(`(\d{2}\.\d{2})`) // Look for XX.XX pattern (like 24.05)
	for _, line := range lines {
		if strings.Contains(line, "nixos") || strings.Contains(line, "NixOS") || strings.Contains(line, "release") {
			matches := re.FindAllString(line, -1)
			for _, match := range matches {
				// Validate that it's a reasonable nixpkgs version (between 20.00 and 30.00)
				if len(match) == 5 { // XX.XX format
					parts := strings.Split(match, ".")
					if len(parts) == 2 {
						major, err1 := strconv.Atoi(parts[0])
						minor, err2 := strconv.Atoi(parts[1])
						if err1 == nil && err2 == nil && major >= 20 && major <= 30 && minor >= 0 && minor <= 12 {
							if !contains(foundVersions, match) {
								foundVersions = append(foundVersions, match)
							}
						}
					}
				}
			}
		}
	}

	if len(foundVersions) == 0 {
		return "", fmt.Errorf("no valid nixpkgs versions found on Nix homepage")
	}

	// Sort and return the latest version
	sort.Strings(foundVersions)
	return foundVersions[len(foundVersions)-1], nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
