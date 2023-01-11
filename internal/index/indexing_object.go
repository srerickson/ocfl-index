package index

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

// IndexingObject represents an OCFL Object for the purposes of indexing.
type IndexingObject struct {
	Obj   *Object
	State map[ocfl.VNum]*pathtree.Node[IndexingVal]
}

type IndexingVal struct {
	Sum  []byte // digest for an index node (file or dir)
	Path string // content path from manifest (file only)
	Size int64  // file size if indexing file size
}

func NewIndexingObject(ctx context.Context, obj *ocflv1.Object, withSize bool) (*IndexingObject, error) {
	fsys, objRoot := obj.Root()
	inv, err := obj.Inventory(ctx)
	if err != nil {
		return nil, err
	}
	idxObj, err := newIndexingObject(inv)
	if err != nil {
		return nil, err
	}
	idxObj.Obj.RootPath = objRoot
	if !withSize {
		return idxObj, nil
	}
	// map source files -> size
	sizes := map[string]int64{}
	for vnum := range idxObj.State {
		// This approach to scanning an object's content for file size information
		// feels too low-level. It requires too much knowledge about the internal
		// structure of an OCFL object. It would be nice for the ocflv1 package
		// to provide an api that abstracts some of this.
		prefix := path.Join(objRoot, vnum.String(), inv.ContentDirectory)
		fn := func(name string, dirent fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name = strings.TrimPrefix(name, objRoot+"/")
			info, err := dirent.Info()
			if err != nil {
				return err
			}
			sizes[name] = info.Size()
			return nil
		}
		if err := ocfl.EachFile(ctx, fsys, prefix, fn); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// OK if content directory doesn't exist.. skip this version
				continue
			}
			return nil, err
		}
	}
	// Apply content file size information to all version states
	for _, node := range idxObj.State {
		if err := pathtree.Walk(node, func(n string, node *pathtree.Node[IndexingVal]) error {
			if node.IsDir() {
				return nil
			}
			size, exists := sizes[node.Val.Path]
			if !exists {
				return fmt.Errorf("while getting file sizes: no size found for srcpath='%s' ", node.Val.Path)
			}
			node.Val.Size = size
			return nil
		}); err != nil {
			return nil, err
		}
		DirSizes(node)
	}

	return idxObj, nil
}

// DirDigests generates digest values for directories in the pathtree based on contents
func DirDigests(node *pathtree.Node[IndexingVal], alg digest.Alg) error {
	if !node.IsDir() {
		return nil
	}
	dirHash := alg.New()
	for _, d := range node.DirEntries() {
		n := d.Name()
		ch := node.Child(n)
		if ch.IsDir() {
			if err := DirDigests(ch, alg); err != nil {
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

// DirSizes generates digest sizes for directories in the pathtree based on contents
func DirSizes(node *pathtree.Node[IndexingVal]) {
	if !node.IsDir() {
		return
	}
	dirSize := int64(0)
	for _, d := range node.DirEntries() {
		n := d.Name()
		ch := node.Child(n)
		if ch.IsDir() {
			DirSizes(ch)
		}
		dirSize += ch.Val.Size
	}
	node.Val.Size = dirSize
}

// newIndexingObject returns an IndexingTree with all node values from the
// inventory.
func newIndexingObject(inv *ocflv1.Inventory) (*IndexingObject, error) {
	alg, err := digest.Get(inv.DigestAlgorithm)
	if err != nil {
		return nil, err
	}
	fullIndex := &IndexingObject{
		Obj: &Object{
			ID:              inv.ID,
			Spec:            inv.Type.Spec,
			Head:            inv.Head,
			DigestAlgorithm: inv.DigestAlgorithm,
			Versions:        make([]*ObjectVersion, len(inv.Versions)),
			InventoryDigest: inv.Digest(),
		},
		State: make(map[ocfl.VNum]*pathtree.Node[IndexingVal], len(inv.Versions)),
	}
	for vnum, ver := range inv.Versions {
		// this assumes the inventory is valid
		fullIndex.Obj.Versions[vnum.Num()-1] = &ObjectVersion{
			Num:     vnum,
			Created: ver.Created,
			Message: ver.Message,
			User:    ver.User,
		}
		vroot := pathtree.NewDir[IndexingVal]()
		if err := ver.State.EachPath(func(logical string, digest string) error {
			var err error
			var idxVal IndexingVal
			idxVal.Path, err = inv.ContentPath(vnum, logical)
			if err != nil {
				return err
			}
			idxVal.Sum, err = hex.DecodeString(digest)
			if err != nil {
				return err
			}
			return vroot.Set(logical, pathtree.NewFile(idxVal))
		}); err != nil {
			return nil, err
		}
		if err := DirDigests(vroot, alg); err != nil {
			return nil, err
		}
		fullIndex.State[vnum] = vroot
	}
	return fullIndex, nil
}
