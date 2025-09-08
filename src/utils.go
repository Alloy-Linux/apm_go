package main

import "fmt"

// Parse method string
func ParseMethod(s string) (InstallationMethod, error) {
	switch s {
	case "nix-env":
		return NixEnv, nil
	case "flatpak":
		return Flatpak, nil
	case "home-manager":
		return HomeManager, nil
	default:
		return -1, fmt.Errorf("invalid method")
	}
}

// Determine method from flags
func DetermineMethod(flatpak, nixEnv, homeManager bool) (InstallationMethod, error) {
	count := 0
	if flatpak {
		count++
	}
	if nixEnv {
		count++
	}
	if homeManager {
		count++
	}
	if count > 1 {
		return -1, fmt.Errorf("multiple methods specified")
	}
	if flatpak {
		return Flatpak, nil
	}
	if nixEnv {
		return NixEnv, nil
	}
	return HomeManager, nil
}
