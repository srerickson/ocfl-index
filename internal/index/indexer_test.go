package index_test

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-logr/logr"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	"github.com/srerickson/ocfl/backend/cloud"
	"gocloud.dev/blob/fileblob"
	_ "modernc.org/sqlite"
)

var fixtureRoot = filepath.Join("..", "..", "testdata")

func newTestIndex(ctx context.Context, dbname string) (*index.Indexer, error) {
	conn := fmt.Sprintf("file:%s?mode=memory&_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared", dbname)
	db, err := sqlite.Open(conn)
	if err != nil {
		return nil, err
	}
	if _, err := db.InitSchema(ctx); err != nil {
		return nil, err
	}
	return &index.Indexer{
		Backend: db,
	}, nil
}

// return a new httptest.Server and a client for connecting to it, all ready to go.
func newTestService(ctx context.Context, fixture string) (*index.Service, error) {
	buck, err := fileblob.OpenBucket(fixtureRoot, nil)
	if err != nil {
		return nil, err
	}
	fsys := cloud.NewFS(buck)

	idx, err := newTestIndex(ctx, fixture)
	if err != nil {
		return nil, fmt.Errorf("initializing fixture index: %w", err)
	}
	srv := &index.Service{
		Index:    idx,
		FS:       fsys,
		RootPath: fixture,
		Log:      logr.Discard(),
		Async:    index.NewAsync(ctx),
	}
	opts := &index.IndexOptions{
		FS:       fsys,
		RootPath: fixture,
		Log:      logr.Discard(),
	}
	if err := srv.Index.Index(ctx, opts); err != nil {
		return nil, err
	}
	return srv, nil
}
