package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/srerickson/ocfl-index/internal/ocfltest"
	"github.com/srerickson/ocfl/backend/cloud"
	"github.com/srerickson/ocfl/extensions"
	"gocloud.dev/blob/s3blob"
)

type config struct {
	S3Bucket   string
	S3Path     string
	S3Endpoint string
	numObjects int
	numFiles   int
	numVers    int
}

func main() {
	var c config
	flag.StringVar(&c.S3Bucket, "bucket", "", "s3 bucket")
	flag.StringVar(&c.S3Path, "path", ".", "s3 path")
	flag.IntVar(&c.numObjects, "objects", 100, "number of objects/inventories in the generated storage root")
	flag.IntVar(&c.numFiles, "files", 10, "number of files in each generated object/inventory")
	flag.IntVar(&c.numVers, "versions", 2, "max number of versions in each generated inventory")
	flag.Parse()
	c.S3Endpoint = envDefault("AWS_S3_ENDPOINT", "")
	if err := doGenStore(&c); err != nil {
		log.Fatal(err)
	}
}

func doGenStore(c *config) error {
	ctx := context.Background()
	log.Printf("using S3 bucket=%s path=%s\n", c.S3Bucket, c.S3Path)
	sess, err := session.NewSession()
	if err != nil {
		return err
	}
	sess.Config.S3ForcePathStyle = aws.Bool(true)
	if c.S3Endpoint != "" {
		sess.Config.Endpoint = aws.String(c.S3Endpoint)
	}
	bucket, err := s3blob.OpenBucket(ctx, sess, c.S3Bucket, nil)
	if err != nil {
		return err
	}
	fsys := cloud.NewFS(bucket)

	conf := ocfltest.GenStoreConf{
		InvNumber:    c.numObjects,
		InvSize:      c.numFiles,
		VNumMax:      c.numVers,
		LayoutConfig: extensions.NewLayoutHashIDTuple(),
	}
	return ocfltest.GenStore(fsys, c.S3Path, &conf)

}

func envDefault(env, def string) string {
	if os.Getenv(env) == "" {
		return def
	}
	return os.Getenv(env)
}
