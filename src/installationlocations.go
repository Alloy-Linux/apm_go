package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
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
