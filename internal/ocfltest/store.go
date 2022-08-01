package ocfltest

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"net/url"
	"path"
	"time"

	"github.com/srerickson/ocfl/backend"
	"github.com/srerickson/ocfl/extensions"
	"github.com/srerickson/ocfl/namaste"
	"github.com/srerickson/ocfl/object"
	"github.com/srerickson/ocfl/ocflv1"
	"github.com/srerickson/ocfl/spec"
)

type GenStoreConf struct {
	InvNumber    int
	InvSize      int
	VNumMax      int
	Layout       *ocflv1.StoreLayout
	LayoutConfig extensions.Extension
}

func GenStore(fsys backend.Interface, dir string, conf *GenStoreConf) error {
	rand.Seed(time.Now().UnixMicro())
	if conf == nil {
		conf = &GenStoreConf{
			InvNumber: 1000,
			InvSize:   5,
			VNumMax:   2,
		}
	}
	dirs, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if len(dirs) > 0 {
		return fmt.Errorf("%s is not empty", dir)
	}
	err = namaste.Declaration{
		Type:    namaste.StoreType,
		Version: spec.Num{1, 0}}.
		Write(fsys, dir)
	if err != nil {
		return err
	}
	if conf.Layout != nil {
		if err := ocflv1.WriteLayout(fsys, dir, conf.Layout); err != nil {
			return err
		}
	}
	var layoutFunc extensions.LayoutFunc
	if conf.LayoutConfig != nil {
		if err := ocflv1.WriteExtensionConfig(fsys, dir, conf.LayoutConfig); err != nil {
			return err
		}
		if layout, ok := conf.LayoutConfig.(extensions.Layout); ok {
			layoutFunc, err = layout.NewFunc()
			if err != nil {
				return err
			}
		}
	}
	for i := 0; i < conf.InvNumber; i++ {
		id := fmt.Sprintf("http://inventory-%d", i)
		invconf := &GenInvConf{
			ID:       id,
			Head:     object.V(rand.Intn(conf.VNumMax) + 1),
			Numfiles: conf.InvSize,
			Add:      0.1,
			Del:      0.1,
			Mod:      0.1,
		}
		var invRoot string
		if layoutFunc == nil {
			invRoot = path.Join(dir, url.QueryEscape(id))
		} else {
			invRoot, err = layoutFunc(id)
			if err != nil {
				return err
			}
			invRoot = path.Join(dir, invRoot)
		}
		inv := GenInv(invconf)
		err = object.WriteInventory(context.Background(), fsys, invRoot, inv.DigestAlgorithm, &inv)
		if err != nil {
			return err
		}
		err = namaste.Declaration{Type: namaste.ObjectType, Version: spec.Num{1, 0}}.Write(fsys, invRoot)
		if err != nil {
			return err
		}
	}
	log.Println("done")
	return nil
}
