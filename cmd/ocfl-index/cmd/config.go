package cmd

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl/backend/cloud"
	"gocloud.dev/blob"
	"gocloud.dev/blob/s3blob"
	"golang.org/x/exp/slog"
)

const (
	envS3Endpoint = "AWS_S3_ENDPOINT"
	envDriver     = "OCFL_INDEX_BACKEND" // "fs" (default), "s3", or "azure"
	envBucket     = "OCFL_INDEX_BUCKET"  // cloud bucket for s3 or azure backend ("" default)
	envPath       = "OCFL_INDEX_STOREDIR"
	envDBFile     = "OCFL_INDEX_SQLITE"
	envAddr       = "OCFL_INDEX_LISTEN"
	envScanConc   = "OCFL_INDEX_SCANWORKERS"  // number of workers for object scan
	envParseConc  = "OCFL_INDEX_PARSEWORKERS" // numer of workers for parsing inventories

	sqliteSettings = "_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared"
)

type config struct {
	Logger *slog.Logger

	// Server
	Addr string // port

	// Backend configuration
	Driver     string // backend driver (supported: "fs", "s3", "azure")
	Bucket     string // Bucket/Container for s3 of azure fs types
	Path       string // Path to storage root (default: ".")
	S3Endpoint string // custom s3 endpoint

	// SQLITE file
	DBFile string // sqlite file

	// Concurrency Settings
	ScanConc  int // number of object scanning workers
	ParseConc int // number of inventory parsing workers
}

func NewLogger() *slog.Logger {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{}))
	logger.Info("ocfl-index", "version", index.Version, "verbosity", verbosity)
	return logger
}

func NewConfig(logger *slog.Logger) config {
	c := config{
		Logger: logger,
	}
	c.Bucket = getenvDefault(envBucket, "")
	c.Driver = getenvDefault(envDriver, "fs")
	c.Path = getenvDefault(envPath, ".")
	c.S3Endpoint = getenvDefault(envS3Endpoint, "")
	c.DBFile = getenvDefault(envDBFile, "index.sqlite")
	c.Addr = getenvDefault(envAddr, ":8080")
	if conc, err := strconv.Atoi(getenvDefault(envScanConc, "0")); err == nil {
		c.ScanConc = conc
	}
	if c.ScanConc < 1 {
		c.ScanConc = runtime.NumCPU()
	}
	if conc, err := strconv.Atoi(getenvDefault(envParseConc, "0")); err == nil {
		c.ParseConc = conc
	}
	if c.ParseConc < 1 {
		c.ParseConc = runtime.NumCPU()
	}
	logger.Info("config loaded", c.Attrs()...)
	return c
}

func (c config) Attrs() []any {
	attrs := []any{
		"addr", c.Addr,
		"driver", c.Driver,
		"bucket", c.Bucket,
		"path", c.Path,
		"dbfile", c.DBFile,
		"scan_workers", c.ScanConc,
		"parse_workers", c.ParseConc,
	}
	if c.S3Endpoint != "" {
		attrs = append(attrs, "s3_endpoint", c.S3Endpoint)
	}
	return attrs
}

func (c config) FS(ctx context.Context) (ocfl.FS, string, error) {
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
