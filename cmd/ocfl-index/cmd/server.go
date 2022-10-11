/*
Copyright © 2022
*/
package cmd

import (
	"log"

	"github.com/muesli/coral"
	"github.com/srerickson/ocfl-index/server"
)

var serveCmd = &coral.Command{
	Use:   "serve",
	Short: "serve content",
	Long:  ``,
	Run: func(cmd *coral.Command, args []string) {
		if err := server.Start(cmd.Context(), dbName, "localhost:8800"); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
