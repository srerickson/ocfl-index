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
	idx, err := sqlite.Open("file:tmp.sqlite?mode=memory")
	expNil(t, err)
	defer idx.Close()
	ctx := context.Background()
	_, err = idx.InitSchema(ctx)
	expNil(t, err)
	major, minor, err := idx.GetSchemaVersion(ctx)
	expNil(t, err)
	if major != expSchema[0] || minor != expSchema[1] {
		t.Errorf("expected schema version %d.%d, got %d.%d", expSchema[0], expSchema[1], major, minor)
	}
}

func TestSummary(t *testing.T) {
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx, t.Name())
	expNil(t, err)
	defer idx.Close()
	tx, err := idx.NewTx(ctx)
	expNil(t, err)
	defer tx.Rollback()
	m := mock.NewIndexingObject("test-object")
	expNil(t, tx.IndexObjectInventory(ctx, m.IndexedAt, index.ObjectInventory{
		Path:      m.RootDir,
		Inventory: m.Inventory,
	}))
	expNil(t, tx.Commit())
	summary, err := idx.GetIndexSummary(ctx)
	expNil(t, err)
	exp := index.IndexSummary{
		NumInventories: 1,
		NumObjects:     1,
		UpdatedAt:      m.IndexedAt.UTC(),
	}
	expEq(t, "indexed summary", summary, exp)
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
	idx, err := sqlite.Open("file:test_index_inventory.sqlite?mode=memory")
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	_, err = idx.InitSchema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := idx.NewTx(ctx)
	expNil(t, err)
	defer tx.Rollback()
	const numInvs = 27
	idxObjs := make([]*mock.IndexingObject, numInvs)
	letters := []rune("abcdefghijklmnopqrstuvqwxyz")
	for i := 0; i < len(idxObjs); i++ {
		l := string(letters[i%len(letters)])
		idxObjs[i] = mock.NewIndexingObject(fmt.Sprintf("%s-test-%d", l, i))

		err := tx.IndexObjectInventory(ctx, idxObjs[i].IndexedAt, index.ObjectInventory{
			Inventory: idxObjs[i].Inventory,
			Path:      idxObjs[i].RootDir,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	expNil(t, tx.Commit())

	t.Run("sort by ID, ASC", func(t *testing.T) {
		sort.Slice(idxObjs, func(i, j int) bool {
			return idxObjs[i].Inventory.ID < idxObjs[j].Inventory.ID
		})
		allObjects := make([]index.ObjectListItem, 0, len(idxObjs))
		cursor := ""
		limit := 3
		sort := index.SortID | index.ASC
		for {
			list, err := idx.ListObjects(ctx, sort, limit, cursor)
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Objects) == 0 {
				break
			}
			allObjects = append(allObjects, list.Objects...)
			cursor = list.NextCursor
		}
		if l := len(allObjects); l != len(idxObjs) {
			t.Fatal("wrong number of total objects")
		}
		for i := range allObjects {
			if allObjects[i].ID != idxObjs[i].Inventory.ID {
				t.Fatal("failed sort")
			}
		}
	})

	t.Run("sort by ID, DESC", func(t *testing.T) {
		sort.Slice(idxObjs, func(i, j int) bool {
			return idxObjs[i].Inventory.ID > idxObjs[j].Inventory.ID
		})
		allObjects := make([]index.ObjectListItem, 0, len(idxObjs))
		cursor := ""
		limit := 3
		sort := index.SortID | index.DESC
		for {
			list, err := idx.ListObjects(ctx, sort, limit, cursor)
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Objects) == 0 {
				break
			}
			allObjects = append(allObjects, list.Objects...)
			cursor = list.NextCursor
		}
		if l := len(allObjects); l != len(idxObjs) {
			t.Fatalf("wrong number of total objects: got %d, want %d", l, len(idxObjs))
		}
		for i := range allObjects {
			if allObjects[i].ID != idxObjs[i].Inventory.ID {
				t.Fatal("failed sort")
			}
		}
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
