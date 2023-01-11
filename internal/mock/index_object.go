package mock

import (
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl-index/internal/pathtree"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

type indexingObject index.IndexingObject

type IndexingObjectOption func(*indexingObject)

func IndexingObject(id string, opts ...IndexingObjectOption) *index.IndexingObject {
	obj := &indexingObject{
		Obj: &index.Object{
			ID:              id,
			Spec:            ocfl.Spec{1, 1},
			Head:            ocfl.V(1),
			DigestAlgorithm: digest.SHA512id,
			RootPath:        id,
		},
	}
	obj.State = make(map[ocfl.VNum]*pathtree.Node[index.IndexingVal])
	obj.State[obj.Obj.Head] = pathtree.NewDir[index.IndexingVal]()
	obj.State[obj.Obj.Head].SetFile("v1/file", index.IndexingVal{
		Sum:  quickSum(digest.SHA512(), "fake content"),
		Path: "v1/content/file",
		Size: 12,
	})
	for _, o := range opts {
		o(obj)
	}
	// versions with random creation times (up to six years ago)
	created := time.Now().AddDate(-1*(rand.Intn(6)+1), 0, 0).Truncate(time.Second)
	for _, vnum := range obj.Obj.Head.VNumSeq() {
		v := &index.ObjectVersion{
			Num:     vnum,
			Message: "commit version " + vnum.String(),
			User:    &ocflv1.User{Name: "nobody", Address: "email:none@none.com"},
			Created: created,
		}
		obj.Obj.Versions = append(obj.Obj.Versions, v)
		created = created.AddDate(0, 0, rand.Intn(90))
	}
	// generate inventory digest
	h := digest.SHA512().New()
	if err := json.NewEncoder(h).Encode(obj.Obj); err != nil {
		panic(err)
	}
	// generatee recursive digest for state tree
	for _, vroot := range obj.State {
		if err := index.DirDigests(vroot, digest.SHA512()); err != nil {
			panic(err)
		}
	}
	obj.Obj.InventoryDigest = hex.EncodeToString(h.Sum(nil))

	return (*index.IndexingObject)(obj)
}

func quickSum(alg digest.Alg, cont string) []byte {
	h := alg.New()
	h.Write([]byte(cont))
	return h.Sum(nil)
}
