package index

import (
	"encoding/hex"
	"fmt"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
	"github.com/srerickson/ocfl/pathtree"
)

// IndexingTree is an alias for a pathtree.Node that stores references to
// IndexingVal. Indexing tree is used to temporarily hold values from
// the an inventory during the indexing process.
type IndexingTree = pathtree.Node[*IndexingVal]

type IndexingVal struct {
	Sum  []byte // digest for an index node (file or dir)
	Path string // content path from manifest (file only)
}

// InventoryTree returns an IndexingTree with all node values from the
// inventory.
func InventoryTree(inv *ocflv1.Inventory) (*IndexingTree, error) {
	fullIndex := ocfl.NewIndex()
	for vnum := range inv.Versions {
		verIndex, err := inv.IndexFull(vnum, true, false)
		if err != nil {
			return nil, fmt.Errorf("indexing %s: %w", vnum, err)
		}
		if err := fullIndex.SetDir(vnum.String(), verIndex, false); err != nil {
			return nil, fmt.Errorf("indexing %s: %w", vnum, err)
		}
	}
	if err := fullIndex.SetDirDigests(inv.DigestAlgorithm); err != nil {
		return nil, err
	}
	mapFn := func(inf *ocfl.IndexItem) (*IndexingVal, error) {
		alg := inv.DigestAlgorithm
		sumstr := inf.Digests[alg]
		if sumstr == "" {
			return nil, fmt.Errorf("missing %s digest", alg)
		}
		sumbyt, err := hex.DecodeString(sumstr)
		if err != nil {
			return nil, fmt.Errorf("decoding digest: %w", err)
		}
		var srcPath string
		if len(inf.SrcPaths) > 0 {
			srcPath = inf.SrcPaths[0]
		}
		return &IndexingVal{
			Sum:  sumbyt,
			Path: srcPath,
		}, nil

	}
	return ocfl.MapIndex(fullIndex, mapFn)
}
