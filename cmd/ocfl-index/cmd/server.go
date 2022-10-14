/*
Copyright © 2022
*/
package cmd

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/muesli/coral"
	"github.com/srerickson/ocfl-index/server"
	"github.com/srerickson/ocfl-index/sqlite"
)

var serveCmd = &coral.Command{
	Use:   "serve",
	Short: "serve content",
	Long:  ``,
	Run: func(cmd *coral.Command, args []string) {
		db, err := sql.Open("sqlite", "file:"+dbFlag)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		idx := sqlite.New(db)
		if err := setupFS(cmd.Context(), &fsFlags); err != nil {
			log.Fatal(err)
		}
		srv, err := server.New(fsFlags.fs, fsFlags.rootDir, idx)
		if err != nil {
			log.Fatal(err)
		}
		if err := http.ListenAndServe("localhost:8800", srv); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}