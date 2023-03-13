/*
Copyright © 2022
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var serverFlags struct {
	skipIndexing bool // skip indexing on startup
	inventories  bool // indexing level
}

var serveCmd = &cobra.Command{
	Use:   "server",
	Short: "run gRPC service",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		logger := NewLogger()
		conf := NewConfig(logger)
		fsys, rootDir, err := conf.FS(ctx)
		if err != nil {
			logger.Error(err, "can't connect to backend")
			return
		}
		if closer, ok := fsys.(io.Closer); ok {
			defer closer.Close()
		}
		if err := startServer(ctx, &conf, fsys, rootDir); err != nil {
			logger.Error(err, "server stopped")
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().BoolVar(&serverFlags.skipIndexing, "skip-indexing", false, "skip indexing step on startup")
	serveCmd.Flags().BoolVar(&serverFlags.inventories, "inventories", false, "index inventories during reindex")
}

func startServer(ctx context.Context, c *config, fsys ocfl.FS, rootDir string) error {

	db, err := sqlite.Open("file:" + c.DBFile + "?" + sqliteSettings)
	if err != nil {
		return err
	}
	if _, err := db.InitSchema(ctx); err != nil {
		return fmt.Errorf("while initializing index tables: %w", err)
	}
	maj, min, err := db.GetSchemaVersion(ctx)
	if err != nil {
		return fmt.Errorf("while getting index schema version: %w", err)
	}
	defer db.Close()
	schemaV := fmt.Sprintf("%d.%d", maj, min)
	c.Logger.Info("using index file", "file", c.DBFile, "schema", schemaV)
	idx := &index.Indexer{Backend: db}

	// summary, err := idx.GetStoreSummary(ctx)
	// if err != nil {
	// 	return fmt.Errorf("getting summary info from index: %w", err)
	// }
	// c.Logger.Info("index summary",
	// 	"description", summary.Description,
	// 	"object_count", summary.NumObjects,
	// 	"ocfl spec", summary.Spec,
	// 	"last_indexed", summary.IndexedAt)
	service := index.Service{
		Index:     idx,
		Async:     index.NewAsync(ctx),
		FS:        fsys,
		RootPath:  rootDir,
		ScanConc:  c.ScanConc,
		ParseConc: c.ParseConc,
		Log:       c.Logger,
	}
	c.Logger.Info("starting http/grpc server", "port", c.Addr)
	if err := http.ListenAndServe(c.Addr, h2c.NewHandler(service.HTTPHandler(), &http2.Server{})); err != nil {
		return err
	}
	c.Logger.Info("http/grpc server stopped")
	return nil
}
