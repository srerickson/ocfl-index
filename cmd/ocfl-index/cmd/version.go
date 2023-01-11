/*
Copyright © 2022
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/internal/index"
)

// benchmarkCmd represents the benchmark command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "print version information",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(`ocfl-index`, index.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
