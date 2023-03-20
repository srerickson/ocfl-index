package ls

import (
	"context"
	"errors"
	"fmt"
	"io"
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
		switch len(args) {
		case 0:
			// complete object id
			comps, _ = ls.completeObjectIDsPrefix(cmd.Context(), toComplete)
		case 1:
			// complete path
			objectID := args[0]
			comps, _ = ls.completePathPrefix(cmd.Context(), objectID, toComplete)
		}
		return comps, cobra.ShellCompDirectiveNoFileComp
	}
}

func (ls Cmd) listObjects(ctx context.Context) error {
	iter := ls.root.ListObjects("", 1000)
	for {
		obj, err := iter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		fmt.Println(obj.ID, obj.Head, obj.HeadCreated.Format(time.RFC822))
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

// used for tab completion of object id
func (ls Cmd) completeObjectIDsPrefix(ctx context.Context, prefix string) ([]string, error) {
	cli := ls.root.ServiceClient()
	req := ocflv1.ListObjectsRequest{
		PageSize: 1000,
		IdPrefix: prefix,
	}
	resp, err := cli.ListObjects(ctx, connect.NewRequest(&req))
	if err != nil {
		return nil, err
	}
	// if there are more than a thousand entries, no completion
	if resp.Msg.NextPageToken != "" {
		return nil, nil
	}
	ids := make([]string, len(resp.Msg.Objects))
	for i, obj := range resp.Msg.Objects {
		ids[i] = obj.ObjectId
	}
	return ids, nil
}

// used for tab completion of path argument
func (ls Cmd) completePathPrefix(ctx context.Context, id string, prefix string) ([]string, error) {
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

func (ls Cmd) doReindex(ctx context.Context, ids ...string) error {
	client := ls.root.ServiceClient()
	rq := &ocflv1.IndexIDsRequest{
		ObjectIds: ids,
	}
	// without an object id, list object ids in the index
	_, err := client.IndexIDs(ctx, connect.NewRequest(rq))
	if err != nil {
		return err
	}
	return nil
}
