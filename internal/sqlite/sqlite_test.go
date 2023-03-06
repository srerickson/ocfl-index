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
	idx, err := sqlite.Open("file:tmp.sqlite?mode=memory")
	expNil(t, err)
	defer idx.Close()
	ctx := context.Background()
	_, err = idx.InitSchema(ctx)
	expNil(t, err)
	major, minor, err := idx.GetSchemaVersion(ctx)
	expNil(t, err)
	if major != 0 || minor != 3 {
		t.Errorf("expected schema version 0.2, got %d.%d", major, minor)
	}
}

func TestSummary(t *testing.T) {
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx)
	expNil(t, err)
	defer idx.Close()
	m := mock.NewIndexingObject("test-object", index.ModeInventories)
	expNil(t, idx.IndexObjectInventory(ctx, m.RootDir, m.IndexedAt, m.Inventory))
	sum := index.StoreSummary{
		RootPath:    "root-dir",
		Description: "store description",
		Spec:        ocfl.Spec{1, 1},
		NumObjects:  1,
	}
	expNil(t, idx.SetStoreInfo(ctx, sum.RootPath, sum.Description, sum.Spec))
	summary, err := idx.GetStoreSummary(ctx)
	expNil(t, err)
	expEq(t, "indexed summary", summary, sum)
}

func TestIndexObject(t *testing.T) {
	// TODO: scenarios to test
	// - basic inventory: rows created
	// - repeat index of same inventory: no changes
	// - index inventory with head=v1, head=v2 (same no file size)
	// - index inventory (head=1), then index file sizes (v2)
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx)
	expNil(t, err)
	defer idx.Close()
	id := "test-object"
	// index successive versions of the same object
	mock1 := mock.NewIndexingObject(id, index.ModeObjectDirs)
	mock2 := mock.NewIndexingObject(id, index.ModeInventories, mock.WithHead(ocfl.V(2)))
	mock2.IndexedAt = mock1.IndexedAt.AddDate(0, 0, 1)
	mock3 := mock.NewIndexingObject(id, index.ModeFileSizes, mock.WithHead(ocfl.V(3)))
	mock3.IndexedAt = mock1.IndexedAt.AddDate(0, 0, 2)
	expNil(t, idx.IndexObjectRoot(ctx, mock1.RootDir, mock1.IndexedAt))
	expNil(t, idx.IndexObjectInventory(ctx, mock2.RootDir, mock2.IndexedAt, mock2.Inventory))
	expNil(t, idx.IndexObjectInventorySize(ctx, mock3.RootDir, mock3.IndexedAt, mock3.Inventory, mock3.FileSizes))
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
	idx, err := newSqliteIndex(ctx)
	expNil(t, err)
	defer idx.Close()
	numObjects := 721
	for i := 0; i < numObjects; i++ {
		id := fmt.Sprintf("test-listroots-%d", i)
		// index successive versions of the same object
		mock := mock.NewIndexingObject(id, index.ModeObjectDirs)
		expNil(t, idx.IndexObjectRoot(ctx, mock.RootDir, mock.IndexedAt))
	}
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
	const numInvs = 27
	idxObjs := make([]*mock.IndexingObject, numInvs)
	letters := []rune("abcdefghijklmnopqrstuvqwxyz")
	for i := 0; i < len(idxObjs); i++ {
		l := string(letters[i%len(letters)])
		idxObjs[i] = mock.NewIndexingObject(fmt.Sprintf("%s-test-%d", l, i), index.ModeInventories)
		err := idx.IndexObjectInventory(ctx, idxObjs[i].RootDir, idxObjs[i].IndexedAt, idxObjs[i].Inventory)
		if err != nil {
			t.Fatal(err)
		}
	}

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
	idx, err := newSqliteIndex(ctx)
	expNil(t, err)
	defer idx.Close()
	// index an object with 1231 files in a directory called "long"
	id := "object-1"
	head := ocfl.V(1)
	dir := "long"
	dirsize := 1231
	mock1 := mock.NewIndexingObject(id,
		index.ModeFileSizes,
		mock.WithHead(head),
		mock.BigDir(dir, dirsize))
	err = idx.IndexObjectInventorySize(ctx, mock1.RootDir, mock1.IndexedAt, mock1.Inventory, mock1.FileSizes)
	expNil(t, err)
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
		expEq(t, "HasSize", inf.HasSize, true)
		expEq(t, "IsDir", inf.IsDir, false)
		expEq(t, "Sum len", len(inf.Sum), 128)
		expEq(t, "Children len", len(inf.Children), 0)
		if inf.Size == 0 {
			t.Fatal("size is missing")
		}
	}
}

func newSqliteIndex(ctx context.Context) (*sqlite.Backend, error) {
	idx, err := sqlite.Open("file:test_index_inventory.sqlite?mode=memory")
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
