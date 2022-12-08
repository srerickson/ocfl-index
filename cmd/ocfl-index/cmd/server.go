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
	Use:   "server",
	Short: "server",
	Long:  ``,
	Run: func(cmd *coral.Command, args []string) {

		port := ":8080"
		if len(args) > 0 {
			port = args[0]
		}
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
		log.Println("starting server on", port)
		if err := http.ListenAndServe(port, srv); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
