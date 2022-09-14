package cmd

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/srerickson/ocfl"
)

var testDir = filepath.Join(`..`, `..`, `..`, `testdata`, `simple-root`)

func createS3Root(ctx context.Context, conf indexConfig) error {
	// create bucket with test data
	if err := setupFS(ctx, &conf); err != nil {
		return err
	}
	if conf.closer != nil {
		defer conf.closer.Close()
	}
	writefs, ok := conf.fs.(ocfl.WriteFS)
	if !ok {
		return errors.New("failed to set up test storage root")
	}
	srcFS := os.DirFS(testDir)
	return fs.WalkDir(srcFS, ".", func(name string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.Type().IsRegular() {
			return nil
		}
		f, err := srcFS.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = writefs.Write(ctx, name, f)
		return err
	})
}

func TestDoIndex(t *testing.T) {
	ctx := context.Background()
	t.Run("local storage root", func(t *testing.T) {
		conf := indexConfig{
			Driver: "fs",
			Path:   testDir,
		}
		dbName := "test?mode=memory"
		err := DoIndex(ctx, dbName, &conf)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("s3 storage root", func(t *testing.T) {
		// test index on s3
		env := map[string]string{
			"AWS_ACCESS_KEY_ID":     "minioadmin",
			"AWS_SECRET_ACCESS_KEY": "minioadmin",
			"AWS_REGION":            "us-west",
			"AWS_S3_ENDPOINT":       "http://localhost:9000",
		}
		for k, v := range env {
			os.Setenv(k, v)
		}
		defer func() {
			for k := range env {
				os.Unsetenv(k)
			}
		}()
		ctx := context.Background()
		conf := indexConfig{
			Driver: "s3",
			Bucket: "simple-root",
			Path:   ".",
		}
		dbName := "test?mode=memory"
		if err := createS3Root(ctx, conf); err != nil {
			t.Fatal(err)
		}
		if err := DoIndex(ctx, dbName, &conf); err != nil {
			t.Fatal(err)
		}
	})

}
