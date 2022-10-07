// Package ocfltest provides functions for generating dummy inventories with
// various properties.
package ocfltest

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"path"
	"strings"
	"time"

	"github.com/srerickson/ocfl"
	"github.com/srerickson/ocfl/digest"
	"github.com/srerickson/ocfl/ocflv1"
)

type GenInvConf struct {
	ID       string
	Head     ocfl.VNum
	Numfiles int
	Add      float64
	Mod      float64
	Del      float64
}

func GenInv(conf *GenInvConf) *ocflv1.Inventory {
	inv := &ocflv1.Inventory{
		Type:            ocfl.MustParseSpec("1.0").AsInvType(),
		ID:              conf.ID,
		Head:            conf.Head,
		DigestAlgorithm: digest.SHA256,
		Manifest:        digest.NewMap(),
		Versions:        map[ocfl.VNum]*ocflv1.Version{},
	}
	state := randomState(conf.Numfiles)
	vnum := ocfl.Head
	for vnum.Num() < conf.Head.Num() {
		vnum, _ = vnum.Next()
		inv.Versions[vnum] = &ocflv1.Version{
			Created: time.Now(),
			Message: fmt.Sprintf("commit %d", vnum.Num()),
		}
		if vnum.Num() > 1 {
			state = randomModifyState(state, conf.Del, conf.Mod, conf.Add)
		}
		inv.Versions[vnum].State = state
		for dig := range state.AllDigests() {
			if inv.Manifest.DigestExists(dig) {
				continue
			}
			p := state.DigestPaths(dig)[0]
			p = path.Join(vnum.String(), `content`, p)
			if err := inv.Manifest.Add(dig, p); err != nil {
				panic(err)
			}
		}
	}
	return inv
}

// randomModifyState returns new digest.Map based on dm; the del, mod, and add
// arguments represent proportions to delete, modify, and add.
func randomModifyState(dm *digest.Map, del, mod, add float64) *digest.Map {
	addNum := int(float64(len(dm.AllPaths())) * add)
	newMap := digest.NewMap()
	_, keepMap := sampleDigestMap(dm, del)
	modMap, _ := sampleDigestMap(keepMap, mod)
	for p := range modMap.AllPaths() {
		s := make([]byte, 32)
		rand.Read(s)
		hex.EncodeToString(s)
		if err := newMap.Add(hex.EncodeToString(s), p); err != nil {
			panic(err)
		}
	}
	for p, d := range keepMap.AllPaths() {
		newMap.Add(d, p)
	}
	for p, d := range randomState(addNum).AllPaths() {
		newMap.Add(d, p)
	}
	return newMap
}

// randomState returns digest.Map with num files with random paths
func randomState(num int) *digest.Map {
	dm := digest.NewMap()
	prevDir := "."
	prevSum := ""
	for i := 0; i < num; i++ {
		var p string // new path
		// reuse dir
		if rand.Float64() < 0.25 {
			p = randomPath(4, 10)
		} else {
			p = path.Join(prevDir, randomPath(2, 10))
		}
		// reuse sum (5%)
		if rand.Float64() > 0.05 || prevSum == "" {
			sum := make([]byte, 32)
			rand.Read(sum)
			prevSum = hex.EncodeToString(sum)
		}
		for j := 0; j < 5; j++ {
			err := dm.Add(prevSum, p)
			if err == nil {
				break
			}
		}
		prevDir = path.Dir(p)
	}
	return dm
}

func sampleDigestMap(dm *digest.Map, ratio float64) (*digest.Map, *digest.Map) {
	samp := digest.NewMap()
	remain := digest.NewMap()
	pathDigest := dm.AllPaths()
	allPaths := make([]string, len(pathDigest))
	i := 0
	for p := range pathDigest {
		allPaths[i] = p
		i += 1
	}
	size := int(float64(len(allPaths)) * ratio)
	for i := 0; i < size; i++ {
		r := rand.Intn(len(allPaths))
		p := allPaths[r]
		d := pathDigest[p]
		if err := samp.Add(d, p); err != nil {
			panic(err)
		}
		allPaths = append(allPaths[:r], allPaths[r+1:]...)
	}
	for _, p := range allPaths {
		d := pathDigest[p]
		if err := remain.Add(d, p); err != nil {
			panic(err)
		}
	}
	return samp, remain
}

// returns a slice of random strings, joined as a path
func randomPath(depth, nameLen int) string {
	depth = rand.Intn(depth) + 1
	parts := make([]string, depth)
	for i := range parts {
		parts[i] = randomName(nameLen)
	}
	return strings.Join(parts, "/")
}

func randomName(l int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789_")
	size := rand.Intn(l) + 1
	part := make([]rune, size)
	for j := range part {
		part[j] = letters[rand.Intn(len(letters))]
	}
	return string(part)
}
