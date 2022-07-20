/*
Copyright © 2022

*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/muesli/coral"
	"github.com/srerickson/ocfl-index/internal/testinv"
	"github.com/srerickson/ocfl/object"
	"github.com/srerickson/ocfl/ocflv1"
)

var benchmarkFlags = struct {
	file string
	num  int // number of inventories
	size int // size of iventories
}{}

// benchmarkCmd represents the benchmark command
var benchmarkCmd = &coral.Command{
	Use:   "benchmark",
	Short: "benchmarks indexing with generated inventories",
	Long:  ``,
	Run: func(cmd *coral.Command, args []string) {
		if strings.Index(benchmarkFlags.file, "%d") > 0 {
			benchmarkFlags.file = fmt.Sprintf(benchmarkFlags.file, time.Now().Unix())
		}
		err := doBenchmark(cmd.Context(), benchmarkFlags.file, benchmarkFlags.num, benchmarkFlags.size)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)
	benchmarkCmd.Flags().IntVar(&benchmarkFlags.size, "size", 100, "the number of files in generated inventories (each version)")
	benchmarkCmd.Flags().IntVar(&benchmarkFlags.num, "num", 100, "number of inventories to index")
	benchmarkCmd.Flags().StringVarP(&benchmarkFlags.file, "file", "f", "benchmark-%d.sqlite", "complete index sqlite file")
}

func doBenchmark(ctx context.Context, fname string, numinv int, size int) error {
	idx, err := openIndex(ctx, fname)
	if err != nil {
		return err
	}
	fmt.Printf("indexing %d generated inventories (1-3 versions, %d files/version)\n", numinv, size)
	sampleInvN := rand.Intn(numinv) + 1
	var sampleInv *ocflv1.Inventory
	var timer, avgTime float64
	var i int
	for i = 0; i < numinv; i++ {
		inv := testinv.GenInv(&testinv.GenInvConf{
			ID:       fmt.Sprintf("test-%d", i),
			Head:     object.V(rand.Intn(2) + 1),
			Numfiles: size,
			Del:      .05, // delete .05 of files with each version
			Add:      .05, // modify .05 of files remaining
			Mod:      .05, // add .05 new random file
		})
		if i == sampleInvN {
			sampleInv = inv
		}
		indexStart := time.Now()
		if err := idx.IndexInventory(ctx, inv); err != nil {
			return err
		}
		timer += time.Since(indexStart).Seconds()
		avgTime = timer / float64(i+1)
		fmt.Printf("\rindexed %d/%d in %02f sec. avg", i+1, numinv, avgTime)
		if err := inv.Validate().Err(); err != nil {
			return err
		}
	}
	fmt.Printf("\nbenchmarking index queries ...\n")
	i = 0
	timer = 0
	for _, vnum := range sampleInv.VNums() {
		vstate := sampleInv.VState(vnum)
		for lpath := range vstate.State {
			contentStart := time.Now()
			_, err := idx.GetContent(ctx, sampleInv.ID, vnum, lpath)
			if err != nil {
				return err
			}
			i++
			timer += time.Since(contentStart).Seconds()
			avgTime = timer / float64(i)
		}
	}
	fmt.Printf("queried #%d paths, %02f sec./query\n", i, avgTime)
	return nil
}
