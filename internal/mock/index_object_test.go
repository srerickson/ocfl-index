package mock_test

import (
	"testing"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/mock"
)

func TestIndexingObject(t *testing.T) {
	mk := mock.NewIndexingObject("test-inv",
		mock.WithHead(ocfl.V(3)),
		mock.BigDir("big", 1000),
	)
	if len(mk.Inventory.Versions) != 3 {
		t.Fatal("expected three versions")
	}
	for v := range mk.Inventory.Versions {
		tree, err := index.PathTree(mk.Inventory, v, mk.FileSizes)
		if err != nil {
			t.Fatal(err)
		}
		if !tree.Val.HasSize {
			t.Fatal("expected tree to have file size")
		}
	}
}
