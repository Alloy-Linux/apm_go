package main

import (
	"bufio"
	"os"
	"strings"
)

// List packages
func ListInstalledPackages(flakeLocation string, method InstallationMethod) ([]string, error) {
	files, err := ListFilePaths(flakeLocation)
	if err != nil {
		return nil, err
	}

	var blockName string
	switch method {
	case NixEnv:
		blockName = "environment.systemPackages"
	case Flatpak:
		blockName = "services.flatpak.packages"
	case HomeManager:
		blockName = "home.packages"
	default:
		return nil, nil
	}

	var results []string
	for _, f := range files {
		if !strings.HasSuffix(f, ".nix") {
			continue
		}
		entries, err := readBlockEntries(f, blockName)
		if err != nil {
			// ignore unreadable files
			continue
		}
		results = append(results, entries...)
	}

	return results, nil
}

// Read block
func readBlockEntries(path, blockName string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var inBlock bool
	var entries []string
	for scanner.Scan() {
		line := scanner.Text()
		if !inBlock && strings.Contains(line, blockName) {
			// wait for '[' line
			if strings.Contains(line, "[") {
				inBlock = true
				continue
			}
			continue
		}
		if inBlock {
			if strings.Contains(line, "]") {
				break
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "[" {
				continue
			}
			entries = append(entries, trimmed)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
