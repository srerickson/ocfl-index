/*
Copyright © 2022

*/
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/muesli/coral"
	"github.com/srerickson/ocfl/backend/s3fs"
	"github.com/srerickson/ocfl/ocflv1"
)

const (
	// envS3AccessKey = "AWS_ACCESS_KEY_ID"
	// envS3Secret    = "AWS_SECRET_ACCESS_KEY"
	// envS3Region    = "AWS_REGION"
	envS3Endpoint = "AWS_S3_ENDPOINT"
)

type indexConfig struct {
	FSDir      string
	S3Bucket   string
	S3Path     string
	S3Endpoint string
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
	indexCmd.PersistentFlags().StringVar(
		&indexFlags.S3Bucket, "s3-bucket", "", "s3 bucket for storage root",
	)
	indexCmd.PersistentFlags().StringVar(
		&indexFlags.S3Path, "s3-path", "", "s3 path for storage root",
	)
}

func DoIndex(ctx context.Context, dbName string, c *indexConfig) error {
	// load env variables
	c.S3Endpoint = getenvDefault(envS3Endpoint, "")

	db, err := sql.Open("sqlite", "file:"+dbName)
	if err != nil {
		return err
	}
	defer db.Close()
	idx, err := prepareIndex(ctx, db)
	if err != nil {
		return err
	}
	major, minor, err := idx.GetSchemaVersion(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("indexing to %s, schema: v%d.%d\n", dbName, major, minor)
	if err := setupFS(c); err != nil {
		return err
	}
	if c.closer != nil {
		defer c.closer.Close()
	}
	store, err := ocflv1.GetStore(ctx, c.fs, c.rootDir)
	if err != nil {
		return fmt.Errorf("reading storage root: %w", err)
	}
	fmt.Print("scanning for objects...")
	objPaths, err := store.ScanObjects(ctx)
	if err != nil {
		fmt.Println("")
		return err
	}
	total := len(objPaths)
	fmt.Println("found", total)
	i := 0
	for objPath := range objPaths {
		obj, err := store.GetPath(ctx, objPath)
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
	if c.S3Bucket != "" && c.S3Path != "" {
		log.Printf("using S3 bucket=%s path=%s\n", c.S3Bucket, c.S3Path)
		sess, err := session.NewSession()
		if err != nil {
			return err
		}
		sess.Config.S3ForcePathStyle = aws.Bool(true)
		if c.S3Endpoint != "" {
			sess.Config.Endpoint = aws.String(c.S3Endpoint)
		}
		c.fs = s3fs.New(s3.New(sess), c.S3Bucket)
		c.rootDir = c.S3Path
	} else {
		log.Printf("using FS dir=%s\n", c.FSDir)
		c.fs = os.DirFS(c.FSDir)
		c.rootDir = "."
	}
	return nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
