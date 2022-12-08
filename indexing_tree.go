package index

import (
	"encoding/hex"
	"fmt"
	"path"

	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

// IndexingTree is
type IndexingTree struct {
	pathtree.Node[IndexingVal]
	alg digest.Alg
}

type IndexingVal struct {
	Sum  []byte // digest for an index node (file or dir)
	Path string // content path from manifest (file only)
}

// SetDirDigests generates sums values for directories in idx based on their
// contents.
func (idx *IndexingTree) SetDirDigests() error {
	return digestDirNode(&idx.Node, idx.alg)
}

// digestDirNode generates sum values for directories in node based on their
// contents
func digestDirNode(node *pathtree.Node[IndexingVal], alg digest.Alg) error {
	if !node.IsDir() {
		return nil
	}
	dirHash := alg.New()
	for _, d := range node.DirEntries() {
		n := d.Name()
		ch := node.Child(n)
		if err := digestDirNode(ch, alg); err != nil {
			return err
		}
		typ := "f" // file or directory
		if ch.IsDir() {
			typ = "d"
		}
		_, err := fmt.Fprintf(dirHash, "%x %s %s\n", ch.Val.Sum, typ, n)
		if err != nil {
			return err
		}
	}
	node.Val.Sum = dirHash.Sum(nil)
	return nil
}

// InventoryTree returns an IndexingTree with all node values from the
// inventory.
func InventoryTree(inv *ocflv1.Inventory) (*IndexingTree, error) {
	alg, err := digest.Get(inv.DigestAlgorithm)
	if err != nil {
		return nil, err
	}
	fullIndex := &IndexingTree{
		alg:  alg,
		Node: *pathtree.NewDir[IndexingVal](),
	}
	for vnum := range inv.Versions {
		verIndex, err := inv.Index(vnum)
		if err != nil {
			return nil, fmt.Errorf("indexing %s: %w", vnum, err)
		}
		walkFn := func(name string, isdir bool, digs digest.Set, src []string) error {
			if isdir {
				return nil
			}
			if len(src) == 0 {
				return fmt.Errorf("inventory has missing content path for '%s'", name)
			}
			sumstr := digs[alg.ID()]
			if sumstr == "" {
				return fmt.Errorf("inventory has missing %s for %s", alg.ID(), name)
			}
			var err error
			newVal := IndexingVal{Path: src[0]}
			newVal.Sum, err = hex.DecodeString(sumstr)
			if err != nil {
				return fmt.Errorf("inventory has invalid digest for %s: %w", name, err)
			}
			return fullIndex.Set(path.Join(vnum.String(), name), pathtree.NewFile(newVal))
		}
		if err := verIndex.Walk(walkFn); err != nil {
			return nil, fmt.Errorf("indexing %s: %w", vnum, err)
		}
	}
	if err := fullIndex.SetDirDigests(); err != nil {
		return nil, err
	}
	return fullIndex, nil
}
