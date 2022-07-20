package digest

import (
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/srerickson/ocfl/ocflv1"
)

var (
	ErrInvalidPath = errors.New("invalid name or path")
	ErrNotFound    = errors.New("node not found")
	ErrNotDir      = errors.New("not a directory node")
	ErrNotFile     = errors.New("not a file node")
)

type Tree struct {
	children map[string]*Tree
	sum      []byte
}

func InventoryTree(inv *ocflv1.Inventory) (*Tree, error) {
	root := NewTree()
	for vnum, ver := range inv.Versions {
		root.MkdirAll(vnum.String()) // in case version is empty
		for lPath, cs := range ver.State.AllPaths() {
			sum, err := hex.DecodeString(cs)
			if err != nil {
				return nil, err
			}
			err = root.Set(path.Join(vnum.String(), lPath), sum)
			if err != nil {
				return nil, err
			}
		}
	}
	err := root.Digest(inv.DigestAlgorithm.New)
	if err != nil {
		return nil, err
	}
	return root, nil
}

func NewTree() *Tree {
	return &Tree{
		children: make(map[string]*Tree),
	}
}

func (n Tree) IsDir() bool {
	return n.children != nil
}

func (n *Tree) Val() []byte {
	return n.sum
}

func (n *Tree) Digest(newH func() hash.Hash) error {
	if n.children == nil {
		if n.sum == nil {
			return fmt.Errorf("missing value in file node")
		}
		return nil
	}
	for _, ch := range n.children {
		ch.Digest(newH)
	}
	h := newH()
	for _, name := range n.Children() {
		ch := n.children[name]
		_, err := fmt.Fprintf(h, "%x %s\n", ch.sum, name)
		if err != nil {
			return err
		}
	}
	n.sum = h.Sum(nil)
	return nil
}

func (n *Tree) Get(p string) (*Tree, error) {
	p = path.Clean(p)
	if !fs.ValidPath(p) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPath, p)
	}
	if p == "." {
		return n, nil
	}
	first, rest, more := strings.Cut(p, `/`)
	child, exists := n.children[first]
	if !exists {
		return nil, ErrNotFound
	}
	if more {
		return child.Get(rest)
	}
	return child, nil
}

func (n *Tree) Children() []string {
	if n.children == nil {
		return nil
	}
	names := make([]string, len(n.children))
	i := 0
	for n := range n.children {
		names[i] = n
		i++
	}
	sort.Strings(names)
	return names
}

func (n *Tree) Set(p string, val []byte) error {
	if !fs.ValidPath(p) {
		return ErrInvalidPath
	}
	dirName := path.Dir(p)
	baseName := path.Base(p)
	if baseName == "." {
		return ErrInvalidPath
	}
	parent, err := n.MkdirAll(dirName)
	if err != nil {
		return err
	}
	child, exists := parent.children[baseName]
	if !exists {
		parent.children[baseName] = &Tree{sum: val}
		return nil
	}
	if child.children != nil {
		return fmt.Errorf("%s: %w", p, ErrNotFile)
	}
	child.sum = val
	return nil
}

func (n *Tree) MkdirAll(p string) (*Tree, error) {
	if n.children == nil {
		return nil, ErrNotDir
	}
	p = path.Clean(p)
	if p == "." {
		return n, nil
	}
	name, rest, more := strings.Cut(p, "/")
	nextNode, exists := n.children[name]
	if !exists {
		nextNode = NewTree()
		n.children[name] = nextNode
	}
	if !more {
		return nextNode, nil
	}
	return nextNode.MkdirAll(rest)
}
