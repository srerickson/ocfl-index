package index_test

import (
	"context"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl/ocflv1"
)

func TestNewIndexingObject(t *testing.T) {
	ctx := context.Background()
	fsys := ocfl.DirFS(filepath.Join(fixtureRoot, "simple-root"))
	dirents, err := fsys.ReadDir(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, dirent := range dirents {
		if !dirent.Type().IsDir() {
			continue
		}
		obj, err := ocflv1.GetObject(ctx, fsys, dirent.Name())
		if err != nil {
			t.Fatal(err)
		}
		inv, err := obj.Inventory(ctx)
		if err != nil {
			t.Fatal(err)
		}
		idxObj, err := index.NewIndexingObject(ctx, obj, true)
		if err != nil {
			t.Fatal(err)
		}
		if idxObj.Obj.ID != inv.ID {
			t.Fatal("wrong inventory in indexing object")
		}
		if idxObj.Obj.DigestAlgorithm != inv.DigestAlgorithm {
			t.Fatal("wrong digest algorithm in indexing object")
		}
		if idxObj.Obj.Head != inv.Head {
			t.Fatal("wrong head in indexing object")
		}
		if idxObj.Obj.InventoryDigest != inv.Digest() {
			t.Fatal("wrong inventory digest in indexing object")
		}
		if idxObj.Obj.Spec != inv.Type.Spec {
			t.Fatal("wrong ocfl spec in indexing object")
		}
		if idxObj.Obj.RootPath != dirent.Name() {
			t.Fatal("wrong root path in indexing object")
		}
		if len(idxObj.Obj.Versions) != len(inv.Versions) {
			t.Fatal("wrong number of versions in indexing object")
		}
		for i, idxVer := range idxObj.Obj.Versions {
			if idxVer.Num.Num() != i+1 {
				t.Fatalf("indexing object versions aren't sorted correctly: i=%d is %s", i+1, idxVer.Num)
			}
			ver := inv.Versions[idxVer.Num]
			if idxVer.Created != ver.Created {
				t.Fatal("wrong created date in idexing object version")
			}
			if idxVer.Message != ver.Message {
				t.Fatal("wrong message in indexing object version")
			}
			if ver.User != nil {
				if idxVer.User == nil {
					t.Fatal("missing user in indexing object version")
				}
				if idxVer.User.Name != ver.User.Name {
					t.Fatal("wrong user name in indexing object version")
				}
				if idxVer.User.Address != ver.User.Address {
					t.Fatal("wrong user name in indexing object version")
				}
			}
			//every path in the version state should be in the indexing tree
			for name, digest := range ver.State.AllPaths() {
				node, err := idxObj.State[idxVer.Num].Get(name)
				if err != nil {
					t.Fatal("indexing object version state err:", err)
				}
				if !strings.EqualFold(hex.EncodeToString(node.Val.Sum), digest) {
					t.Fatal("wrong digest in indexing object version state")
				}
				if node.Val.Path == "" {
					t.Fatal("missing src path in indexing object version state")
				}
				// fixme: files can have zero size, even if they were scanned
				// if node.Val.Size == 0 {
				// 	t.Fatal("missing size in indexing object version state", dirent.Name(), idxVer.Num, name, node.Val.Path)
				// }
			}

		}
	}
}
