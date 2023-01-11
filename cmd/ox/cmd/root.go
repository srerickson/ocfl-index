/*
Copyright (c) 2022 The Regents of the University of California.
*/
package cmd

import (
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/gen/ocfl/v0/ocflv0connect"
)

const (
	envRemote = "OCFL_INDEX"
)

type oxCmd struct {
	cmd cobra.Command

	// flag values used by multiple commands
	commonFlags struct {
		vnum string // version
	}
}

func (ox oxCmd) serviceClient() ocflv0connect.IndexServiceClient {
	// TODO timeout and client settings
	serURL := getenvDefault(envRemote, "http://localhost:8080")
	cli := http.DefaultClient
	return ocflv0connect.NewIndexServiceClient(cli, serURL)
}

var root = &oxCmd{
	cmd: cobra.Command{
		Use:   "ox",
		Short: "ocfl-index client",
		Long:  ``,
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := root.cmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cmd.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	root.cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
