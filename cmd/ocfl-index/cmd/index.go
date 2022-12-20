/*
Copyright © 2022
*/
package cmd

import (
	"context"
	"log"

	"github.com/go-logr/stdr"
	"github.com/muesli/coral"
	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	_ "gocloud.dev/blob/azureblob"
)

type indexConfig struct {
	conc int
}

var indexFlags indexConfig

// indexCmd represents the index command
var indexCmd = &coral.Command{
	Use:   "index",
	Short: "index an OCFL storage root",
	Long: `The index command indexes all objects in a specified OCFL storage root. The
index file will be created if it does not exist.`,
	Run: func(cmd *coral.Command, args []string) {
		log.Printf("ocfl-index %s", index.Version)
		if err := setupFS(cmd.Context(), &fsFlags); err != nil {
			log.Fatal(err)
		}
		if fsFlags.closer != nil {
			defer fsFlags.closer.Close()
		}
		err := DoIndex(cmd.Context(), fsFlags.fs, fsFlags.rootDir, dbFlag, indexFlags.conc)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().IntVar(
		&indexFlags.conc, "concurrency", 4, "number of concurrent operations duration indexing",
	)
}

func DoIndex(ctx context.Context, fsys ocfl.FS, root string, dbName string, conc int) error {
	idx, err := sqlite.Open(dbName)
	if err != nil {
		return err
	}
	defer idx.Close()
	if _, err := idx.InitSchema(ctx); err != nil {
		return err
	}
	log := stdr.New(nil)
	return index.NewIndex(
		idx, fsFlags.fs, fsFlags.rootDir,
		index.WithConcurrency(conc),
		index.WithLogger(log)).DoIndex(ctx)
}
