package sqlite_test

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/mock"
	"github.com/srerickson/ocfl-index/internal/sqlite"
	"github.com/srerickson/ocfl-index/internal/sqlite/sqlc"
	"github.com/srerickson/ocfl/ocflv1"
)

func TestInitSchema(t *testing.T) {
	expSchema := [2]int{0, 4}
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx, t.Name())
	expNil(t, err)
	defer idx.Close()
	major, minor, err := idx.GetSchemaVersion(ctx)
	expNil(t, err)
	if major != expSchema[0] || minor != expSchema[1] {
		t.Errorf("expected schema version %d.%d, got %d.%d", expSchema[0], expSchema[1], major, minor)
	}
}

func TestGetIndexSummary(t *testing.T) {
	ctx := context.Background()
	t.Run("single object", func(t *testing.T) {
		m := mock.NewIndexingObject(t.Name())
		idx, err := setupSqliteIndex(ctx, t.Name(), func(tx index.BackendTx) error {
			return tx.IndexObjectInventory(ctx, m.IndexedAt, index.ObjectInventory{
				Path:      m.RootDir,
				Inventory: m.Inventory,
			})
		})
		expNil(t, err)
		summary, err := idx.GetIndexSummary(ctx)
		expNil(t, err)
		exp := index.IndexSummary{
			NumInventories: 1,
			NumObjects:     1,
			UpdatedAt:      m.IndexedAt.UTC(),
		}
		expEq(t, "indexed summary", summary, exp)
	})

	t.Run("empty", func(t *testing.T) {
		idx, err := setupSqliteIndex(ctx, t.Name(), func(tx index.BackendTx) error {
			return nil
		})
		expNil(t, err)
		summary, err := idx.GetIndexSummary(ctx)
		expNil(t, err)
		exp := index.IndexSummary{
			NumInventories: 0,
			NumObjects:     0,
			UpdatedAt:      time.Time{},
		}
		expEq(t, "indexed summary", summary, exp)
	})

}

func TestIndexObject(t *testing.T) {
	// TODO: scenarios to test
	// - basic inventory: rows created
	// - repeat index of same inventory: no changes
	// - index inventory with head=v1, head=v2 (same no file size)
	// - index inventory (head=1), then index file sizes (v2)
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx, t.Name())
	expNil(t, err)
	tx, err := idx.NewTx(ctx)
	expNil(t, err)
	defer idx.Close()
	defer tx.Rollback()
	id := "test-object"
	// index successive versions of the same object
	mock1 := mock.NewIndexingObject(id)
	mock2 := mock.NewIndexingObject(id, mock.WithHead(ocfl.V(2)))
	mock2.IndexedAt = mock1.IndexedAt.AddDate(0, 0, 1)
	mock3 := mock.NewIndexingObject(id, mock.WithHead(ocfl.V(3)))
	mock3.IndexedAt = mock1.IndexedAt.AddDate(0, 0, 2)
	expNil(t, tx.IndexObjectRoot(ctx, mock1.IndexedAt, index.ObjectRoot{Path: mock1.RootDir}))
	expNil(t, tx.IndexObjectInventory(ctx, mock2.IndexedAt, index.ObjectInventory{
		Inventory: mock2.Inventory,
		Path:      mock2.RootDir,
	}))
	expNil(t, tx.IndexObjectInventory(ctx, mock3.IndexedAt, index.ObjectInventory{
		Inventory: mock3.Inventory,
		Path:      mock3.RootDir,
	}))
	expNil(t, tx.Commit())
	// object roots table
	idxRoots, err := idx.DEBUG_AllObjecRootss(ctx)
	expNil(t, err)
	expEq(t, "# object roots", len(idxRoots), 1)
	expEq(t, "indexed object root path", idxRoots[0].Path, mock3.RootDir)
	expEq(t, "indexed object root indexed_at", idxRoots[0].IndexedAt.Truncate(time.Second).UTC(), mock3.IndexedAt.Truncate(time.Second).UTC())
	// inventory table values
	idxInvs, err := idx.DEBUG_AllInventories(ctx)
	expNil(t, err)
	expEq(t, "# inventories", len(idxInvs), 1)
	versions, err := idx.DEBUG_AllVersions(ctx)
	expNil(t, err)
	compareInventory(t, idxInvs[0], versions, mock3.Inventory)
}

