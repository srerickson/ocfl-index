package index

import (
	"encoding/hex"
	"fmt"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/ocflv1"
	"github.com/srerickson/ocfl/pathtree"
)

type Tree struct {
	*Node
}

type TreeVal struct {
	Sum  []byte
	Path string
}

type Node = pathtree.Node[*TreeVal]

func InventoryTree(inv *ocflv1.Inventory) (*Tree, error) {
	fullIndex := ocfl.NewIndex()
	for vnum := range inv.Versions {
		verIndex, err := inv.IndexFull(vnum, true, false)
		if err != nil {
			return nil, err
		}
		if err := fullIndex.SetDir(vnum.String(), verIndex, false); err != nil {
			return nil, err
		}
	}
	if err := fullIndex.SetDirDigests(inv.DigestAlgorithm); err != nil {
		return nil, err
	}
	mapFn := func(inf *ocfl.IndexItem) (*TreeVal, error) {
		alg := inv.DigestAlgorithm
		sumstr, exists := inf.Digests[alg]
		if !exists {
			return nil, fmt.Errorf("missing %s digest", alg)
		}
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
		return &TreeVal{
			Sum:  sumbyt,
			Path: srcPath,
		}, nil

	}
	newRoot, err := ocfl.MapIndex(fullIndex, mapFn)
	if err != nil {
		return nil, err
	}
	return &Tree{Node: newRoot}, nil
}
