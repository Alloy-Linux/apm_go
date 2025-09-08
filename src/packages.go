package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Check if input exists in flake
func inputExistsInFlake(flakePath, inputName string) bool {
	content, err := os.ReadFile(flakePath)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), inputName+".url")
}

// Ensure unstable input exists
func ensureUnstableInput(flakeLocation string) error {
	flakePath := filepath.Join(flakeLocation, "flake.nix")

	// Check if unstable input already exists
	if inputExistsInFlake(flakePath, "unstable") {
		return nil
	}

	// Ask user if they want to add the unstable input
	fmt.Println("Unstable packages require the 'unstable' nixpkgs input.")
	fmt.Print("Add unstable input (github:NixOS/nixpkgs/nixos-unstable)? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Unstable input not added. Package installation may fail.")
		return nil
	}

	// Add the unstable input
	return addInput(flakePath, "unstable", "github:NixOS/nixpkgs/nixos-unstable")
}

// Install package
func installPackage(pkgName, flakeLocation string, method InstallationMethod, unstable bool) {
	// Skip Nixpkgs check for Flatpak
	if method != Flatpak && !doesPackageExist(pkgName) {
		fmt.Println("Package not found in Nixpkgs.")
		return
	}

	// Check Flathub availability
	if method == Flatpak {
		available, resolvedAppID := isFlatpakAvailable(pkgName)
		if !available {
			fmt.Printf("Flatpak '%s' not found.\n", pkgName)
			return
		}
		// Use resolved app ID
		pkgName = resolvedAppID
	}

	// Check if already installed
	if presentInFlake(pkgName, flakeLocation, method) {
		fmt.Printf("%s already installed.\n", pkgName)
		return
	}

	// Ensure unstable input exists if using unstable packages
	if unstable && method != Flatpak {
		err := ensureUnstableInput(flakeLocation)
		if err != nil {
			fmt.Printf("Error setting up unstable input: %v\n", err)
			return
		}
	}

	// Ask for confirmation before modifying files
	var methodName string
	switch method {
	case NixEnv:
		methodName = "NixEnv"
	case Flatpak:
		methodName = "Flatpak"
	case HomeManager:
		methodName = "HomeManager"
	default:
		methodName = "Unknown"
	}
	fmt.Printf("About to install '%s' (%s)\n", pkgName, methodName)
	fmt.Print("Proceed? [y/N]: ")
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Installation cancelled.")
		return
	}

	// Get all .nix files
	files, err := ListFilePaths(flakeLocation)
	if err != nil {
		fmt.Printf("Error reading files: %v\n", err)
		return
	}

	// Check if any file contains the required block
	block := blockNameForMethod(method)
	hasBlock := false
	for _, f := range files {
		if !strings.HasSuffix(f, ".nix") {
			continue
		}
		content, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), block) {
			hasBlock = true
			break
		}
	}

	// If no file has the required block, create the appropriate package file
	if !hasBlock {
		switch method {
		case HomeManager:
			fmt.Println("No home-manager packages file found. Creating one...")
			makeHomeEnv()
			// Re-get the file list after creating the file
			files, err = ListFilePaths(flakeLocation)
			if err != nil {
				fmt.Printf("Error reading files: %v\n", err)
				return
			}
		case NixEnv:
			fmt.Println("No Nix environment packages file found. Creating one...")
			makeNixEnv()
			// Re-get the file list after creating the file
			files, err = ListFilePaths(flakeLocation)
			if err != nil {
				fmt.Printf("Error reading files: %v\n", err)
				return
			}
		case Flatpak:
			fmt.Println("No Flatpak packages file found. Creating one...")
			setupFlatpak()
			// For Flatpak, we need to create the packages file manually since setupFlatpak doesn't do it
			homedir, err := os.UserHomeDir()
			if err != nil {
				fmt.Printf("Error getting home directory: %v\n", err)
				return
			}
			flakeLocationPath := filepath.Join(homedir, ".config", "apm", "flakelocation.txt")
			flakeDir, err := readFlakeLocation(flakeLocationPath)
			if err != nil {
				fmt.Printf("Error reading flake location: %v\n", err)
				return
			}
			createPackageFile(flakeDir, "flatpak-packages.nix", "services.flatpak.packages", flatpakPackagesBoilerplate, "./packages/flatpak-packages.nix")
			// Re-get the file list after creating the file
			files, err = ListFilePaths(flakeLocation)
			if err != nil {
				fmt.Printf("Error reading files: %v\n", err)
				return
			}
		}
	}

	modified := false
	filesProcessed := 0
	// Process each .nix file
	for _, f := range files {
		if !strings.HasSuffix(f, ".nix") {
			continue
		}

		entry := buildEntry(pkgName, method, unstable)

		res := insertIntoNixBlock(f, block, entry, method)
		filesProcessed++
		switch res {
		case InsertAdded:
			fmt.Printf("Added %s to %s\n", pkgName, f)
			modified = true
		case InsertAlreadyPresent:
			fmt.Printf("%s already in %s\n", pkgName, f)
		case InsertError:
			// Only show real file errors
			if _, err := os.ReadFile(f); err != nil {
				fmt.Printf("File error: %s\n", f)
			}
			// Skip files without block
		}
	}

	// Show result summary
	if !modified {
		if filesProcessed == 0 {
			fmt.Println("No .nix files found.")
		} else {
			fmt.Printf("No file with '%s' block found.\n", block)
		}
	}
}

