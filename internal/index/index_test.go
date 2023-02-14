package index_test

import (
	"context"
	"path/filepath"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	_ "modernc.org/sqlite"
)

var fixtureRoot = filepath.Join("..", "..", "testdata")

func newTestIndex(ctx context.Context, fixture string, opts ...index.Option) (*index.Index, error) {
	fsys := ocfl.DirFS(fixtureRoot)
	db, err := sqlite.Open("file:tmp.sqlite?mode=memory&cache=shared")
	if err != nil {
		return nil, err
	}
	if _, err := db.InitSchema(ctx); err != nil {
		return nil, err
	}
	return index.NewIndex(db, fsys, fixture, opts...), nil
}
