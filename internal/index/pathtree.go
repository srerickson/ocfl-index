package index

import (
	"encoding/hex"
	"fmt"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

type IndexingVal struct {
	Sum     []byte // digest for an index node (file or dir)
	Size    int64  // file size if indexing file size
	HasSize bool   // Whether we have size for the node
}

// PathTree builds path tree with recursive digests and and size information for the specified version state
// in the inventory.
func PathTree(inv *ocflv1.Inventory, vnum ocfl.VNum, sizes map[string]int64) (*pathtree.Node[IndexingVal], error) {
	ver := inv.Versions[vnum]
	if ver == nil {
		return nil, fmt.Errorf("missing version '%s' in inventory", vnum)
	}
	vroot := pathtree.NewDir[IndexingVal]()
	if err := ver.State.EachPath(func(logical string, digest string) error {
		var err error
		var idxVal IndexingVal
		idxVal.Sum, err = hex.DecodeString(digest)
		if err != nil {
			return err
		}
		if len(sizes) > 0 {
			cp, _ := inv.ContentPath(vnum.Num(), logical)
			if size, ok := sizes[cp]; ok {
				idxVal.HasSize = true
				idxVal.Size = size
			}
		}
		return vroot.Set(logical, pathtree.NewFile(idxVal))
	}); err != nil {
		return nil, err
	}
	// directory digests are always sha256, regardless of invnetory digest
	if err := dirDigests(vroot, digest.SHA256()); err != nil {
		return nil, err
	}
	if len(sizes) > 0 {
		dirSizes(vroot)
	}
	return vroot, nil
}

// dirDigests generates digest values for directories in the pathtree based on contents
func dirDigests(node *pathtree.Node[IndexingVal], alg digest.Alg) error {
	if !node.IsDir() {
		return nil
	}
	dirHash := alg.New()
	for _, d := range node.DirEntries() {
		n := d.Name()
		ch := node.Child(n)
		if ch.IsDir() {
			if err := dirDigests(ch, alg); err != nil {
				return err
			}
			n += "/"
		}
		if _, err := fmt.Fprintf(dirHash, "%x %s\n", ch.Val.Sum, n); err != nil {
			return err
		}
	}
	node.Val.Sum = dirHash.Sum(nil)
	return nil
}

// DirSizes generates digest sizes for directories in the pathtree based on contents.
// A directory can only have size information if all it's children have size information.
func dirSizes(node *pathtree.Node[IndexingVal]) {
	if !node.IsDir() {
		return
	}
	dirSize := int64(0)
	allChilSizes := true // all children have size info
	for _, d := range node.DirEntries() {
		n := d.Name()
		ch := node.Child(n)
		if ch.IsDir() {
			dirSizes(ch)
		}
		allChilSizes = allChilSizes && ch.Val.HasSize
		dirSize += ch.Val.Size
	}
	node.Val.HasSize = allChilSizes
	if allChilSizes {
		node.Val.Size = dirSize
	}
}
