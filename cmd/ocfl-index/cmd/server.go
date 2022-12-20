/*
Copyright © 2022
*/
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/go-logr/logr"
	"github.com/iand/logfmtr"
	"github.com/muesli/coral"
	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/internal/server"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

type serverConfig struct {
	port   string
	fsys   ocfl.FS
	root   string
	dbfile string
	log    logr.Logger
}

var serveCmd = &coral.Command{
	Use:   "server",
	Short: "server",
	Long:  ``,
	Run: func(cmd *coral.Command, args []string) {
		ctx := cmd.Context()
		port := ":8080"
		log := logfmtr.NewWithOptions(logfmtr.Options{
			Writer:    os.Stderr,
			Humanize:  true,
			NameDelim: "/",
		})
		if len(args) > 0 {
			port = args[0]
		}
		if err := setupFS(ctx, &fsFlags); err != nil {
			log.Error(err, "connecting to file system")
			return
		}
		log.Info("fs settings", "driver", fsFlags.Driver, "bucket", fsFlags.Bucket, "path", fsFlags.Path)
		conf := &serverConfig{
			port:   port,
			fsys:   fsFlags.fs,
			root:   fsFlags.rootDir,
			dbfile: dbFlag,
			log:    log,
		}
		if err := startServer(ctx, conf); err != nil {
			log.Error(err, "quitting")
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func startServer(ctx context.Context, conf *serverConfig) error {
	db, err := sqlite.Open("file:" + conf.dbfile)
	if err != nil {
		return err
	}
	created, err := db.InitSchema(ctx)
	if err != nil {
		return err
	}
	maj, min, err := db.GetSchemaVersion(ctx)
	if err != nil {
		return err
	}
	defer db.Close()
	schemaV := fmt.Sprintf("%d.%d", maj, min)
	conf.log.Info("using index file", "file", conf.dbfile, "schema", schemaV)
	idx := index.NewIndex(db, conf.fsys, conf.root, index.WithLogger(conf.log))
	if created {
		go func() {
			// initial indexing
			idx.DoIndex(ctx)
		}()
	}
	srv, err := server.NewHandler(idx)
	if err != nil {
		return err
	}
	conf.log.Info("starting http server", "port", conf.port)
	if err := http.ListenAndServe(conf.port, h2c.NewHandler(srv, &http2.Server{})); err != nil {
		return err
	}
	conf.log.Info("http server stopped")
	return nil
}
