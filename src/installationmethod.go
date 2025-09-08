package main

type InstallationMethod int

const (
	// System
	NixEnv InstallationMethod = iota

	// Flatpak
	Flatpak

	// Home manager
	HomeManager
)