func compareInventory(t *testing.T, idxInv sqlc.OcflIndexInventory, idxVers []sqlc.OcflIndexVersion, inv *ocflv1.Inventory) {
	expEq(t, "indexed inventory ID", idxInv.OcflID, inv.ID)
	expEq(t, "indexed inventory Head", idxInv.Head, inv.Head.String())
	expEq(t, "indexed digest alg", idxInv.DigestAlgorithm, inv.DigestAlgorithm)
	expEq(t, "indexed inventory digest", idxInv.InventoryDigest, inv.Digest())
	expEq(t, "indexed inventory spec", idxInv.Spec, inv.Type.Spec.String())
	expEq(t, "# indexed inventory versions", len(idxVers), len(inv.Versions))
	for _, idxver := range idxVers {
		vnum := ocfl.MustParseVNum(idxver.Name)
		ver := inv.Versions[vnum]
		expNotNil(t, "version "+vnum.String(), ver)
		expEq(t, "indexed version created", idxver.Created.Truncate(time.Second).UTC(), ver.Created.Truncate(time.Second).UTC())
		expEq(t, "indexed version message", idxver.Message, ver.Message)
		expEq(t, "indexed version num", idxver.Name, vnum.String())
		if ver.User != nil {
			expEq(t, "indexed version user addr", idxver.UserAddress, ver.User.Address)
			expEq(t, "indexed version user name", idxver.UserName, ver.User.Name)
		}
	}
}

func TestListObjectRoot(t *testing.T) {
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx, t.Name())
	expNil(t, err)
	defer idx.Close()
	tx, err := idx.NewTx(ctx)
	expNil(t, err)
	defer tx.Rollback()
	numObjects := 721
	for i := 0; i < numObjects; i++ {
		id := fmt.Sprintf("test-listroots-%d", i)
		// index successive versions of the same object
		mock := mock.NewIndexingObject(id)
		root := index.ObjectRoot{Path: mock.RootDir}
		expNil(t, tx.IndexObjectRoot(ctx, mock.IndexedAt, root))
	}
	expNil(t, tx.Commit())
	found := 0
	cursor := ""
	for {
		items, err := idx.ListObjectRoots(ctx, 17, cursor)
		if err != nil {
			t.Fatal(err)
		}
		found += len(items.ObjectRoots)
		if items.NextCursor == "" {
			break
		}
		cursor = items.NextCursor
	}
	expEq(t, "indexed object roots", found, numObjects)
}

func TestRemoveObjectsBefore(t *testing.T) {
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx, t.Name())
	expNil(t, err)
	defer idx.Close()
	tx, err := idx.NewTx(ctx)
	expNil(t, err)
	defer tx.Rollback()
	numObjects := 721
	indexedAt := time.Now().Add(-7 * time.Hour) // seven hours ago
	for i := 0; i < numObjects; i++ {
		id := fmt.Sprintf("test-delroots-%d", i)
		// index successive versions of the same object
		mock := mock.NewIndexingObject(id)
		root := index.ObjectRoot{Path: mock.RootDir}
		expNil(t, tx.IndexObjectRoot(ctx, indexedAt, root))
		indexedAt = indexedAt.Add(time.Hour)
	}
	expNil(t, tx.Commit())
	roots, err := idx.DEBUG_AllObjecRootss(ctx)
	expNil(t, err)
	lenBefore := len(roots)
	tx, err = idx.NewTx(ctx)
	expNil(t, err)
	// should remove seven
	expNil(t, tx.RemoveObjectsBefore(ctx, time.Now()))
	expNil(t, tx.Commit())
	roots, err = idx.DEBUG_AllObjecRootss(ctx)
	expNil(t, err)
	lenAfter := len(roots)
	// expect 8 deleted objects
	expEq(t, "deleted object roots", lenAfter, lenBefore-8)
}

