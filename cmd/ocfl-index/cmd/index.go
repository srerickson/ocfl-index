/*
Copyright © 2022
*/
package cmd

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	_ "gocloud.dev/blob/azureblob"
)

var indexFlags struct {
	conc int
}

// indexCmd represents the index command
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "index an OCFL storage root",
	Long: `The index command indexes all objects in a specified OCFL storage root. The
index file will be created if it does not exist.`,
	Run: func(cmd *cobra.Command, args []string) {
		logger := NewLogger()
		conf, err := NewConfig(logger)
		if err != nil {
			logger.Error(err, "configuration error")
			return
		}
		fsys, rootDir, err := conf.FS(cmd.Context())
		if err != nil {
			logger.Error(err, "can't connect to backend")
			return
		}
		if closer, ok := fsys.(io.Closer); ok {
			defer closer.Close()
		}
		if err := DoIndex(cmd.Context(), conf, fsys, rootDir); err != nil {
			logger.Error(err, "index failed")
		}
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().IntVar(
		&indexFlags.conc, "concurrency", 4, "number of concurrent operations duration indexing",
	)
}

func DoIndex(ctx context.Context, conf *config, fsys ocfl.FS, rootDir string) error {
	idx, err := sqlite.Open(conf.DBFile)
	if err != nil {
		return err
	}
	defer idx.Close()
	if _, err := idx.InitSchema(ctx); err != nil {
		return err
	}
	return index.NewIndex(
		idx, fsys, rootDir,
		index.WithConcurrency(conf.Conc),
		index.WithLogger(conf.Logger)).DoIndex(ctx, true)
}
