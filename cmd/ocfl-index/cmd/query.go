/*
Copyright © 2022

*/
package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/muesli/coral"
	"github.com/srerickson/ocfl/object"
)

var queryFlags = struct {
	version string
}{}

// queryCmd represents the query command
var queryCmd = &coral.Command{
	Use:   "query [object-id] [path]",
	Short: "query the index",
	Long: `The query command queries an existing index. The path should be a
	relative path (using '/' as a separator) referencing a file or directory
	in the object. Without any arguments, query lists all objects in the
	index. If only the object-id is specified, the object's versions are
	listed. With both an object-id and path, query prints information about
	the given path: for files, the manifest entry for the corresponding
	content is returned; for directories, the directing listing is returned.`,
	Run: func(cmd *coral.Command, args []string) {
		err := DoQuery(cmd.Context(), dbName, args)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringVarP(&queryFlags.version, "ver", "v", "HEAD", "version to query")
}

func DoQuery(ctx context.Context, dbName string, args []string) error {
	db, err := sql.Open("sqlite", "file:"+dbName)
	if err != nil {
		return err
	}
	defer db.Close()
	idx, err := prepareIndex(ctx, db)
	if err != nil {
		return err
	}
	defer idx.Close()
	if len(args) == 0 {
		objs, err := idx.AllObjects(ctx)
		if err != nil {
			return err
		}
		for _, o := range objs {
			fmt.Println(o.ID, o.Head, o.HeadCreated)
		}
		return nil
	}
	objID := args[0]
	vers, err := idx.GetVersions(ctx, objID)
	if err != nil {
		return err
	}
	if len(vers) == 0 {
		fmt.Println(objID, "has no versions!")
	}
	// list versions in object
	if len(args) == 1 {
		for _, v := range vers {
			fmt.Println(v.Num, v.Created)
		}
		return nil
	}
	// list contents of vnum/path
	lpath := args[1]
	vnum := vers[len(vers)-1].Num
	if !strings.EqualFold("head", queryFlags.version) {
		err = object.ParseVNum(queryFlags.version, &vnum)
		if err != nil {
			return fmt.Errorf("%s: %w", queryFlags.version, err)
		}
	}
	cont, err := idx.GetContent(ctx, objID, vnum, lpath)
	if err != nil {
		return err
	}
	if !cont.IsDir {
		fmt.Println(cont.ContentPath)
		return nil
	}
	for _, c := range cont.Children {
		fmt.Println(c.Name, c.IsDir)
	}
	return nil
}
