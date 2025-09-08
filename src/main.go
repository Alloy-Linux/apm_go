package main

import (
	cache "alloylinux/apm/src/database"
	"fmt"
	"log"
	"os"
	"os/exec"
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
			// Read the actual flake directory from the config file
			flakeDir, err := readFlakeLocation(flakeLocationPath)
			if err != nil {
				fmt.Printf("Error reading flake location: %v\n", err)
				return
			}
			// Show installed packages
			pkgs, err := ListInstalledPackages(flakeDir, method)
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
		Short: "Set the flake path for package management.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			createFlakeLocationFile(configDir, args)
		},
	}

	var updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update the flake inputs.",
		Run: func(cmd *cobra.Command, args []string) {
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

			// Check if flake.lock exists and is writable
			flakeLockPath := filepath.Join(flakeDir, "flake.lock")
			if _, err := os.Stat(flakeLockPath); err == nil {
				fmt.Printf("Found flake.lock at: %s\n", flakeLockPath)
			}

			fmt.Println("Updating flake inputs...")
			cmdExec := exec.Command("sudo", "nix", "flake", "update")
			cmdExec.Dir = flakeDir
			cmdExec.Stdout = os.Stdout
			cmdExec.Stderr = os.Stderr
			if err := cmdExec.Run(); err != nil {
				log.Printf("Error running nix flake update with sudo: %v", err)
				fmt.Println("\nTroubleshooting:")
				fmt.Println("- Make sure you have sudo permissions")
				fmt.Println("- Check that your user is in the sudoers file")
				return
			}
			fmt.Println("Flake inputs updated successfully!")
		},
	}

	// Rebuild command
	var rebuildCmd = &cobra.Command{
		Use:   "rebuild",
		Short: "Rebuild the NixOS/Alloy system.",
		Run: func(cmd *cobra.Command, args []string) {
			// rebuild the system
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

			cmdExec := exec.Command("sudo", "nixos-rebuild", "switch", "--flake", flakeDir)
			cmdExec.Stdout = os.Stdout
			cmdExec.Stderr = os.Stderr
			if err := cmdExec.Run(); err != nil {
				log.Printf("Error running nixos-rebuild: %v", err)
				return
			}
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

	var makenixenvCmd = &cobra.Command{
		Use:   "makenixenv",
		Short: "Create Nix environment structure and packages file.",
		Run: func(cmd *cobra.Command, args []string) {
			makeNixEnv()
		},
	}

	var makehomeenvCmd = &cobra.Command{
		Use:   "makehomeenv",
		Short: "Create Home Manager packages file.",
		Run: func(cmd *cobra.Command, args []string) {
			makeHomeEnv()
		},
	}

	var setupflatpakCmd = &cobra.Command{
		Use:   "setupflatpak",
		Short: "Add Flatpak module to flake configuration.",
		Run: func(cmd *cobra.Command, args []string) {
			setupFlatpak()
		},
	}

	var addInputCmd = &cobra.Command{
		Use:   "add-input [name] [url]",
		Short: "Add an input to flake configuration.",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
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

			err = addInput(filepath.Join(flakeDir, "flake.nix"), args[0], args[1])
			if err != nil {
				log.Printf("Error adding input: %v", err)
			}
		},
	}

	var showNixpkgsVersionCmd = &cobra.Command{
		Use:   "show-nixpkgs-version",
		Short: "Show the current nixpkgs version in flake configuration.",
		Run: func(cmd *cobra.Command, args []string) {
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

			version, err := getNixpkgsVersion(filepath.Join(flakeDir, "flake.nix"))
			if err != nil {
				log.Printf("Error getting nixpkgs version: %v", err)
				return
			}

			fmt.Printf("Current nixpkgs version: %s\n", version)
		},
	}

	var updateNixpkgsCmd = &cobra.Command{
		Use:   "update-nixpkgs",
		Short: "Update nixpkgs to the latest stable version in flake configuration.",
		Run: func(cmd *cobra.Command, args []string) {
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

			flakePath := filepath.Join(flakeDir, "flake.nix")

			// Get current version
			currentVersion, err := getNixpkgsVersion(flakePath)
			if err != nil {
				log.Printf("Error getting current nixpkgs version: %v", err)
				return
			}

			fmt.Printf("Current nixpkgs version: %s\n", currentVersion)

			// Fetch latest stable version
			latestVersion, err := getLatestNixpkgsVersion()
			if err != nil {
				log.Printf("Error fetching latest nixpkgs version: %v", err)
				return
			}

			fmt.Printf("Latest stable version: %s\n", latestVersion)

			if currentVersion == latestVersion {
				fmt.Println("Already up to date!")
				return
			}

			// Ask for confirmation
			fmt.Printf("Update nixpkgs from %s to %s? [y/N]: ", currentVersion, latestVersion)
			var response string
			fmt.Scanln(&response)
			if strings.ToLower(strings.TrimSpace(response)) != "y" {
				fmt.Println("Update cancelled.")
				return
			}

			// Update the flake
			err = updateNixpkgsVersion(flakePath, latestVersion)
			if err != nil {
				log.Printf("Error updating nixpkgs version: %v", err)
				return
			}

			fmt.Printf("Successfully updated nixpkgs to version %s\n", latestVersion)

			// Update flake lock file
			fmt.Println("Updating flake lock file...")
			cmdExec := exec.Command("sudo", "nix", "flake", "update")
			cmdExec.Dir = flakeDir
			cmdExec.Stdout = os.Stdout
			cmdExec.Stderr = os.Stderr
			if err := cmdExec.Run(); err != nil {
				log.Printf("Warning: Failed to update flake lock file with sudo: %v", err)
				fmt.Println("You may need to check your sudo permissions.")
			} else {
				fmt.Println("Flake lock file updated successfully!")
			}

			fmt.Println("Run 'apm rebuild' to apply the changes.")
		},
	}

	var listInputsCmd = &cobra.Command{
		Use:   "list-inputs",
		Short: "List all inputs in flake configuration.",
		Run: func(cmd *cobra.Command, args []string) {
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

			err = listInputs(filepath.Join(flakeDir, "flake.nix"))
			if err != nil {
				log.Printf("Error listing inputs: %v", err)
			}
		},
	}

	var listModulesCmd = &cobra.Command{
		Use:   "list-modules",
		Short: "List available modules from flake inputs.",
		Run: func(cmd *cobra.Command, args []string) {
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

			err = extractInputModules(filepath.Join(flakeDir, "flake.nix"))
			if err != nil {
				log.Printf("Error extracting modules: %v", err)
			}
		},
	}

	// Add commands to the root command
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(listPackages)
	rootCmd.AddCommand(setFlakeLocation)
	rootCmd.AddCommand(makecacheCmd)
	rootCmd.AddCommand(removecacheCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(rebuildCmd)
	rootCmd.AddCommand(makenixenvCmd)
	rootCmd.AddCommand(makehomeenvCmd)
	rootCmd.AddCommand(setupflatpakCmd)
	rootCmd.AddCommand(addInputCmd)
	rootCmd.AddCommand(listInputsCmd)
	rootCmd.AddCommand(listModulesCmd)
	rootCmd.AddCommand(showNixpkgsVersionCmd)
	rootCmd.AddCommand(updateNixpkgsCmd)

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
