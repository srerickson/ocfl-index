package mock

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/url"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl-index/internal/index"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

type IndexingObject struct {
	RootDir   string // object root path relative to FS
	Inventory *ocflv1.Inventory
	FileSizes map[string]int64 // content path -> size
	IndexedAt time.Time
}

type indexingObjectConf struct {
	Head ocfl.VNum
}

type IndexingObjectOption func(*indexingObjectConf)

func WithHead(h ocfl.VNum) IndexingObjectOption {
	return func(conf *indexingObjectConf) {
		conf.Head = h
	}
}

func NewIndexingObject(id string, mode index.IndexMode, opts ...IndexingObjectOption) *IndexingObject {
	conf := indexingObjectConf{
		Head: ocfl.V(1),
	}
	for _, o := range opts {
		o(&conf)
	}
	obj := &IndexingObject{
		RootDir:   url.PathEscape(id),
		IndexedAt: time.Now(),
	}
	if mode == index.ModeObjectDirs {
		return obj
	}
	obj.Inventory = mockInventory(id, conf.Head)
	if mode == index.ModeInventories {
		return obj
	}
	// index.ModeFileSizes is left
	// fake sizes for every manifest entry
	obj.FileSizes = map[string]int64{}
	obj.Inventory.Manifest.EachPath(func(n, _ string) error {
		obj.FileSizes[n] = int64(len(n))
		return nil
	})
	return obj
}

func mockInventory(id string, head ocfl.VNum) *ocflv1.Inventory {
	alg := digest.SHA512()

	inv := &ocflv1.Inventory{
		ID:               id,
		Type:             ocfl.Spec{1, 1}.AsInvType(),
		Head:             head,
		DigestAlgorithm:  alg.ID(),
		ContentDirectory: "content",
		Versions:         make(map[ocfl.VNum]*ocflv1.Version),
		Manifest:         digest.NewMap(),
	}
	// generate contents
	// for 3 versions, we want:
	// v1:
	// 	- common.txt:    v1/content/common.txt
	//  - change.txt:    v1/content/change.txt
	//  - v1-new.txt: 	 v1/content/v1-new.txt
	//  - v1-rename.txt: v1/content/v1-rename.txt
	// 	v2:
	// 	- common.txt:    v1/content/common.txt
	//  - change.txt:    v2/content/change.txt
	//  - v2-new.txt: 	 v2/content/v2-new.txt
	// 	- v2-rename.txt: v1/content/v1-rename.txt
	// 	v3:
	// 	- common.txt:    v1/content/common.txt
	//  - change.txt:    v3/content/change.txt
	//  - v3-new.txt: 	 v3/content/v3-new.txt
	// 	- v3-rename.txt: v1/content/v1-rename.txt
	common := quickSumStr(alg, id+"common file")
	rename := quickSumStr(alg, id+"rename file")
	inv.Manifest.Add(rename, "v1/content/rename.txt")
	inv.Manifest.Add(common, "v1/content/common.txt")
	created := time.Date(2001, 1, 1, 1, 1, 1, 0, time.UTC)
	for _, vnum := range inv.Head.VNumSeq() {
		ver := &ocflv1.Version{
			Message: "commit version " + vnum.String(),
			User:    &ocflv1.User{Name: "nobody", Address: "email:none@none.com"},
			Created: created,
			State:   digest.NewMap(),
		}
		// Add files to state and manifest
		v := vnum.String()
		// common in very version
		ver.State.Add(common, "a/common.txt")
		// changes every version
		change := quickSumStr(alg, id+"change "+v)
		ver.State.Add(change, "a/b/change.txt")
		inv.Manifest.Add(change, v+"/content/a/b/change.txt")
		// only in one verion
		vNew := quickSumStr(alg, id+"new "+vnum.String())
		ver.State.Add(vNew, v+"-new.txt")
		inv.Manifest.Add(vNew, v+"/content/"+v+"-new.txt")
		// renamed in every verion
		ver.State.Add(rename, v+"-rename.txt")
		inv.Versions[vnum] = ver
		created = created.AddDate(0, 0, 1)
	}
	// encode and decode inventory to set digest
	reader, writer := io.Pipe()
	enc := json.NewEncoder(writer)
	errch := make(chan error, 1)
	go func() {
		errch <- enc.Encode(inv)
	}()
	ctx := context.Background()
	newInv, result := ocflv1.ValidateInventoryReader(ctx, reader, alg)
	if err := result.Err(); err != nil {
		panic(err)
	}
	if err := <-errch; err != nil {
		panic(err)
	}
	return newInv
}

func quickSumStr(alg digest.Alg, cont string) string {
	h := alg.New()
	h.Write([]byte(cont))
	return hex.EncodeToString(h.Sum(nil))
}
