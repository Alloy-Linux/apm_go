package main

import (
	cache "alloylinux/apm/src/database"
	"log"

	"github.com/spf13/cobra"
)

func main() {

	var rootCmd = &cobra.Command{
		Use:   "apm",
		Short: "Apm is a CLI tool for managing packages on Alloy Linux and other NixOS-based systems.",
	}
	/*
		var add = &cobra.Command{
			Use:   "add [package]",
			Short: "Add a package to the system.",
			Args:  cobra.ExactArgs(1),
			Run: func(cmd *cobra.Command, args []string) {
				fmt.Printf("Adding package: %s\n", args[0])
			},
		}
	*/

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
	rootCmd.AddCommand(makecacheCmd)
	rootCmd.AddCommand(removecacheCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
