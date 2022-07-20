package digest_test

import (
	"crypto/sha256"
	"testing"

	index "github.com/srerickson/ocfl-index/internal/digest"
)

func sum(val string) []byte {
	h := sha256.Sum256([]byte(val))
	return h[:]
}

func TestDTree(t *testing.T) {
	n := index.NewTree()
	n.Set("v1/a.txt", sum("contents-1"))
	n.Set("v2/b.txt", sum("contents-2"))
	n.Set("v2/c.txt", sum("contents-3"))
	n.Digest(sha256.New)

	children := n.Children()
	expect := []string{"v1", "v2"}
	if len(children) != len(expect) {
		t.Fatalf("children should be %v, got %v", children, expect)
	}
	for i := range expect {
		if expect[i] != children[i] {
			t.Fatalf("children should be %v, got %v", children, expect)
		}
	}

}
