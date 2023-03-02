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
	obj.Inventory = mockInventory(id, conf.Head, conf.BigDirName, conf.BigDirSize)
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
		stage := ocfl.NewStage(alg)
		var err error
		// a common file for all versions, using v1 content
		commonsrc := ""
		if i == 0 {
			commonsrc = "common.txt"
		}
		stage.UnsafeAdd("common.txt", commonsrc, commonSum)
		// a renamed file in every version, using v1 content
		renamesrc := ""
		if i == 0 {
			renamesrc = "rename.txt"
		}
		stage.UnsafeAdd(v+"-rename.txt", renamesrc, renameSum)
		// a uniqe file for every version
		stage.UnsafeAdd(v+"-new.txt", v+"-new.txt", quickDigestSet(alg, id+v+"new"))
		// a file that is changed in every version
		stage.UnsafeAdd("change.txt", "change.txt", quickDigestSet(alg, id+v+"change"))

		// big directory
		for i := 0; i < bigsize; i++ {
			name := fmt.Sprintf("%s/%d-file.txt", bigname, i)
			stage.UnsafeAdd(name, name, quickDigestSet(alg, name))
		}

		if i == 0 {
			inv, err = ocflv1.NewInventory(stage, id, "content", 0, created, v, &user)
			if err != nil {
				panic(err)
			}
			continue
		}
		inv, err = inv.NextVersionInventory(stage, created, v, &user)
		if err != nil {
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
