package reindex

import (
	"context"
	"log"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/root"
	ocflv0 "github.com/srerickson/ocfl-index/gen/ocfl/v0"
)

type Cmd struct {
	root *root.Cmd
}

func (idx *Cmd) NewCommand(r *root.Cmd) *cobra.Command {
	idx.root = r
	cmd := &cobra.Command{
		Use:   `reindex`,
		Short: "reindex",
		Long:  "reindex",
	}
	return cmd
}

// ParseArgs is always run before Run
func (ls *Cmd) ParseArgs(args []string) error {
	return nil
}

func (ls *Cmd) Run(ctx context.Context, args []string) error {
	client := ls.root.ServiceClient()
	// without an object id, list object ids in the index
	req := connect.NewRequest(&ocflv0.ReindexRequest{})
	stream, err := client.Reindex(ctx, req)
	if err != nil {
		return err
	}
	for stream.Receive() {
		msg := stream.Msg()
		log.Println(msg.LogMessage)
	}
	if err := stream.Err(); err != nil {
		return err
	}

	return nil
}
