/*
Copyright © 2022

*/
package cmd

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/muesli/coral"
	"github.com/srerickson/ocfl/backend/s3fs"
	"github.com/srerickson/ocfl/ocflv1"
)

type indexConfig struct {
	FSDir    string
	ZipPath  string
	S3Bucket string
	S3Path   string
	// rest is set during setupFS
	fs      fs.FS
	rootDir string
	closer  io.Closer
}

var indexFlags indexConfig

// indexCmd represents the index command
var indexCmd = &coral.Command{
	Use:   "index",
	Short: "index an OCFL storage root",
	Long: `The index command indexes all objects in an OCFL storage root to the
	logical path level. The index is written to a sqlite3 database specified
	with the output option.`,
	Run: func(cmd *coral.Command, args []string) {
		err := DoIndex(cmd.Context(), dbName, &indexFlags)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().StringVarP(
		&indexFlags.FSDir, "dir", "d", ".", "path to storage root directory",
	)
	indexCmd.PersistentFlags().StringVarP(
		&indexFlags.ZipPath, "zip", "z", "", "path to storage root zip file",
	)
	indexCmd.PersistentFlags().StringVar(
		&indexFlags.S3Bucket, "s3-bucket", "", "s3 bucket for storage root",
	)
	indexCmd.PersistentFlags().StringVar(
		&indexFlags.S3Path, "s3-path", "", "s3 path for storage root",
	)
}

func DoIndex(ctx context.Context, dbName string, c *indexConfig) error {
	idx, err := openIndex(ctx, dbName)
	if err != nil {
		return err
	}
	defer idx.Close()
	major, minor, err := idx.GetSchemaVersion(ctx)
	if err != nil {
		return err
	}
	log.Printf("indexing to %s, schema: v%d.%d", dbName, major, minor)
	if err := setupFS(c); err != nil {
		return err
	}
	if c.closer != nil {
		defer c.closer.Close()
	}
	store, err := ocflv1.GetStore(ctx, c.fs, c.rootDir)
	if err != nil {
		return err
	}
	objPaths, err := store.ScanObjects(ctx)
	if err != nil {
		return err
	}
	total := len(objPaths)
	i := 0
	for objPath := range objPaths {
		obj, err := ocflv1.GetObject(ctx, c.fs, objPath)
		if err != nil {
			return err
		}
		inv, err := obj.Inventory(ctx)
		if err != nil {
			return err
		}
		err = idx.IndexInventory(ctx, inv)
		if err != nil {
			return err
		}
		i++
		fmt.Printf("\rindexed %d/%d objects", i, total)
	}
	fmt.Println("\ndone")
	return nil
}

func setupFS(c *indexConfig) error {
	if c.ZipPath != "" {
		zipFS, err := zip.OpenReader(c.ZipPath)
		if err != nil {
			return err
		}
		c.fs = zipFS
		c.rootDir = "."
		c.closer = zipFS
	} else if c.S3Bucket != "" && c.S3Path != "" {
		sess, err := session.NewSession()
		if err != nil {
			return err
		}
		c.fs = s3fs.New(s3.New(sess), c.S3Bucket)
		c.rootDir = c.S3Path
	} else {
		c.fs = os.DirFS(c.FSDir)
		c.rootDir = "."
	}
	return nil
}
