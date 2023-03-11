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
	root     *root.Cmd
	logs     bool
	objectID string
}

func (idx *Cmd) NewCommand(r *root.Cmd) *cobra.Command {
	idx.root = r
	cmd := &cobra.Command{
		Use:   `reindex`,
		Short: "reindex",
		Long:  "reindex",
	}
	cmd.Flags().StringVar(&idx.objectID, "id", "", "reindex the given object ID only")
	cmd.Flags().BoolVar(&idx.logs, "logs", false, "follow logs of an existing reindexing process")
	return cmd
}

// ParseArgs is always run before Run
func (ls *Cmd) ParseArgs(args []string) error {
	return nil
}

func (idx *Cmd) Run(ctx context.Context, args []string) error {
	client := idx.root.ServiceClient()
	rq := &ocflv0.ReindexRequest{
		Op: ocflv0.ReindexRequest_OP_REINDEX_ALL,
	}
	if idx.logs {
		rq.Op = ocflv0.ReindexRequest_OP_FOLLOW_LOGS
	}
	if idx.objectID != "" {
		rq.Op = ocflv0.ReindexRequest_OP_REINDEX_IDS
		rq.Args = []string{idx.objectID}
	}
	// without an object id, list object ids in the index
	stream, err := client.Reindex(ctx, connect.NewRequest(rq))
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
