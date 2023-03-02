/*
Copyright (c) 2022 The Regents of the University of California.
*/
package status

import (
	"context"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/root"
	ocflv0 "github.com/srerickson/ocfl-index/gen/ocfl/v0"
)

type Cmd struct {
	root *root.Cmd
}

func (status *Cmd) NewCommand(root *root.Cmd) *cobra.Command {
	status.root = root
	cmd := &cobra.Command{
		Use:   "status",
		Short: "print summary info about the index and its storage root",
		Long:  `print summary info about the index and its storage root`,
	}
	return cmd
}

func (status *Cmd) ParseArgs(args []string) error {
	return nil
}

func (status Cmd) Run(ctx context.Context, args []string) error {
	client := status.root.ServiceClient()
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
