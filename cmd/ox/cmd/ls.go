/*
Copyright (c) 2022 The Regents of the University of California.
*/
package cmd

import (
	"context"
	"fmt"
	"log"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	ocflv0 "github.com/srerickson/ocfl-index/gen/ocfl/v0"
)

func init() {
	lsCmd := &cobra.Command{
		Use:   "ls",
		Short: "query paths in an object logical state",
		Long:  `query paths in an object logical state`,
		Args:  cobra.MinimumNArgs(1),
		Run: func(c *cobra.Command, args []string) {
			basePath := "."
			if len(args) > 1 {
				basePath = args[1]
			}
			id := args[0]
			v := root.commonFlags.vnum
			if err := root.LS(c.Context(), id, v, basePath); err != nil {
				log.Fatal(err)
			}
		},
	}
	lsCmd.Flags().StringVar(&root.commonFlags.vnum, "version", "", "object version")
	root.cmd.AddCommand(lsCmd)
}

func (ox *oxCmd) LS(ctx context.Context, id string, v string, p string) error {
	client := ox.serviceClient()
	cursor := ""
	for {
		req := connect.NewRequest(&ocflv0.GetObjectStateRequest{
			ObjectId:  id,
			Version:   v,
			BasePath:  p,
			PageToken: cursor,
			PageSize:  1000,
		})
		resp, err := client.GetObjectState(ctx, req)
		if err != nil {
			return err
		}
		if !resp.Msg.Isdir {
			fmt.Println(resp.Msg.Digest)
			return nil
		}
		for _, child := range resp.Msg.Children {
			n := child.Name
			if child.Isdir {
				n += "/"
			}
			fmt.Printf("[%s] %d %s\n", child.Digest[0:8], child.Size, n)
		}
		if resp.Msg.NextPageToken == "" {
			break
		}
		cursor = resp.Msg.NextPageToken
	}
	return nil
}
