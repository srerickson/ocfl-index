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
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/muesli/coral"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl/backend/s3fs"
	"github.com/srerickson/ocfl/ocflv1"
	"github.com/srerickson/ocfl/spec"
)

const (
	// envS3AccessKey = "AWS_ACCESS_KEY_ID"
	// envS3Secret    = "AWS_SECRET_ACCESS_KEY"
	// envS3Region    = "AWS_REGION"
	envS3Endpoint = "AWS_S3_ENDPOINT"
)

type indexConfig struct {
	FSDir       string
	S3Bucket    string
	S3Path      string
	S3Endpoint  string
	Concurrency int
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
	Long: `The index command indexes all objects in a specified OCFL storage root.
	The index file will be created if it does not exist.`,
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
	indexCmd.Flags().StringVar(
		&indexFlags.S3Bucket, "s3-bucket", "", "s3 bucket for storage root",
	)
	indexCmd.Flags().StringVar(
		&indexFlags.S3Path, "s3-path", "", "s3 path for storage root",
	)
	indexCmd.Flags().IntVar(
		&indexFlags.Concurrency, "concurrency", 4, "number of concurrent operations duration indexing",
	)
}

func DoIndex(ctx context.Context, dbName string, c *indexConfig) error {
	log.Printf("ocfl-index %s", index.Version)
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
	log.Printf("indexing to %s, ocfl-index schema: v%d.%d\n", dbName, major, minor)
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

// concurrent indexing for objects paths in store
func indexStore(ctx context.Context, idx index.Interface, store *ocflv1.Store, paths map[string]spec.Num, workers int) error {
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