func TestListObjects(t *testing.T) {
	ctx := context.Background()
	t.Run("empty", func(t *testing.T) {
		idx, err := setupSqliteIndex(ctx, t.Name(), func(tx index.BackendTx) error {
			return nil
		})
		expNil(t, err)
		_, err = idx.ListObjects(ctx, "", 10, "")
		expNil(t, err)
	})
	t.Run("no prefix", func(t *testing.T) {
		const numInvs = 27
		mocks := make([]*mock.IndexingObject, numInvs)
		idx, err := setupSqliteIndex(ctx, t.Name(), func(tx index.BackendTx) error {
			letters := []rune("abcdefghijklmnopqrstuvqwxyz")
			for i := 0; i < len(mocks); i++ {
				id := fmt.Sprintf("%c-test-%d", letters[i%len(letters)], i)
				m := mock.NewIndexingObject(id, mock.WithHead(ocfl.V(2)))
				err := tx.IndexObjectInventory(ctx, m.IndexedAt, index.ObjectInventory{
					Inventory: m.Inventory,
					Path:      m.RootDir,
				})
				if err != nil {
					return err
				}
				mocks[i] = m
			}
			return nil
		})
		expNil(t, err)
		sort.Slice(mocks, func(i, j int) bool {
			return mocks[i].Inventory.ID < mocks[j].Inventory.ID
		})
		objs := []index.ObjectListItem{}
		cursor := ""
		for {
			results, err := idx.ListObjects(ctx, "", 5, cursor)
			expNil(t, err)
			objs = append(objs, results.Objects...)
			cursor = results.NextCursor
			if cursor == "" {
				break
			}
		}
		expEq(t, "number of objects in result", len(objs), len(mocks))
		for i, m := range mocks {
			expEq(t, "result id matches mock id", objs[i].ID, m.Inventory.ID)
			expEq(t, "result root matches mock root", objs[i].RootPath, m.RootDir)
			expEq(t, "result spec matches mock spec", objs[i].Spec, m.Inventory.Type.Spec)
			expEq(t, "result head matches mock head", objs[i].Head, m.Inventory.Head)
			expEq(t, "result v1 created match mock v1 created", objs[i].V1Created, m.Inventory.Versions[ocfl.V(1)].Created)
			expEq(t, "result head created match mock v2 created", objs[i].HeadCreated, m.Inventory.Versions[ocfl.V(2)].Created)
		}
	})

	t.Run("with prefix", func(t *testing.T) {
		const numInvs = 54
		idx, err := setupSqliteIndex(ctx, t.Name(), func(tx index.BackendTx) error {
			letters := []rune("abcdefghijklmnopqrstuvqwxyz")
			for i := 0; i < numInvs; i++ {
				id := fmt.Sprintf("%c-test-%d", letters[i%len(letters)], i)
				m := mock.NewIndexingObject(id, mock.WithHead(ocfl.V(2)))
				err := tx.IndexObjectInventory(ctx, m.IndexedAt, index.ObjectInventory{
					Inventory: m.Inventory,
					Path:      m.RootDir,
				})
				if err != nil {
					return err
				}
			}
			return nil
		})
		expNil(t, err)
		results, err := idx.ListObjects(ctx, "a", 5, "")
		expNil(t, err)
		expEq(t, "number of results", len(results.Objects), 2)
		expEq(t, "next page cursor", results.NextCursor, "")
	})

}

func TestGetObjectState(t *testing.T) {
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx, t.Name())
	expNil(t, err)
	defer idx.Close()
	tx, err := idx.NewTx(ctx)
	expNil(t, err)
	defer tx.Rollback()
	// index an object with 1231 files in a directory called "long"
	id := "object-1"
	head := ocfl.V(1)
	dir := "long"
	dirsize := 1231
	mock1 := mock.NewIndexingObject(id,
		mock.WithHead(head),
		mock.BigDir(dir, dirsize))
	err = tx.IndexObjectInventory(ctx, mock1.IndexedAt, index.ObjectInventory{
		Inventory: mock1.Inventory,
		Path:      mock1.RootDir,
	})
	expNil(t, err)
	expNil(t, tx.Commit())
	// read long directory
	cursor := ""
	var allFiles []string
	for {
		// list of all files in head state
		pathinfo, err := idx.GetObjectState(ctx, id, head, ".", true, 41, cursor)
		expNil(t, err)
		for _, ch := range pathinfo.Children {
			allFiles = append(allFiles, ch.Name)
		}
		if pathinfo.NextCursor == "" {
			break
		}
		cursor = pathinfo.NextCursor
	}
	expEq(t, "directory entries", len(allFiles), dirsize+4)
	for _, f := range allFiles {
		inf, err := idx.GetObjectState(ctx, id, head, f, false, 1, "")
		expNil(t, err)
		// expEq(t, "HasSize", inf.HasSize, true)
		expEq(t, "IsDir", inf.IsDir, false)
		expEq(t, "Sum len", len(inf.Sum), 128)
		expEq(t, "Children len", len(inf.Children), 0)
		// if inf.Size == 0 {
		// 	t.Fatal("size is missing")
		// }
	}
}

func newSqliteIndex(ctx context.Context, name string) (*sqlite.Backend, error) {
	con := fmt.Sprintf("file:%s?mode=memory&_busy_timeout=10000&_journal=WAL&_sync=NORMAL&cache=shared", name)
	idx, err := sqlite.Open(con)
	if err != nil {
		return nil, err
	}
	if _, err = idx.InitSchema(ctx); err != nil {
		idx.Close()
		return nil, err
	}
	return idx, nil
}

func setupSqliteIndex(ctx context.Context, name string, setup func(tx index.BackendTx) error) (*sqlite.Backend, error) {
	db, err := newSqliteIndex(ctx, name)
	if err != nil {
		return nil, err
	}
	tx, err := db.NewTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := setup(tx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return db, nil
}

func expNil(t *testing.T, val any) {
	t.Helper()
	if val != nil {
		t.Fatalf("%v!=nil", val)
	}
}

func expNotNil(t *testing.T, desc string, val any) {
	t.Helper()
	if val == nil {
		t.Fatalf("%s=nil", desc)
	}
}

func expEq(t *testing.T, desc string, got, expect any) {
	t.Helper()
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("%s: got='%v', expected='%v'", desc, got, expect)
	}
}
