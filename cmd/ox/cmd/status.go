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
		Use:   "status",
		Short: "print summary info about the index and its storage root",
		Long:  `print summary info about the index and its storage root`,
		Run: func(c *cobra.Command, a []string) {
			if err := root.Status(c.Context()); err != nil {
				log.Fatal(err)
			}
		},
	})
}

func (ox *oxCmd) Status(ctx context.Context) error {
	client := ox.serviceClient()
	req := connect.NewRequest(&ocflv0.GetSummaryRequest{})
	resp, err := client.GetSummary(ctx, req)
	if err != nil {
		return err
	}
	lastIndexed := "never"
	if resp.Msg.IndexedAt.IsValid() {
		lastIndexed = resp.Msg.IndexedAt.AsTime().Format(time.RFC3339)
	}
	// TODO: different format options
	fmt.Println("OCFL spec:", resp.Msg.Spec)
	fmt.Println("description:", resp.Msg.Description)
	fmt.Println("indexed objects:", resp.Msg.NumObjects)
	fmt.Println("last indexed:", lastIndexed)
	return nil
}
