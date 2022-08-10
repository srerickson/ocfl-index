/*
Copyright © 2022

*/
package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/muesli/coral"
	"github.com/srerickson/ocfl/object"
)

const defaultVersion = "HEAD"

type queryConfig struct {
	objectID   string
	path       string
	version    string
	vnum       object.VNum
	jsonOutput bool
}

var queryFlags queryConfig

// queryCmd represents the query command
var queryCmd = &coral.Command{
	Use:   "query [object-id] [path]",
	Short: "query the index",
	Long: `The query command queries an existing index. The path should be a relative
path (using '/' as a separator) referencing a file or directory in the
object's logical state. Without any arguments, query lists all objects in the
index. If only the object-id is specified, the object's versions are listed.
With both an object-id and path, query prints information about the given
path: for files, the manifest entry for the corresponding content is
returned; for directories, the directing listing is returned.`,
	Run: func(cmd *coral.Command, args []string) {
		if len(args) > 0 {
			queryFlags.objectID = args[0]
		}
		if len(args) > 1 {
			queryFlags.path = args[1]
		}
		if queryFlags.version != defaultVersion {
			err := object.ParseVNum(queryFlags.version, &queryFlags.vnum)
			if err != nil {
				log.Fatal(err)
			}
		}
		err := DoQuery(cmd.Context(), dbName, &queryFlags)
		if err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringVarP(&queryFlags.version, "ver", "v", defaultVersion, "version to query")
	queryCmd.Flags().BoolVar(&queryFlags.jsonOutput, "json", false, "json output")
}

func DoQuery(ctx context.Context, dbName string, c *queryConfig) error {
	db, err := sql.Open("sqlite", "file:"+dbName)
	if err != nil {
		return err
	}
	defer db.Close()
	idx, err := prepareIndex(ctx, db)
	if err != nil {
		return err
	}
	if c.objectID == "" && c.path == "" {
		// list all objects
		objRes, err := idx.AllObjects(ctx)
		if err != nil {
			return err
		}
		if c.jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent(``, ` `)
			return enc.Encode(objRes)
		}
		for _, o := range objRes.Objects {
			fmt.Printf("%s, %s, %s\n", o.ID, o.Head, o.HeadCreated.UTC().Truncate(time.Second))
		}
		return nil
	}
	verRes, err := idx.GetVersions(ctx, c.objectID)
	if err != nil {
		return err
	}
	if len(verRes.Versions) == 0 {
		return fmt.Errorf("object has not versions")
	}
	// list versions in object
	if c.path == "" {
		if c.jsonOutput {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent(``, ` `)
			return enc.Encode(verRes)
		}
		for _, v := range verRes.Versions {
			fmt.Println(v.Num, v.Created)
		}
		return nil
	}
	// list contents of vnum/path
	if c.vnum.Empty() {
		c.vnum = verRes.Versions[len(verRes.Versions)-1].Num
	}
	contRes, err := idx.GetContent(ctx, c.objectID, c.vnum, c.path)
	if err != nil {
		return err
	}
	if c.jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent(``, ` `)
		return enc.Encode(contRes)
	}
	if !contRes.Content.IsDir {
		fmt.Println(contRes.Content.ContentPath)
		return nil
	}
	for _, c := range contRes.Content.Children {
		if c.IsDir {
			fmt.Println(c.Name + "/")
			continue
		}
		fmt.Println(c.Name)
	}
	return nil
}
