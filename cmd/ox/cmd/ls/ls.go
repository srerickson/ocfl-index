package ls

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/root"
	ocflv1 "github.com/srerickson/ocfl-index/gen/ocfl/v1"
)

type Cmd struct {
	root      *root.Cmd
	objectID  string
	dir       string
	version   string
	recursive bool
	versions  bool
	reindex   bool
}

func (ls *Cmd) NewCommand(r *root.Cmd) *cobra.Command {
	ls.root = r
	cmd := &cobra.Command{
		Use:               `ls [(--recursive | -r)] [--versions] [(--version= | -V) {""}] [object_id] [dir]`,
		Short:             "list objects, object versions, or object contents",
		Long:              "Without any arguments, ls lists all object IDs in the index. If an object ID is given, ls lists files from the head version state. A path may be given as an optional second argument to specifiy the directory of files to list. With the --versions flag, ls lists the object's versions.",
		ValidArgsFunction: ls.ValidArgsFunction(),
	}
	cmd.Flags().BoolVarP(&ls.recursive, "recursive", "r", false, "list files within a directory recursively")
	cmd.Flags().BoolVar(&ls.reindex, "reindex", false, "reindex the object's inventory before listing (requires object id)")
	cmd.Flags().BoolVar(&ls.versions, "versions", false, "list an object's versions instead of its files")
	cmd.Flags().StringVarP(&ls.version, "version", "V", "", "use the specified object version (default value refers to HEAD)")
	return cmd
}

// ParseArgs is always run before Run
func (ls *Cmd) ParseArgs(args []string) error {
	if len(args) > 0 {
		ls.objectID = args[0]
	}
	if len(args) > 1 {
		ls.dir = args[1]
	}
	return nil
}

func (ls *Cmd) Run(ctx context.Context, args []string) error {
	client := ls.root.ServiceClient()
	// without an object id, list object ids in the index
	if ls.objectID == "" {
		return ls.listObjects(ctx)
	}
	if ls.reindex {
		// first reindex the object
		if err := ls.doReindex(ctx, ls.objectID); err != nil {
			return fmt.Errorf("while reindexing: %w", err)
		}
	}
	// if versions flag is set, list an object's versions
	if ls.versions {
		return ls.listObjectVersions(ctx)
	}
	cursor := ""
	for {
		req := connect.NewRequest(&ocflv1.GetObjectStateRequest{
			ObjectId:  ls.objectID,
			Version:   ls.version,
			BasePath:  ls.dir,
			Recursive: ls.recursive,
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

// Tab completion
func (ls Cmd) ValidArgsFunction() func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		var comps []string
		if len(args) == 1 {
			// return directory list matches
			comps, _ = ls.listEntriesPrefix(cmd.Context(), args[0], toComplete)
		}
		return comps, cobra.ShellCompDirectiveNoFileComp
	}
}

func (ls Cmd) listObjects(ctx context.Context) error {
	client := ls.root.ServiceClient()
	cursor := ""
	for {
		req := connect.NewRequest(&ocflv1.ListObjectsRequest{
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
		if resp.Msg.NextPageToken == "" {
			break
		}
		cursor = resp.Msg.NextPageToken
	}
	return nil
}

func (ls Cmd) listObjectVersions(ctx context.Context) error {
	client := ls.root.ServiceClient()
	req := connect.NewRequest(&ocflv1.GetObjectRequest{ObjectId: ls.objectID})
	resp, err := client.GetObject(ctx, req)
	if err != nil {
		return err
	}
	for _, v := range resp.Msg.Versions {
		fmt.Println(v.Num, v.Message, v.Created.AsTime().Local().Format(time.RFC822))
	}
	return nil
}

// used for tab completion
func (ls Cmd) listEntriesPrefix(ctx context.Context, id string, prefix string) ([]string, error) {
	client := ls.root.ServiceClient()
	var entries []string
	dir := path.Dir(prefix)
	if path.IsAbs(dir) {
		return nil, errors.New("invalid path")
	}
	if prefix == "." {
		prefix = ""
	}
	cursor := ""
	for {
		req := connect.NewRequest(&ocflv1.GetObjectStateRequest{
			ObjectId:  id,
			Version:   ls.version,
			BasePath:  dir,
			PageToken: cursor,
			PageSize:  1000,
		})
		resp, err := client.GetObjectState(ctx, req)
		if err != nil {
			return nil, err
		}
		if !resp.Msg.Isdir {
			fmt.Println(resp.Msg.Digest)
			return nil, nil
		}
		for _, child := range resp.Msg.Children {
			p := path.Join(dir, child.Name)
			if !strings.HasPrefix(p, prefix) {
				continue
			}
			if child.Isdir {
				p += "/"
			}
			entries = append(entries, p)
		}
		if resp.Msg.NextPageToken == "" {
			break
		}
		cursor = resp.Msg.NextPageToken
	}
	return entries, nil
}

func (ls Cmd) doReindex(ctx context.Context, id string) error {
	client := ls.root.ServiceClient()
	rq := &ocflv1.IndexIDsRequest{
		ObjectIds: []string{id},
	}
	// without an object id, list object ids in the index
	_, err := client.IndexIDs(ctx, connect.NewRequest(rq))
	if err != nil {
		return err
	}
	return nil
}
