/*
Copyright (c) 2022 The Regents of the University of California.
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	ocflv0 "github.com/srerickson/ocfl-index/gen/ocfl/v0"
)

func init() {
	root.cmd.AddCommand(&cobra.Command{
		Use:   "objects",
		Short: "list indexed objects",
		Long:  `list indexed objects`,
		Run: func(c *cobra.Command, a []string) {
			if err := root.Objects(c.Context()); err != nil {
				log.Fatal(err)
			}
		},
	})
}

func (ox *oxCmd) Objects(ctx context.Context) error {
	client := ox.serviceClient()
	cursor := ""
	for {
		req := connect.NewRequest(&ocflv0.ListObjectsRequest{
			PageToken: cursor,
			PageSize:  1000,
		})
		resp, err := client.ListObjects(ctx, req)
		if err != nil {
			return err
		}
		objects := resp.Msg.Objects
		if len(objects) == 0 {
			break
		}
		for _, obj := range objects {
			// TODO: formatting
			fmt.Println(obj.ObjectId, obj.Head, obj.GetHeadCreated().AsTime().Format(time.RFC3339))
		}
		// FIXME: only the token check should be necessary here
		if len(objects) == 0 || resp.Msg.NextPageToken == "" {
			break
		}
		cursor = resp.Msg.NextPageToken
	}
	return nil
}
