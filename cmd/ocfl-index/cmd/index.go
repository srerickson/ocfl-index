/*
Copyright © 2022

*/
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/muesli/coral"
	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl/backend/cloud"
	"github.com/srerickson/ocfl/ocflv1"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob"
	"gocloud.dev/blob/s3blob"
)

const (
	// envS3AccessKey = "AWS_ACCESS_KEY_ID"
	// envS3Secret    = "AWS_SECRET_ACCESS_KEY"
	// envS3Region    = "AWS_REGION"
	envS3Endpoint = "AWS_S3_ENDPOINT"
)

type indexConfig struct {
	// Backend configuration
	Driver     string // backend driver (supported: "fs", "s3", "azure")
	Bucket     string // Bucket/Container for s3 of azure fs types
	Path       string // Path to storage root (default: ".")
	S3Endpoint string // custom s3 endpoint

	Concurrency int

	// rest is set during setupFS
	fs      ocfl.FS
	rootDir string
	closer  io.Closer
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
		err := DoIndex(cmd.Context(), dbName, &indexFlags)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(indexCmd)
	indexCmd.Flags().StringVar(
		&indexFlags.Driver, "driver", "fs", "backend driver for accessing storage root ('fs', 's3', or 'azure')",
	)
	indexCmd.Flags().StringVar(
		&indexFlags.Bucket, "bucket", "", "bucket or container for storage root on S3 or Azure",
	)
	indexCmd.Flags().StringVar(
		&indexFlags.Path, "path", ".", "path for storage root (relative to driver settings)",
	)
	indexCmd.Flags().IntVar(
		&indexFlags.Concurrency, "concurrency", 4, "number of concurrent operations duration indexing",
	)
}

func DoIndex(ctx context.Context, dbName string, c *indexConfig) error {
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
	log.Printf("indexing to %s, ocfl-index schema: v%d.%d\n", dbName, major, minor)
	if err := setupFS(ctx, c); err != nil {
		return err
	}
	if c.closer != nil {
		defer c.closer.Close()
	}
	store, err := ocflv1.GetStore(ctx, c.fs, c.rootDir)
	if err != nil {
		return fmt.Errorf("reading storage root: %w", err)
	}
	log.Printf("starting object scan (concurrency=%d)", c.Concurrency)
	startScan := time.Now()
	objPaths, err := store.ScanObjects(ctx, &ocflv1.ScanObjectsOpts{
		Strict:      false,
		Concurrency: c.Concurrency,
	})
	if err != nil {
		return err
	}
	total := len(objPaths)
	startIndexing := time.Now()
	log.Printf("scan finished in %.2f sec., indexing %d objects ...", time.Since(startScan).Seconds(), total)
	err = indexStore(ctx, idx, store, objPaths, c.Concurrency)
	if err != nil {
		return err
	}
	log.Printf("indexing finished in %.2f sec. (total time %.2f sec.)", time.Since(startIndexing).Seconds(), time.Since(startScan).Seconds())
	return nil
}

func setupFS(ctx context.Context, c *indexConfig) error {
	switch c.Driver {
	case "fs":
		log.Printf("using FS dir=%s\n", c.Path)
		c.fs = ocfl.NewFS(os.DirFS(c.Path))
		c.rootDir = "."
	case "s3":
		log.Printf("using S3 bucket=%s path=%s\n", c.Bucket, c.Path)
		sess, err := session.NewSession()
		if err != nil {
			return err
		}
		sess.Config.S3ForcePathStyle = aws.Bool(true)
		c.S3Endpoint = getenvDefault(envS3Endpoint, "")
		if c.S3Endpoint != "" {
			sess.Config.Endpoint = aws.String(c.S3Endpoint)
		}
		bucket, err := s3blob.OpenBucket(ctx, sess, c.Bucket, nil)
		if err != nil {
			return err
		}
		c.fs = cloud.NewFS(bucket)
		c.closer = bucket
		c.rootDir = c.Path
	case "azure":
		log.Printf("using Azure container=%s path=%s\n", c.Bucket, c.Path)
		bucket, err := blob.OpenBucket(ctx, "azblob://"+c.Bucket)
		if err != nil {
			return err
		}
		c.fs = cloud.NewFS(bucket)
		c.closer = bucket
		c.rootDir = c.Path
	default:
		return fmt.Errorf("unsupported storage driver %s", c.Driver)
	}
	return nil
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// concurrent indexing for objects paths in store
func indexStore(ctx context.Context, idx index.Interface, store *ocflv1.Store, paths map[string]ocfl.Spec, workers int) error {
	type job struct {
		path string
		inv  *ocflv1.Inventory
		err  error
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)
	in := make(chan (*job))
	go func() {
		defer close(in)
	L:
		for p := range paths {
			select {
			case in <- &job{path: p}:
			case <-ctx.Done():
				break L
			}
		}
	}()
	out := make(chan (*job))
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := range in {
				obj, err := store.GetPath(ctx, j.path)
				if err != nil {
					j.err = err
					out <- j
					continue
				}
				j.inv, err = obj.Inventory(ctx)
				if err != nil {
					j.err = err
					out <- j
					continue
				}
				out <- j
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	var returnErr error
	var i int
	total := len(paths)
	for j := range out {
		i++
		if j.err != nil {
			returnErr = j.err
			break
		}
		err := idx.IndexInventory(ctx, j.inv)
		if err != nil {
			returnErr = j.err
			break
		}
		fmt.Printf("\r%d/%d\r", i, total)
	}
	cancel()
	return returnErr
}
