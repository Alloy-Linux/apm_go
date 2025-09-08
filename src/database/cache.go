package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type PackageInfo struct {
	Description string `json:"description"`
	Pname       string `json:"pname"`
	Version     string `json:"version"`
}

func MakeCache() {
	RemoveCache()
	ctx := context.Background()

	// Get JSON from nix
	output, err := exec.Command("nix", "search", "nixpkgs", "", "--json").Output()
	if err != nil {
		fmt.Printf("Error running nix search: %v\n", err)
		return
	}

	// Parse JSON
	var rawPackages map[string]PackageInfo
	err = json.Unmarshal(output, &rawPackages)
	if err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return
	}

	var packages []PackageInfo
	for _, pkg := range rawPackages {
		packages = append(packages, pkg)
	}

	fmt.Println("Found packages:")

	homedir, err := os.UserHomeDir()
	apmDir := homedir + "/.cache/apm"
	dbPath := apmDir + "/apm.db"

	// Ensure cache directory
	if err := os.MkdirAll(apmDir, 0o755); err != nil {
		fmt.Printf("Error creating apm cache directory: %v\n", err)
		return
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		return
	}

	db.AutoMigrate(&PackageInfo{})

	// Collect errors
	var errs []error

	for i, pkg := range packages {
		fmt.Printf("%3d. %s (v%s)\n    %s\n", i+1, pkg.Pname, pkg.Version, pkg.Description)

		err = db.WithContext(ctx).Create(&PackageInfo{
			Pname:       pkg.Pname,
			Version:     pkg.Version,
			Description: pkg.Description,
		}).Error

		if err != nil {
			fmt.Printf("Error inserting package %s: %v\n", pkg.Pname, err)
			errs = append(errs, err)
		}
	}

	for i, err := range errs {
		fmt.Printf("Error %d: %v\n", i+1, err)
	}
}

func RemoveCache() {
	homedir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting user home directory: %v\n", err)
		return
	}
	cacheFile := homedir + "/.cache/apm/apm.db"
	if err := os.Remove(cacheFile); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("Cache file does not exist.")
		} else {
			fmt.Printf("Error removing cache file: %v\n", err)
		}
	} else {
		fmt.Println("Cache file removed successfully.")
	}
}
