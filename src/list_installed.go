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
			// Check if '[' is on the same line as blockName
			if strings.Contains(line, "[") {
				inBlock = true
				// If there are entries on the same line after '[', extract them
				bracketIndex := strings.Index(line, "[")
				afterBracket := line[bracketIndex+1:]
				trimmed := strings.TrimSpace(afterBracket)
				if trimmed != "" && !strings.HasPrefix(trimmed, "]") {
					// Remove trailing comments and brackets
					if idx := strings.Index(trimmed, "#"); idx != -1 {
						trimmed = trimmed[:idx]
					}
					trimmed = strings.TrimRight(trimmed, " ]")
					if trimmed != "" {
						entries = append(entries, strings.TrimSpace(trimmed))
					}
				}
				continue
			}
			// If '[' is not on this line, wait for the next line with '['
			continue
		}
		if inBlock {
			if strings.Contains(line, "]") {
				// Extract any remaining entries before the closing bracket
				beforeBracket := line[:strings.Index(line, "]")]
				trimmed := strings.TrimSpace(beforeBracket)
				if trimmed != "" {
					// Remove trailing comments
					if idx := strings.Index(trimmed, "#"); idx != -1 {
						trimmed = trimmed[:idx]
					}
					if trimmed != "" {
						entries = append(entries, strings.TrimSpace(trimmed))
					}
				}
				break
			}
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			// Remove comments
			if idx := strings.Index(trimmed, "#"); idx != -1 {
				trimmed = trimmed[:idx]
			}
			trimmed = strings.TrimSpace(trimmed)
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
