package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDoIndex(t *testing.T) {
	ctx := context.Background()
	t.Run("local storage root", func(t *testing.T) {
		testDir := filepath.Join(`..`, `..`, `..`, `testdata`, `simple-root`)
		conf := indexConfig{
			FSDir: testDir,
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
		conf := indexConfig{
			S3Bucket: "ocfl-test",
			S3Path:   "simple-root",
		}
		dbName := "test?mode=memory"
		err := DoIndex(ctx, dbName, &conf)
		if err != nil {
			t.Fatal(err)
		}
	})

}
