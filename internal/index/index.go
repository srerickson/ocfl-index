package index

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/go-logr/logr"
	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
)

// set during with build with
// -ldflags -X 'github.com/srerickson/ocfl-index/internal/index.Version=v0.0.X'
var Version = "devel"

var ErrNotFound = errors.New("not found")
var ErrMissingValue = errors.New("missing value")

// Index provides indexing for an OCFL Storage Root
type Index struct {
	Backend            // index database
	ocfl.FS            // storage root fs
	root        string // storage root directory
	concurrency int
	log         logr.Logger
}

// NewIndex returns a new Index for OCFL storage root at root in fsys. An indexing
// backend implementation (currently, sqlite) is also required.
func NewIndex(db Backend, fsys ocfl.FS, root string, opts ...Option) *Index {
	idx := &Index{
		Backend:     db,
		FS:          fsys,
		root:        root,
		concurrency: runtime.GOMAXPROCS(-1),
		log:         logr.Discard(),
	}
	for _, o := range opts {
		o(idx)
	}
	return idx
}

// Option is used by NewIndex to configure the Index
type Option func(*Index)

func WithConcurrency(c int) Option {
	return func(opt *Index) {
		opt.concurrency = c
	}
}

func WithLogger(l logr.Logger) Option {
	return func(opt *Index) {
		opt.log = l
	}
}

// DoIndex() indexes the storage root associated with the index.
func (idx Index) DoIndex(ctx context.Context, withSize bool) error {
	store, err := ocflv1.GetStore(ctx, idx.FS, idx.root)
	if err != nil {
		return err
	}
	// store the storage root's info in the database
	if err := idx.SetStoreInfo(ctx, idx.root, store.Description(), store.Spec()); err != nil {
		return err
	}
	idx.log.Info("indexing storage root...", "path", idx.root, "concurrency", idx.concurrency, "withFileSize", withSize)
	numObjs := 0
	scanFn := func(obj *ocflv1.Object) error {
		numObjs++
		return idx.indexObject(ctx, obj, withSize)
	}
	if err := store.ScanObjects(ctx, scanFn, &ocflv1.ScanObjectsOpts{
		Strict:      false,
		Concurrency: idx.concurrency,
	}); err != nil {
		return fmt.Errorf("indexing storage root: %w", err)
	}
	idx.SetStoreIndexedAt(ctx)
	idx.log.Info("indexing complete", "path", idx.root)
	return nil
}

// TODO: return a boolean indicating if any changes were made
func (idx Index) indexObject(ctx context.Context, obj *ocflv1.Object, withSize bool) error {
	// TODO: avoid repeated scans of content files. We don't want to scan
	// directories unnecessarily because it's slow (over S3).
	//
	// Some things to consider: Files can have zero size which means a version's
	// size shouldn't be used to indicate if its content was previously scanned.
	// A another challenge is that content files in one version can be used in
	// later versions. If we only scan the content files for the later version,
	// and the version state refers to files from a previous version, we will
	// have partial size information and the recursive directory digests will be
	// wrong. It seems pretty complicated to merge previously indexed size
	// information into the indexing object state. Is there any other way?
	//
	// Until this is figured out, take the brute-force approach and just
	// rescanning everything on each run.
	_, p := obj.Root()
	idxObj, err := idx.GetObjectByPath(ctx, p)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	if idxObj != nil {
		sidecar, err := obj.InventorySidecar(ctx)
		if err != nil {
			return err
		}
		if sidecar == idxObj.InventoryDigest {
			idx.log.Info("object unchanged", "path", p)
			return nil
		}
	}
	idx.log.Info("indexing object", "path", p)
	tree, err := NewIndexingObject(ctx, obj, withSize)
	if err != nil {
		return err
	}
	return idx.IndexObject(ctx, tree)
}
