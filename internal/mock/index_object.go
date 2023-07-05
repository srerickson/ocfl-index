package mock

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/srerickson/ocfl"
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
	Head       ocfl.VNum
	BigDirSize int
	BigDirName string
}

type IndexingObjectOption func(*indexingObjectConf)

func WithHead(h ocfl.VNum) IndexingObjectOption {
	return func(conf *indexingObjectConf) {
		conf.Head = h
	}
}

func BigDir(name string, size int) IndexingObjectOption {
	return func(conf *indexingObjectConf) {
		conf.BigDirSize = size
		conf.BigDirName = name
	}
}

func NewIndexingObject(id string, opts ...IndexingObjectOption) *IndexingObject {
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
	obj.Inventory = mockInventory(id, conf.Head, conf.BigDirName, conf.BigDirSize)
	// index.ModeFileSizes is left
	// fake sizes for every manifest entry
	obj.FileSizes = map[string]int64{}
	obj.Inventory.Manifest.EachPath(func(n, _ string) error {
		obj.FileSizes[n] = int64(len(n))
		return nil
	})
	return obj
}

func mockInventory(id string, head ocfl.VNum, bigname string, bigsize int) *ocflv1.Inventory {
	alg := digest.SHA512()
	created := time.Date(2001, 1, 1, 1, 1, 1, 0, time.UTC)
	user := ocflv1.User{Name: "nobody", Address: "email:none@none.com"}
	// file content is unique to the object id
	commonSum := quickDigestSet(alg, id+"common")
	renameSum := quickDigestSet(alg, id+"rename")
	var inv *ocflv1.Inventory
	for i := 0; i < head.Num(); i++ {
		v := "v" + strconv.Itoa(i+1)
		created = created.AddDate(0, 0, 1)
		stage, err := ocfl.NewStage(alg, digest.Map{})
		if err != nil {
			panic(err)
		}
		// a common file for all versions, using v1 content
		commonsrc := ""
		if i == 0 {
			commonsrc = "common.txt"
		}
		stage.UnsafeAddPathAs(commonsrc, "common.txt", commonSum)
		// a renamed file in every version, using v1 content
		renamesrc := ""
		if i == 0 {
			renamesrc = "rename.txt"
		}
		stage.UnsafeAddPathAs(renamesrc, v+"-rename.txt", renameSum)
		// a uniqe file for every version
		stage.UnsafeAddPath(v+"-new.txt", quickDigestSet(alg, id+v+"new"))
		// a file that is changed in every version
		stage.UnsafeAddPath("change.txt", quickDigestSet(alg, id+v+"change"))

		stage.State()
		// big directory
		for i := 0; i < bigsize; i++ {
			name := fmt.Sprintf("%s/%d-file.txt", bigname, i)
			stage.UnsafeAddPath(name, quickDigestSet(alg, name))
		}
		if i == 0 {
			inv = &ocflv1.Inventory{ID: id, Type: ocfl.Spec{1, 1}.AsInvType()}
		}
		if err := inv.AddVersion(stage, v, &user, created, nil); err != nil {
			panic(err)
		}
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

func quickDigestSet(alg digest.Alg, cont string) digest.Set {
	h := alg.New()
	h.Write([]byte(cont))
	dig := hex.EncodeToString(h.Sum(nil))
	return digest.Set{alg.ID(): dig}
}
