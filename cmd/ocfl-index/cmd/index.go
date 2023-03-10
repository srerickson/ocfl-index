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

// indexCmd represents the index command
var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "index an OCFL storage root",
	Long: `The index command indexes all objects in a specified OCFL storage root. The
index file will be created if it does not exist.`,
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
		if err := DoIndex(ctx, &conf, fsys, rootDir); err != nil {
			logger.Error(err, "index failed")
		}
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

func DoIndex(ctx context.Context, conf *config, fsys ocfl.FS, rootDir string) error {
	db, err := sqlite.Open("file:" + conf.DBFile + "?" + sqliteSettings)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.InitSchema(ctx); err != nil {
		return err
	}
	idx := &index.Index{Backend: db}
	opts := &index.ReindexOptions{
		FS:        fsys,
		RootPath:  rootDir,
		ScanConc:  conf.ScanConc,
		ParseConc: conf.ParseConc,
		Log:       conf.Logger,
	}
	return idx.Reindex(ctx, opts)
}
