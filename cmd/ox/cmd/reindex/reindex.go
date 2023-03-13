package reindex

import (
	"context"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/root"
	ocflv1 "github.com/srerickson/ocfl-index/gen/ocfl/v1"
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

	if idx.logs {
		return idx.root.FollowLogs(ctx)
	}
	if idx.objectID != "" {
		rq := ocflv1.IndexIDsRequest{
			ObjectIds: []string{idx.objectID},
		}
		_, err := client.IndexIDs(ctx, connect.NewRequest(&rq))
		return err
	}
	// without an object id, list object ids in the index
	rq := ocflv1.IndexAllRequest{}
	_, err := client.IndexAll(ctx, connect.NewRequest(&rq))
	return err
}
