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

	// Get all .nix files
	files, err := ListFilePaths(flakeLocation)
	if err != nil {
		fmt.Printf("Error reading files: %v\n", err)
		return
	}

	modified := false
	filesProcessed := 0
	block := blockNameForMethod(method)
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
			fmt.Printf("✓ Added %s to %s\n", pkgName, f)
			modified = true
		case InsertAlreadyPresent:
			fmt.Printf("✓ %s already in %s\n", pkgName, f)
		case InsertError:
			// Only show real file errors
			if _, err := os.ReadFile(f); err != nil {
				fmt.Printf("✗ File error: %s\n", f)
			}
			// Skip files without block
		}
	}

	// Show result summary
	if !modified {
		if filesProcessed == 0 {
			fmt.Println("✗ No .nix files found.")
		} else {
			fmt.Printf("✗ No file with '%s' block found.\n", block)
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
		fmt.Printf("✗ Home directory error: %v\n", err)
		return false
	}
	apmDir := homedir + "/.cache/apm"
	dbPath := apmDir + "/apm.db"

	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Printf("✗ Database error: %v\n", err)
		return false
	}

	var pkg PackageInfo
	result := db.WithContext(ctx).Where("pname = ?", pkgName).First(&pkg)
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
	var results []PackageInfo
	for _, app := range apps {
		results = append(results, PackageInfo{
			Pname:       app.FlatpakAppId,
			Description: app.Summary,
			Version:     "",
		})
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
	ctx := context.Background()
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	var results []PackageInfo
	// Search by name
	err = db.WithContext(ctx).Where("pname LIKE ?", "%"+query+"%").Find(&results).Error
	if err != nil {
		return nil, err
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
