package ocfltest

import (
	"context"
	"fmt"
	"log"
	"math/rand"
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
	Layout       extensions.Layout
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
	if err := ocflv1.InitStore(ctx, fsys, dir, nil); err != nil {
		return err
	}
	store, err := ocflv1.GetStore(ctx, fsys, dir)
	if err != nil {
		return err
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
		inv := GenInv(invconf)
		invPath, err := store.ResolveID(invconf.ID)
		if err != nil {
			return err
		}
		invPath = path.Join(dir, invPath)
		err = ocflv1.WriteInventory(ctx, fsys, inv, invPath)
		if err != nil {
			return err
		}
		decl := ocfl.Declaration{Type: ocfl.DeclObject, Version: ocfl.Spec{1, 0}}
		err = ocfl.WriteDeclaration(ctx, fsys, invPath, decl)
		if err != nil {
			return err
		}
	}
	log.Println("done")
	return nil
}