// Build entry
func buildEntry(pkgName string, method InstallationMethod, unstable bool) string {
	// Create package entry string
	switch method {
	case Flatpak:
		return fmt.Sprintf(`{ appId = "%s"; origin = "flathub"; }`, pkgName)
	case HomeManager, NixEnv:
		if unstable {
			if strings.HasPrefix(pkgName, "unstable.") {
				return pkgName
			}
			return "unstable." + pkgName
		}
		if strings.HasPrefix(pkgName, "pkgs.") || strings.HasPrefix(pkgName, "unstable.") {
			return pkgName
		}
		return "pkgs." + pkgName
	default:
		return pkgName
	}
}

// Block name
func blockNameForMethod(method InstallationMethod) string {
	// Map method to config block
	switch method {
	case NixEnv:
		return "environment.systemPackages"
	case Flatpak:
		return "services.flatpak.packages"
	case HomeManager:
		return "home.packages"
	default:
		return ""
	}
}

// Check installed
func presentInFlake(pkgName, flakeLocation string, method InstallationMethod) bool {
	installed, err := ListInstalledPackages(flakeLocation, method)
	if err != nil {
		return false
	}
	for _, e := range installed {
		t := strings.TrimSpace(e)
		if method == Flatpak {
			if strings.Contains(t, pkgName) || strings.Contains(t, fmt.Sprintf("appId = \"%s\"", pkgName)) {
				return true
			}
		} else {
			if t == pkgName || t == "pkgs."+pkgName || t == "unstable."+pkgName {
				return true
			}
		}
	}
	return false
}

type InsertStatus int

const (
	InsertError InsertStatus = iota
	InsertAdded
	InsertAlreadyPresent
)

func insertIntoNixBlock(file, blockName, entry string, method InstallationMethod) InsertStatus {
	data, err := os.ReadFile(file)
	if err != nil {
		return InsertError
	}
	lines := strings.Split(string(data), "\n")

	// Find block name line
	blockLineIdx := -1
	for i, l := range lines {
		if strings.Contains(l, blockName) {
			blockLineIdx = i
			break
		}
	}
	if blockLineIdx == -1 {
		// Block not found
		return InsertError
	}

	// Find opening bracket
	openIdx := -1
	for i := blockLineIdx; i < len(lines); i++ {
		if strings.Contains(lines[i], "[") {
			openIdx = i
			break
		}
	}
	if openIdx == -1 {
		return InsertError
	}

	// Find closing bracket
	closeIdx := -1
	for i := openIdx; i < len(lines); i++ {
		if strings.Contains(lines[i], "]") {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return InsertError
	}

	// Check if already exists
	alreadyPresent := false
	for i := openIdx + 1; i < closeIdx; i++ {
		l := lines[i]
		switch method {
		case Flatpak:
			if strings.Contains(l, entry) || (strings.Contains(l, "appId") && strings.Contains(l, strings.Split(entry, `"`)[1])) {
				alreadyPresent = true
			}
		default:
			if strings.TrimSpace(l) == entry {
				alreadyPresent = true
			}
		}
		if alreadyPresent {
			break
		}
	}

	if alreadyPresent {
		return InsertAlreadyPresent
	}

	// Add entry before closing bracket
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:closeIdx]...)
	newLines = append(newLines, "    "+entry)
	newLines = append(newLines, lines[closeIdx:]...)

	err = os.WriteFile(file, []byte(strings.Join(newLines, "\n")), 0644)
	if err != nil {
		return InsertError
	}
	return InsertAdded
}

type PackageInfo struct {
	Description string
	Pname       string
	Version     string
}

func doesPackageExist(pkgName string) bool {
	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("X Home directory error: %v\n", err)
		return false
	}
	apmDir := homedir + "/.cache/apm"
	dbPath := apmDir + "/apm.db"

	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Println("No local database found! Generate it with 'apm makecache'")
		return false
	}

	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Printf("X Database error: %v\n", err)
		return false
	}

	var pkg PackageInfo
	result := db.WithContext(ctx).Where("pname = ?", pkgName).First(&pkg)

	// Check for table not found error
	if result.Error != nil && strings.Contains(result.Error.Error(), "no such table") {
		fmt.Println("No local database found! Generate it with 'apm makecache'")
		return false
	}

	return result.Error == nil
}

