/*
Copyright (c) 2022 The Regents of the University of California.
*/
package status

import (
	"context"
	"fmt"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/root"
	ocflv1 "github.com/srerickson/ocfl-index/gen/ocfl/v1"
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
	req := connect.NewRequest(&ocflv1.GetStatusRequest{})
	resp, err := client.GetStatus(ctx, req)
	if err != nil {
		return err
	}
	// TODO: different format options
	fmt.Println("indexer status:", resp.Msg.Status)
	fmt.Println("# found objects:", resp.Msg.NumObjectPaths)
	fmt.Println("# indexed inventories:", resp.Msg.NumInventories)
	fmt.Println("storage root OCFL spec:", resp.Msg.StoreSpec)
	fmt.Println("storage root description:", resp.Msg.StoreDescription)
	fmt.Println("storage root path:", resp.Msg.StoreRootPath)
	return nil
}
