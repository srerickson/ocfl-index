package cmd

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/muesli/coral"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/sqlite"
)

var dbName string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &coral.Command{
	Use:   "ocfl-index",
	Short: "Index and query OCFL Storage Roots",
	CompletionOptions: coral.CompletionOptions{
		DisableDefaultCmd: true,
	},
	Long: ``,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(
		&dbName, "file", "f", "index.sqlite", "index filename/connection string",
	)
}

func openIndex(ctx context.Context, name string) (index.Interface, error) {
	var existingIndexFile bool
	inf, err := os.Stat(name)
	if err == nil && inf.Mode().IsRegular() {
		existingIndexFile = true
	}
	db, err := sql.Open("sqlite3", "file:"+name)
	if err != nil {
		return nil, err
	}
	idx := sqlite.New(db)
	if !existingIndexFile {
		created, err := idx.MigrateSchema(ctx, false)
		if err != nil {
			return nil, err
		}
		if created {
			log.Println("created new index tables in", name)
		}
	}
	return idx, err
}
