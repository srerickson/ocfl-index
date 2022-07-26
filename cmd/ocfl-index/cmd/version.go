/*
Copyright © 2022

*/
package cmd

import (
	"fmt"

	"github.com/muesli/coral"
	index "github.com/srerickson/ocfl-index"
)

// benchmarkCmd represents the benchmark command
var versionCmd = &coral.Command{
	Use:   "version",
	Short: "print version information",
	Long:  ``,
	Run: func(cmd *coral.Command, args []string) {
		fmt.Println(`ocfl-index`, index.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
