package sqlite_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/mock"
	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl-index/internal/sqlite"
)

func TestInitSchema(t *testing.T) {
	idx, err := sqlite.Open("file:tmp.sqlite?mode=memory")
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	ctx := context.Background()
	_, err = idx.InitSchema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	major, minor, err := idx.GetSchemaVersion(ctx)
	if err != nil {
		t.Error(err)
	}
	if major != 0 || minor != 2 {
		t.Errorf("expected schema version 0.2, got %d.%d", major, minor)
	}
}

func TestSummary(t *testing.T) {
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	m := mock.IndexingObject("test-object")
	if err := idx.IndexObject(ctx, m); err != nil {
		t.Fatal(err)
	}
	sum := index.StoreSummary{
		RootPath:    "root-dir",
		Description: "store description",
		Spec:        ocfl.Spec{1, 1},
		NumObjects:  1,
	}
	if err := idx.SetStoreInfo(ctx, sum.RootPath, sum.Description, sum.Spec); err != nil {
		t.Fatal(err)
	}
	summary, err := idx.GetStoreSummary(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sum, summary) {
		t.Fatalf("expected summary='%v', got='%v'", sum, summary)
	}
}

func TestIndexObject(t *testing.T) {
	ctx := context.Background()
	idx, err := newSqliteIndex(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	id := "test-object"
	m := mock.IndexingObject(id)
	if err := idx.IndexObject(ctx, m); err != nil {
		t.Fatal(err)
	}
	// object table values
	objects, err := idx.DEBUG_AllObjects(ctx)
	if err != nil {
		t.Fatal(id, err)
	}
	if l := len(objects); l != 1 {
		t.Fatalf("expected 1 object in the index, got %d", l)
	}
	obj := objects[0]
	if obj.OcflID != m.Obj.ID {
		t.Fatalf("expected id='%s', got id='%s'", m.Obj.ID, obj.OcflID)
	}
	if obj.Head != m.Obj.Head.String() {
		t.Fatalf("expected head='%s', got head='%s'", m.Obj.Head, obj.Head)
	}
	if obj.DigestAlgorithm != m.Obj.DigestAlgorithm {
		t.Fatalf("expected digest_algorithm='%s', got digest_algorithm='%s'", m.Obj.DigestAlgorithm, obj.DigestAlgorithm)
	}
	if obj.InventoryDigest != m.Obj.InventoryDigest {
		t.Fatalf("expected inventory_digest='%s', got inventory_digest='%s'", m.Obj.InventoryDigest, obj.InventoryDigest)
	}
	if obj.Spec != m.Obj.Spec.String() {
		t.Fatalf("expected spec='%s', got spec='%s'", m.Obj.Spec, obj.Spec)
	}
	// version table values
	versions, err := idx.DEBUG_AllVersions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if l := len(versions); l != 1 {
		t.Fatalf("expected 1 object version in the index, got %d", l)
	}
	version := versions[0]
	if version.Name != obj.Head {
		t.Fatalf("expected the version to have name='v1', got '%s'", version.Name)
	}
	if version.Num != 1 {
		t.Fatalf("expected the version to have num=1, got %d", version.Num)
	}
	// version created
	idxCreated := version.Created.Unix()
	expCreated := m.Obj.Versions[0].Created.Unix()
	if idxCreated != expCreated {
		t.Fatalf("expected version created='%v', got '%v'", expCreated, idxCreated)
	}
	// version mesage
	idxMessage := version.Message
	expMessage := m.Obj.Versions[0].Message
	if idxMessage != expMessage {
		t.Fatalf("indexed version message doesn't match: %v, not %v", idxMessage, expMessage)
	}
	// version user
	if u := m.Obj.Versions[0].User; u != nil {
		idxName := version.UserName
		idxAddr := version.UserAddress
		if u.Name != idxName {
			t.Fatalf("expected version user name='%s', got '%s'", u.Name, idxName)
		}
		if u.Address != idxAddr {
			t.Fatalf("expected version user address='%s', got '%s'", u.Address, idxAddr)
		}
	}
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
	idxObjs := make([]*index.IndexingObject, numInvs)
	letters := []rune("abcdefghijklmnopqrstuvqwxyz")
	for i := 0; i < len(idxObjs); i++ {
		l := string(letters[i%len(letters)])
		idxObjs[i] = mock.IndexingObject(fmt.Sprintf("%s-test-%d", l, i))
		err := idx.IndexObject(ctx, idxObjs[i])
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("sort by ID, ASC", func(t *testing.T) {
		sort.Slice(idxObjs, func(i, j int) bool {
			return idxObjs[i].Obj.ID < idxObjs[j].Obj.ID
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
			if allObjects[i].ID != idxObjs[i].Obj.ID {
				t.Fatal("failed sort")
			}
		}
	})

	t.Run("sort by ID, DESC", func(t *testing.T) {
		sort.Slice(idxObjs, func(i, j int) bool {
			return idxObjs[i].Obj.ID > idxObjs[j].Obj.ID
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
			if allObjects[i].ID != idxObjs[i].Obj.ID {
				t.Fatal("failed sort")
			}
		}
	})
}

func TestGetPathList(t *testing.T) {
	idx, err := sqlite.Open("file:test_index_inventory.sqlite?mode=memory")
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()
	ctx := context.Background()
	_, err = idx.InitSchema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	inv := mock.IndexingObject("test-object")
	if err := idx.IndexObject(ctx, inv); err != nil {
		t.Fatal(err)
	}
	if err := pathtree.Walk(inv.State[inv.Obj.Head], func(name string, sub *pathtree.Node[index.IndexingVal]) error {
		if sub.IsDir() {
			var expectChilds []string
			for _, ch := range sub.DirEntries() {
				expectChilds = append(expectChilds, ch.Name())
			}
			limits := []int{0, 1, 100, 1001} // test different limit settings
			for _, limit := range limits {
				gotChilds := make([]string, 0, len(expectChilds))
				cursor := ""
				for {
					list, err := idx.GetObjectState(ctx, inv.Obj.ID, inv.Obj.Head, name, false, limit, cursor)
					if err != nil {
						return err
					}
					if len(list.Children) == 0 {
						return errors.New("got empty path list")
					}
					for _, p := range list.Children {
						gotChilds = append(gotChilds, p.Name)
					}
					if list.NextCursor == "" {
						break
					}
					cursor = list.NextCursor
				}
				sort.Strings(expectChilds)
				sort.Strings(gotChilds)
				if !reflect.DeepEqual(gotChilds, expectChilds) {
					return fmt.Errorf("for %s: expected children: %v, got %v", name, expectChilds, gotChilds)
				}
			}
			return nil
		}
		info, err := idx.GetObjectState(ctx, inv.Obj.ID, inv.Obj.Head, name, false, 0, "")
		if err != nil {
			t.Fatal()
		}
		if info.IsDir {
			t.Fatalf("for '%s': expected isdir to be false", name)
		}
		expDigest := hex.EncodeToString(sub.Val.Sum)
		gotDigest := info.Sum
		if !strings.EqualFold(expDigest, gotDigest) {
			t.Fatalf("for '%s': go unexpected digest in index: '%s'", name, gotDigest)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

}

func newSqliteIndex(ctx context.Context) (*sqlite.Backend, error) {
	idx, err := sqlite.Open("file:test_index_inventory.sqlite?mode=memory")
	if err != nil {
		return nil, err
	}
	_, err = idx.InitSchema(ctx)
	if err != nil {
		idx.Close()
		return nil, err
	}
	return idx, nil
}