func ListFilePaths(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".nix") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return paths, nil
}

func searchFlathub(query string) ([]PackageInfo, error) {
	encodedQuery := url.QueryEscape(query)
	url := fmt.Sprintf("https://flathub.org/api/v1/apps/search/%s", encodedQuery)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("Flathub API error: %s", resp.Status)
	}
	var apps []struct {
		FlatpakAppId string `json:"flatpakAppId"`
		Name         string `json:"name"`
		Summary      string `json:"summary"`
	}
	err = json.NewDecoder(resp.Body).Decode(&apps)
	if err != nil {
		return nil, err
	}

	// Sort results by relevance: exact matches, then starts with, then contains
	var exactMatches []struct {
		FlatpakAppId string `json:"flatpakAppId"`
		Name         string `json:"name"`
		Summary      string `json:"summary"`
	}
	var startsWithMatches []struct {
		FlatpakAppId string `json:"flatpakAppId"`
		Name         string `json:"name"`
		Summary      string `json:"summary"`
	}
	var containsMatches []struct {
		FlatpakAppId string `json:"flatpakAppId"`
		Name         string `json:"name"`
		Summary      string `json:"summary"`
	}

	for _, app := range apps {
		appIdLower := strings.ToLower(app.FlatpakAppId)
		queryLower := strings.ToLower(query)

		if appIdLower == queryLower {
			exactMatches = append(exactMatches, app)
		} else if strings.HasPrefix(appIdLower, queryLower) {
			startsWithMatches = append(startsWithMatches, app)
		} else if strings.Contains(appIdLower, queryLower) {
			containsMatches = append(containsMatches, app)
		}
	}

	var results []PackageInfo
	// Add exact matches first
	for _, app := range exactMatches {
		results = append(results, PackageInfo{
			Pname:       app.FlatpakAppId,
			Description: app.Summary,
			Version:     "",
		})
	}
	// Then starts with matches
	for _, app := range startsWithMatches {
		results = append(results, PackageInfo{
			Pname:       app.FlatpakAppId,
			Description: app.Summary,
			Version:     "",
		})
	}
	// Then contains matches
	for _, app := range containsMatches {
		results = append(results, PackageInfo{
			Pname:       app.FlatpakAppId,
			Description: app.Summary,
			Version:     "",
		})
	}

	// Limit to 10 results
	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}

func SearchPackages(query string, method InstallationMethod) ([]PackageInfo, error) {
	if method == Flatpak {
		return searchFlathub(query)
	}
	homedir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := homedir + "/.cache/apm/apm.db"

	// Check if database file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("no local database found! Generate it with 'apm makecache'")
	}

	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	var results []PackageInfo
	var exactMatches []PackageInfo
	var startsWithMatches []PackageInfo
	var containsMatches []PackageInfo

	// First, find exact matches
	err = db.WithContext(ctx).Where("pname = ?", query).Find(&exactMatches).Error
	if err != nil {
		// Check for table not found error
		if strings.Contains(err.Error(), "no such table") {
			return nil, fmt.Errorf("no local database found! Generate it with 'apm makecache'")
		}
		return nil, err
	}

	// Then, find packages that start with the query
	err = db.WithContext(ctx).Where("pname LIKE ?", query+"%").Find(&startsWithMatches).Error
	if err != nil {
		return nil, err
	}

	// Finally, find packages that contain the query (but don't start with it)
	err = db.WithContext(ctx).Where("pname LIKE ? AND pname NOT LIKE ?", "%"+query+"%", query+"%").Find(&containsMatches).Error
	if err != nil {
		return nil, err
	}

	// Combine results in order of relevance, limiting to 10 total
	results = append(results, exactMatches...)
	results = append(results, startsWithMatches...)
	results = append(results, containsMatches...)

	// Limit to 10 results
	if len(results) > 10 {
		results = results[:10]
	}

	return results, nil
}

func isFlatpakAvailable(appID string) (bool, string) {
	// If no dots, treat as search term
	if !strings.Contains(appID, ".") {
		// Search and get first result
		results, err := searchFlathub(appID)
		if err != nil || len(results) == 0 {
			return false, ""
		}
		appID = results[0].Pname
	}

	url := fmt.Sprintf("https://flathub.org/api/v1/apps/%s", appID)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, ""
	}
	// Check if response is valid
	var result interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return false, ""
	}
	return result != nil, appID
}
