/*
Copyright (c) 2022 The Regents of the University of California.
*/
package root

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/bufbuild/connect-go"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl"
	ocflv1 "github.com/srerickson/ocfl-index/gen/ocfl/v1"
	"github.com/srerickson/ocfl-index/gen/ocfl/v1/ocflv1connect"
	"github.com/srerickson/ocfl-index/internal/index"
)

const (
	envRemote     = "OCFL_INDEX"
	defaultRemote = "http://localhost:8080"
)

// RootCmd is the type of the root command
type Cmd struct {
	cobra.Command
	Log        logr.Logger
	RemoteURL  string
	httpClient *http.Client
	rpcClient  ocflv1connect.IndexServiceClient
}

type OxCmd interface {
	NewCommand(*Cmd) *cobra.Command
	ParseArgs([]string) error
	Run(context.Context, []string) error
}

func (ox *Cmd) Init() {
	ox.RemoteURL = getenvDefault(envRemote, defaultRemote)
}

func (ox *Cmd) AddSub(subs ...OxCmd) {
	for _, sub := range subs {
		sub := sub
		cmd := sub.NewCommand(ox)
		if cmd.RunE == nil {
			cmd.RunE = func(c *cobra.Command, args []string) error {
				if err := sub.ParseArgs(args); err != nil {
					return err
				}
				return sub.Run(c.Context(), args)
			}
		}
		ox.AddCommand(cmd)
	}
}

func (ox *Cmd) HTTPClient() *http.Client {
	if ox.httpClient == nil {
		ox.httpClient = &http.Client{
			// Timeout: 20 * time.Second,
		}
	}
	return ox.httpClient
}
func (ox *Cmd) ServiceClient() ocflv1connect.IndexServiceClient {
	if ox.rpcClient == nil {
		ox.rpcClient = ocflv1connect.NewIndexServiceClient(ox.HTTPClient(), ox.RemoteURL)
	}
	return ox.rpcClient
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func (ox Cmd) FollowLogs(ctx context.Context) error {
	cli := ox.ServiceClient()
	rq := ocflv1.FollowLogsRequest{}
	stream, err := cli.FollowLogs(ctx, connect.NewRequest(&rq))
	if err != nil {
		return err
	}
	for stream.Receive() {
		msg := stream.Msg().Message
		ox.Log.Info(msg)
	}
	if err := stream.Err(); err != nil {
		return err
	}
	return nil
}

func (ox Cmd) ListObjects(prefix string, pageSize int) *ListObjectsIterator {
	return &ListObjectsIterator{
		client: ox.ServiceClient(),
		limit:  pageSize,
		prefix: prefix,
	}
}

type ListObjectsIterator struct {
	client   ocflv1connect.IndexServiceClient
	prefix   string
	limit    int
	nextPage string
	results  *ocflv1.ListObjectsResponse
	i        int // index of the object returned by call to Next()
}

func (pager *ListObjectsIterator) Next(ctx context.Context) (*index.ObjectListItem, error) {
	if pager.needNextPage() {
		pager.i = 0 // resets
		req := ocflv1.ListObjectsRequest{
			PageToken: pager.nextPage,
			PageSize:  int32(pager.limit),
			IdPrefix:  pager.prefix,
		}
		resp, err := pager.client.ListObjects(ctx, connect.NewRequest(&req))
		if err != nil {
			return nil, err
		}
		pager.results = resp.Msg
		pager.nextPage = pager.results.NextPageToken
	}
	if pager.i >= len(pager.results.Objects) {
		return nil, io.EOF
	}
	item := pager.results.Objects[pager.i]
	pager.i += 1
	obj := &index.ObjectListItem{
		//RootPath: item.,
		ID:          item.ObjectId,
		V1Created:   item.V1Created.AsTime(),
		HeadCreated: item.V1Created.AsTime(),
	}
	if err := ocfl.ParseVNum(item.Head, &obj.Head); err != nil {
		return nil, fmt.Errorf("received object has invalid head: %w", err)
	}
	return obj, nil
}

func (pager *ListObjectsIterator) needNextPage() bool {
	if pager.results == nil {
		return true
	}
	if pager.nextPage != "" && pager.i == len(pager.results.Objects) && pager.i > 0 {
		return true
	}
	return false
}
