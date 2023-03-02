package cmd

import (
	"os"

	"github.com/go-logr/logr"
	"github.com/iand/logfmtr"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/export"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/ls"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/root"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/status"
)

var rootCmd = root.Cmd{
	Command: cobra.Command{
		Use:   "ox {command}",
		Short: "ocfl-index client",
	},
	Log: defaultLogger(),
}

func Execute() {
	rootCmd.Init()
	rootCmd.AddSub(
		&status.Cmd{},
		&ls.Cmd{},
		&export.Cmd{},
	)
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func defaultLogger() logr.Logger {
	return logfmtr.NewWithOptions(logfmtr.Options{
		Writer:   os.Stderr,
		Humanize: true,
	})
}
