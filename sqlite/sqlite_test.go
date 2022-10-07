package sqlite_test

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/ocfltest"
	"github.com/srerickson/ocfl-index/sqlite"
	"github.com/srerickson/ocfl/ocflv1"
)

func TestCreateTables(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", "file:tmp.sqlite?mode=memory")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	idx := sqlite.New(sqlDB)
	ctx := context.Background()
	_, err = idx.MigrateSchema(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	// if created != true {
	// 	t.Error("expected CreateTables to return true")
	// }
	major, minor, err := idx.GetSchemaVersion(ctx)
	if err != nil {
		t.Error(err)
	}
	if major != 0 && minor != 1 {
		t.Errorf("expected schema version 0.1, got %d.%d", major, minor)
	}
	// create and erase
	_, err = idx.MigrateSchema(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	// if created != true {
	// 	t.Error("expected CreateTables to return true")
	// }
}

func TestIndexInventory(t *testing.T) {
	const numInvs = 50
	sqlDB, err := sql.Open("sqlite", "file:test_index_inventory.sqlite?mode=memory")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	idx := sqlite.New(sqlDB)
	ctx := context.Background()
	_, err = idx.MigrateSchema(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	invs := make([]*ocflv1.Inventory, numInvs)
	for i := 0; i < len(invs); i++ {
		invs[i] = ocfltest.GenInv(&ocfltest.GenInvConf{
			ID:       fmt.Sprintf("test-%d", i),
			Head:     ocfl.V(rand.Intn(3) + 1), // 1-3 versions
			Numfiles: 10,
			Del:      .05, // delete .05 of files with each version
			Add:      .05, // modify .05 of files remaining
			Mod:      .05, // add .05 new random file
		})
		err := idx.IndexInventory(ctx, invs[i])
		if err != nil {
			t.Fatal(err)
		}
	}
	// check inventory is indexed
	for _, inv := range invs {
		verRes, err := idx.GetVersions(ctx, inv.ID)
		if err != nil {
			t.Fatal(inv.ID, err)
		}
		if l := len(verRes.Versions); l != inv.Head.Num() {
			for i := range verRes.Versions {
				t.Log(verRes.Versions[i].Num)
			}
			t.Fatalf("expected %d versions, got %d", inv.Head.Num(), l)
		}
		for i, vnum := range inv.VNums() {
			ver := inv.Versions[vnum]
			// created
			idxCreated := verRes.Versions[i].Created.Unix()
			expCreated := ver.Created.Unix()
			if idxCreated != expCreated {
				t.Fatalf("indexed version date doesn't match: %v, not %v", idxCreated, expCreated)
			}
			// mesage
			idxMessage := verRes.Versions[vnum.Num()-1].Message
			expMessage := ver.Message
			if idxMessage != expMessage {
				t.Fatalf("indexed version message doesn't match: %v, not %v", idxMessage, expMessage)
			}
			verIndex, err := inv.IndexFull(vnum, true, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := verIndex.SetDirDigests(inv.DigestAlgorithm); err != nil {
				t.Fatal(err)
			}
			if _, err := idx.GetContent(ctx, inv.ID, vnum, "."); err != nil {
				t.Fatal(err)
			}
			verIndex.Walk(func(lpath string, isdir bool, vals *ocfl.IndexItem) error {
				entry, err := idx.GetContent(ctx, inv.ID, vnum, lpath)
				if err != nil {
					t.Fatal(inv.ID, vnum, lpath, err)
				}
				if entry.Content.IsDir != isdir {
					t.Fatalf("expected sqlIndex value to match ocfl.Index value for %s", lpath)
				}
				if !strings.EqualFold(entry.Content.Sum, vals.Digests[inv.DigestAlgorithm]) {
					t.Fatalf("%s: %s != %s", lpath, entry.Content.Sum, vals.Digests[inv.DigestAlgorithm])
				}
				if isdir {
					return nil
				}

				if !vals.HasSrc(entry.Content.ContentPath) {
					t.Fatalf("GetContent didn't return expected content path for %s, %s, %s: %s not in %s",
						inv.ID, vnum, lpath, entry.Content.ContentPath, strings.Join(vals.SrcPaths, ", "))
				}

				return nil
			})
		}
	}

}
