package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/cmd/ox/cmd/root"
	ocflv1 "github.com/srerickson/ocfl-index/gen/ocfl/v1"
)

const newDirMode = 0755

type Cmd struct {
	root     *root.Cmd
	objectID string
	dst      string
	version  string
}

// export export files from an object's version state, copying them to the local
// filesystem. The first argument must be an object id. The second argument is a
// local filesystem path to a directory where the object's files will be copied.
// If the directory exists, it must be empty; if it does not exist, it will be
// created.
func (exp *Cmd) NewCommand(r *root.Cmd) *cobra.Command {
	exp.root = r
	cmd := &cobra.Command{
		Use:   `export [-V|--version=] {object_id} {dst}`,
		Short: "export object's files to the local filesystem",
		Long:  "export object's files to the local filesystem",
	}
	cmd.Flags().StringVarP(&exp.version, "version", "V", "", "use the specified object version (default value refers to HEAD)")
	return cmd
}

func (exp *Cmd) ParseArgs(args []string) error {
	if len(args) != 2 {
		return errors.New("export requires two arguments: object id and destination path")
	}
	exp.objectID = args[0]
	exp.dst = filepath.Clean(args[1])
	return nil
}

type srcFile struct {
	sum  string
	name string
}

func (exp *Cmd) Run(ctx context.Context, args []string) error {
	if exp.dst == "" {
		err := errors.New("no destination path to copy to")
		return exportCanceled(err)
	}
	dstmod, err := statDst(exp.dst)
	if err != nil {
		return exportCanceled(err)
	}
	if dstmod == dstExistFile {
		err := errors.New("destination must be a directory")
		return exportCanceled(err)
	}
	// files to copy, with destination path as key
	copies := map[string]srcFile{}
	state, err := exp.getFullObjectState(ctx, ".")
	if err != nil {
		return exportCanceled(err)
	}
	for _, child := range state.Children {
		dst := filepath.Join(exp.dst, filepath.FromSlash(child.Name))
		copies[dst] = srcFile{
			name: child.Name,
			sum:  child.Digest,
		}
	}

	switch dstmod {
	case dstExistDir:
		// must be empty
		items, err := os.ReadDir(exp.dst)
		if err != nil {
			return exportCanceled(err)
		}
		if len(items) != 0 {
			err := errors.New("destination directory is not empty")
			return exportCanceled(err)
		}
	case dstNotExist:
		// create the directory
		if err := os.Mkdir(exp.dst, newDirMode); err != nil {
			return exportCanceled(err)
		}
	}
	htcl := exp.root.HTTPClient()
	// base http for file transfer
	for dst, src := range copies {
		if err := exp.download(htcl, src, dst); err != nil {
			return fmt.Errorf("during export: %w", err)
		}
	}
	return nil
}

func (exp *Cmd) getFullObjectState(ctx context.Context, src string) (*ocflv1.GetObjectStateResponse, error) {
	client := exp.root.ServiceClient()
	cursor := ""
	var state *ocflv1.GetObjectStateResponse
	for {
		req := connect.NewRequest(&ocflv1.GetObjectStateRequest{
			ObjectId:  exp.objectID,
			Version:   exp.version,
			BasePath:  src,
			Recursive: true,
			PageToken: cursor,
			PageSize:  1000,
		})
		resp, err := client.GetObjectState(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("while getting object state: %w", err)
		}
		if state == nil {
			state = resp.Msg
		} else {
			state.Children = append(state.Children, resp.Msg.Children...)
		}
		if resp.Msg.NextPageToken == "" {
			break
		}
		cursor = resp.Msg.NextPageToken
	}
	return state, nil
}

func exportCanceled(err error) error {
	return fmt.Errorf("export canceled: %w", err)
}

type dstMode int

const (
	dstInvalid dstMode = iota
	dstNotExist
	dstExistDir
	dstExistFile
)

// returns exists, isdir, error
func statDst(dst string) (dstMode, error) {
	info, err := os.Stat(dst)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return dstNotExist, nil
		}
		return dstInvalid, err
	}
	if info.Mode().IsRegular() {
		return dstExistFile, nil
	}
	if info.IsDir() {
		return dstExistDir, nil
	}
	return dstInvalid, errors.New("destination exists but is not a regular file or directory")
}

func (exp *Cmd) download(htcl *http.Client, src srcFile, dst string) error {
	exp.root.Log.Info("copying", "src", src.name, "to", dst)
	if err := os.MkdirAll(filepath.Dir(dst), newDirMode); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	dlurl, err := url.JoinPath(exp.root.RemoteURL, "download", src.sum)
	//dlurl, err := url.JoinPath(exp.root.RemoteURL, "download", src.sum, path.Base(src.name))
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, dlurl, nil)
	if err != nil {
		return err
	}
	resp, err := htcl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		bod, _ := io.ReadAll(resp.Body)
		exp.root.Log.Info("server response", "body", string(bod))
		return fmt.Errorf("server response: %d", resp.StatusCode)
	}
	_, err = io.Copy(f, resp.Body)
	return err
}
