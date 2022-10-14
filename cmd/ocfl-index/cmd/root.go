package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"

	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
	_ "modernc.org/sqlite"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/muesli/coral"
	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/sqlite"
	"github.com/srerickson/ocfl/backend/cloud"
)

const envS3Endpoint = "AWS_S3_ENDPOINT"

var dbFlag string
var fsFlags fsConfig

type fsConfig struct {
	// Backend configuration
	Driver     string // backend driver (supported: "fs", "s3", "azure")
	Bucket     string // Bucket/Container for s3 of azure fs types
	Path       string // Path to storage root (default: ".")
	S3Endpoint string // custom s3 endpoint
	// rest is set during setupFS
	fs      ocfl.FS
	rootDir string
	closer  io.Closer
}

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
		&dbFlag, "file", "f", "index.sqlite", "index filename/connection string",
	)
	rootCmd.PersistentFlags().StringVar(
		&fsFlags.Driver, "driver", "fs", "backend driver for accessing storage root ('fs', 's3', or 'azure')",
	)
	rootCmd.PersistentFlags().StringVar(
		&fsFlags.Bucket, "bucket", "", "bucket or container for storage root on S3 or Azure",
	)
	rootCmd.PersistentFlags().StringVar(
		&fsFlags.Path, "path", ".", "path for storage root (relative to driver settings)",
	)
}

func prepareIndex(ctx context.Context, db *sql.DB) (index.Interface, error) {
	idx := sqlite.New(db)
	created, err := idx.MigrateSchema(ctx, false)
	if err != nil {
		return nil, err
	}
	if created {
		log.Println("created new index tables")
	}
	return idx, err
}

func setupFS(ctx context.Context, c *fsConfig) error {
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
