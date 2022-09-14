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

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/extensions"
	"github.com/srerickson/ocfl/ocflv1"
)

type GenStoreConf struct {
	InvNumber    int
	InvSize      int
	VNumMax      int
	Layout       *ocflv1.StoreLayout
	LayoutConfig extensions.Extension
}

func GenStore(fsys ocfl.WriteFS, dir string, conf *GenStoreConf) error {
	ctx := context.Background()
	rand.Seed(time.Now().UnixMicro())
	if conf == nil {
		conf = &GenStoreConf{
			InvNumber: 1000,
			InvSize:   5,
			VNumMax:   2,
		}
	}
	dirs, err := fsys.ReadDir(ctx, dir)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if len(dirs) > 0 {
		return fmt.Errorf("%s is not empty", dir)
	}
	decl := ocfl.Declaration{
		Type:    ocfl.DeclStore,
		Version: ocfl.Spec{1, 0},
	}
	if err := ocfl.WriteDeclaration(ctx, fsys, dir, decl); err != nil {
		return err
	}
	if conf.Layout != nil {
		if err := ocflv1.WriteLayout(ctx, fsys, dir, conf.Layout); err != nil {
			return err
		}
	}
	var layoutFunc extensions.LayoutFunc
	if conf.LayoutConfig != nil {
		if err := ocflv1.WriteExtensionConfig(ctx, fsys, dir, conf.LayoutConfig); err != nil {
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
			Head:     ocfl.V(rand.Intn(conf.VNumMax) + 1),
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
		err = ocflv1.WriteInventory(ctx, fsys, invRoot, inv)
		if err != nil {
			return err
		}
		decl := ocfl.Declaration{Type: ocfl.DeclObject, Version: ocfl.Spec{1, 0}}
		err := ocfl.WriteDeclaration(ctx, fsys, invRoot, decl)
		if err != nil {
			return err
		}
	}
	log.Println("done")
	return nil
}
