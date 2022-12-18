/*
Copyright © 2022
*/
package cmd

import (
	"log"
	"net/http"

	"github.com/muesli/coral"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/internal/server"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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
		if err := setupFS(cmd.Context(), &fsFlags); err != nil {
			log.Fatal(err)
		}
		db, err := sqlite.New("file:" + dbFlag)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		idx := index.NewIndex(db, fsFlags.fs, fsFlags.rootDir)
		if err := idx.Init(cmd.Context()); err != nil {
			log.Fatal(err)
		}
		srv, err := server.NewHandler(idx)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("starting server on", port)
		if err := http.ListenAndServe(port, h2c.NewHandler(srv, &http2.Server{})); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
