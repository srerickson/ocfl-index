package index_test

import (
	"context"
	"path/filepath"

	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	"github.com/srerickson/ocfl/backend/cloud"
	"gocloud.dev/blob/fileblob"
	_ "modernc.org/sqlite"
)

var fixtureRoot = filepath.Join("..", "..", "testdata")

func newTestIndex(ctx context.Context, fixture string, opts ...index.Option) (*index.Index, error) {
	buck, err := fileblob.OpenBucket(fixtureRoot, nil)
	if err != nil {
		return nil, err
	}
	fsys := cloud.NewFS(buck)
	db, err := sqlite.Open("file:tmp.sqlite?mode=memory&_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared")
	if err != nil {
		return nil, err
	}
	if _, err := db.InitSchema(ctx); err != nil {
		return nil, err
	}
	return index.NewIndex(db, fsys, fixture, opts...), nil
}
