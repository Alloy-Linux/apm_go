package main

// import cobra
import (
	"fmt"
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

	// Add commands to the root command
	rootCmd.AddCommand(add)



	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}