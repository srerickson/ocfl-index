/*
Copyright © 2022

*/
package cmd

import (
	"context"
	"database/sql"
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
	Short: "benchmark indexing with generated inventories",
	Long: `The benchmark command generates inventories, indexes them, and measures
	the average time to complete the indexing process. In the process, one
	inventory is randomly selected for querying; all paths in all versions of
	the inventory are queried and the average time is measured. You can
	specify the number of inventories to generate with the'num' flag and the
	number of files in each inventory (version) with the 'size' flag.
	Generated inventories have between 1 and 4 versions, with small
	additions, modifications, and deletions between each version.`,
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

func doBenchmark(ctx context.Context, dbName string, numinv int, size int) error {
	db, err := sql.Open("sqlite", "file:"+dbName)
	if err != nil {
		return err
	}
	defer db.Close()
	idx, err := prepareIndex(ctx, db)
	if err != nil {
		return err
	}
	fmt.Printf("indexing %d generated inventories (1-4 versions, %d files/version)\n", numinv, size)
	rand.Seed(time.Now().UnixNano())
	sampleInvN := rand.Intn(numinv) // inventory to query later
	totalTimer := time.Now()
	var sampleInv *ocflv1.Inventory
	var timer, avgTime float64
	var i int
	for i = 0; i < numinv; i++ {
		inv := testinv.GenInv(&testinv.GenInvConf{
			ID:       fmt.Sprintf("http://test-object-%d", i),
			Head:     object.V(rand.Intn(4) + 1),
			Numfiles: size,
			Del:      .05, // delete .05 of files with each version
			Mod:      .05, // modify .05 of files remaining after delete
			Add:      .05, // add .05 new random files
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
		fmt.Printf("\rindexed %d/%d (%.2f sec/op avg)", i+1, numinv, avgTime)
		if err := inv.Validate().Err(); err != nil {
			return err
		}
	}
	fmt.Println()
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
	fmt.Printf("queried %d paths (%.4f sec/op avg)\n", i, avgTime)
	fmt.Printf("benchmark complete in %.1f sec\n", time.Since(totalTimer).Seconds())
	return nil
}
