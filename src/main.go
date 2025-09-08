package main

import (
	cache "alloylinux/apm/src/database"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func main() {
	// Setup config paths
	homedir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Error getting home directory: %v", err)
	}

	configDir := filepath.Join(homedir, ".config", "apm")
	flakeLocationPath := filepath.Join(homedir, ".config", "apm", "flakelocation.txt")

	ensureFlakeLocationExists(configDir, flakeLocationPath)

	var rootCmd = &cobra.Command{
		Use:   "apm",
		Short: "Apm is a CLI tool for managing packages on Alloy Linux and other NixOS-based systems.",
	}

	var listPackages = &cobra.Command{
		Use:   "list",
		Short: "List installed packages for a given installation method.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			flatpak, _ := cmd.Flags().GetBool("flatpak")
			nixEnv, _ := cmd.Flags().GetBool("nix-env")
			homeManager, _ := cmd.Flags().GetBool("home-manager")
			method, err := DetermineMethod(flatpak, nixEnv, homeManager)
			if err != nil {
				fmt.Println("Error: " + err.Error())
				return
			}
			// Show installed packages
			pkgs, err := ListInstalledPackages(flakeLocationPath, method)
			if err != nil {
				fmt.Printf("Error listing packages: %v\n", err)
				return
			}
			for _, p := range pkgs {
				fmt.Println(p)
			}
		},
	}
	// add flags for list
	listPackages.Flags().Bool("flatpak", false, "List Flatpak packages")
	listPackages.Flags().Bool("nix-env", false, "List NixEnv packages")
	listPackages.Flags().Bool("home-manager", false, "List HomeManager packages")

	var addCmd = &cobra.Command{
		Use:   "add [package]",
		Short: "Add a package to configuration.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			flatpak, _ := cmd.Flags().GetBool("flatpak")
			nixEnv, _ := cmd.Flags().GetBool("nix-env")
			homeManager, _ := cmd.Flags().GetBool("home-manager")
			method, err := DetermineMethod(flatpak, nixEnv, homeManager)
			if err != nil {
				fmt.Println("Error: " + err.Error())
				return
			}
			// Get flake directory
			flakeDir, err := readFlakeLocation(flakeLocationPath)
			if err != nil {
				log.Fatalf("Error reading flake location file: %v", err)
			}
			unstable, _ := cmd.Flags().GetBool("unstable")
			exact, _ := cmd.Flags().GetBool("exact")

			if exact {
				// Install directly
				installPackage(args[0], flakeDir, method, unstable)
				return
			}

			// Search for packages
			candidates, err := SearchPackages(args[0], method)
			if err != nil {
				fmt.Printf("Error searching packages: %v\n", err)
				return
			}
			if len(candidates) == 0 {
				fmt.Println("No matching packages found. Try --exact or run makecache.")
				return
			}
			if len(candidates) == 1 {
				// Ask for confirmation
				fmt.Printf("Install '%s'? [y/N]: ", candidates[0].Pname)
				var ans string
				_, err = fmt.Scanln(&ans)
				if err != nil {
					fmt.Println("No selection made")
					return
				}
				if strings.ToLower(strings.TrimSpace(ans)) == "y" {
					installPackage(candidates[0].Pname, flakeDir, method, unstable)
				}
				return
			}
			// Show numbered list
			fmt.Println("Multiple matches found; choose one:")
			for i, p := range candidates {
				fmt.Printf("%d) %s - %s\n", i+1, p.Pname, p.Description)
			}
			var choice int
			fmt.Print("Select number: ")
			_, err = fmt.Scanln(&choice)
			if err != nil {
				fmt.Println("Invalid selection")
				return
			}
			if choice < 1 || choice > len(candidates) {
				fmt.Println("Selection out of range")
				return
			}
			installPackage(candidates[choice-1].Pname, flakeDir, method, unstable)
		},
	}
	// add --unstable flag
	addCmd.Flags().BoolP("unstable", "u", false, "Install from unstable channel")
	addCmd.Flags().BoolP("exact", "e", false, "Exact package name (no search)")
	// add method flags
	addCmd.Flags().Bool("flatpak", false, "Install as Flatpak")
	addCmd.Flags().Bool("nix-env", false, "Install as NixEnv")
	addCmd.Flags().Bool("home-manager", false, "Install as HomeManager")

	var setFlakeLocation = &cobra.Command{
		Use:   "set-flake-location [location]",
		Short: "Set the flake URL for package management.",
		Run: func(cmd *cobra.Command, args []string) {
			createFlakeLocationFile(configDir, args)
		},
	}

	var makecacheCmd = &cobra.Command{
		Use:   "makecache",
		Short: "Update the package cache.",
		Run: func(cmd *cobra.Command, args []string) {
			cache.MakeCache()
		},
	}

	var removecacheCmd = &cobra.Command{
		Use:   "removecache",
		Short: "Remove the package cache.",
		Run: func(cmd *cobra.Command, args []string) {
			cache.RemoveCache()
		},
	}

	// Add commands to the root command
	rootCmd.AddCommand(listPackages)
	rootCmd.AddCommand(setFlakeLocation)
	rootCmd.AddCommand(makecacheCmd)
	rootCmd.AddCommand(removecacheCmd)
	rootCmd.AddCommand(addCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func createFlakeLocationFile(configDir string, args []string) {
	// Create flake file
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		log.Fatalf("Error creating config directory: %v", err)
	}

	file, err := os.Create(configDir + "/flakelocation.txt")
	if err != nil {
		log.Fatalf("Error creating flake location file: %v", err)
	}
	defer file.Close()

	if _, err := file.Write([]byte(args[0])); err != nil {
		log.Fatalf("Error writing to flake location file: %v", err)
	}

	fmt.Printf("Flake location set to: %s\n", args[0])
}

// Ensure flake file
func ensureFlakeLocationExists(configDir, flakePath string) {
	if _, err := os.Stat(flakePath); os.IsNotExist(err) {
		if err := os.MkdirAll(configDir, 0o755); err != nil {
			log.Fatalf("Error creating config directory: %v", err)
		}
		file, err := os.Create(flakePath)
		if err != nil {
			log.Fatalf("Error creating flake location file: %v", err)
		}
		defer file.Close()
		if _, err := file.Write([]byte("/etc/nixos/")); err != nil {
			log.Fatalf("Error writing default flake location: %v", err)
		}
	}
}

// Read flake path
func readFlakeLocation(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
