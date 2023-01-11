package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-logr/logr"
	"github.com/iand/logfmtr"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl/backend/cloud"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
)

const (
	envS3Endpoint = "AWS_S3_ENDPOINT"
	envDriver     = "OCFL_INDEX_BACKEND" // "fs" (default), "s3", or "azure"
	envBucket     = "OCFL_INDEX_BUCKET"  // cloud bucket for s3 or azure backend ("" default)
	envPath       = "OCFL_INDEX_STOREDIR"
	envDBFile     = "OCFL_INDEX_SQLITE"
	envAddr       = "OCFL_INDEX_LISTEN"
	envConc       = "OCFL_INDEX_MAXGOROUTINES"
)

type config struct {
	Logger logr.Logger

	// Backend configuration
	Driver     string // backend driver (supported: "fs", "s3", "azure")
	Bucket     string // Bucket/Container for s3 of azure fs types
	Path       string // Path to storage root (default: ".")
	S3Endpoint string // custom s3 endpoint

	// SQLITE file
	DBFile string // sqlite file

	// Server
	Addr string // port

	// Concurrency
	Conc int
}

func NewLogger() logr.Logger {
	logger := logfmtr.NewWithOptions(logfmtr.Options{
		Writer:    os.Stderr,
		Humanize:  true,
		NameDelim: "/",
	})
	logfmtr.SetVerbosity(verbosity)
	logger.Info("ocfl-index", "version", index.Version, "verbosity", verbosity)
	return logger
}

func NewConfig(logger logr.Logger) (*config, error) {
	c := &config{
		Logger: logger,
	}
	// values from environment variables
	c.Bucket = getenvDefault(envBucket, "")
	c.Driver = getenvDefault(envDriver, "fs")
	c.Path = getenvDefault(envPath, ".")
	c.S3Endpoint = getenvDefault(envS3Endpoint, "")
	c.DBFile = getenvDefault(envDBFile, "index.sqlite")
	c.Addr = getenvDefault(envAddr, ":8080")
	conc, err := strconv.Atoi(getenvDefault(envConc, "0"))
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", envConc, err)
	}
	// concurrency
	c.Conc = conc
	if c.Conc < 1 {
		c.Conc = runtime.GOMAXPROCS(-1)
	}
	return c, nil
}

func (c *config) FS(ctx context.Context) (ocfl.FS, string, error) {
	switch c.Driver {
	case "fs":
		return ocfl.NewFS(os.DirFS(c.Path)), ".", nil
	case "s3":
		sess, err := session.NewSession()
		if err != nil {
			return nil, "", fmt.Errorf("configuring s3: %w", err)
		}
		sess.Config.S3ForcePathStyle = aws.Bool(true)
		if c.S3Endpoint != "" {
			sess.Config.Endpoint = aws.String(c.S3Endpoint)
		}
		bucket, err := s3blob.OpenBucket(ctx, sess, c.Bucket, nil)
		if err != nil {
			return nil, "", fmt.Errorf("opening s3 bucket: %w", err)
		}
		fsys := cloud.NewFS(bucket, cloud.WithLogger(c.Logger))
		return fsys, c.Path, nil
	case "azure":
		bucket, err := blob.OpenBucket(ctx, "azblob://"+c.Bucket)
		if err != nil {
			return nil, "", fmt.Errorf("configuring azure: %w", err)
		}
		fsys := cloud.NewFS(bucket, cloud.WithLogger(c.Logger))
		return fsys, c.Path, nil
	default:
		return nil, "", fmt.Errorf("unsupported storage driver %s", c.Driver)
	}
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
