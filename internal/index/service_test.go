package index_test

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"testing"

	"github.com/bufbuild/connect-go"
	ocflv0 "github.com/srerickson/ocfl-index/gen/ocfl/v0"
	"github.com/srerickson/ocfl-index/gen/ocfl/v0/ocflv0connect"
	"github.com/srerickson/ocfl-index/internal/index"
)

func TestService(t *testing.T) {
	t.Run("GetContent", func(t *testing.T) {
		runServiceTest(t, testServiceGetContent)
	})
}

// Helpers below

type serviceTestFunc func(ctx context.Context, cli ocflv0connect.IndexServiceClient) error

// return a new httptest.Server and a client for connecting to it, all ready to go.
func newTestService(ctx context.Context, fixture string) (*index.Service, error) {
	idx, err := newTestIndex(ctx, fixture)
	if err != nil {
		return nil, fmt.Errorf("initializing fixture index: %w", err)
	}
	srv := &index.Service{Index: idx}
	log.Println("updating object root directory index ... ")
	if err := srv.SyncObjectRoots(ctx); err != nil {
		return nil, fmt.Errorf("initial object root sync: %w", err)
	}
	log.Println("indexing inventories ... ")
	if err := srv.IndexInventories(ctx); err != nil {
		return nil, fmt.Errorf("initial fixture indexing: %w", err)
	}
	return srv, nil
}

func runServiceTest(t *testing.T, fn serviceTestFunc) {
	ctx := context.Background()
	service, err := newTestService(ctx, "simple-root")
	if err != nil {
		t.Fatal(err)
	}
	httpSrv := httptest.NewTLSServer(service.HTTPHandler())
	cli := ocflv0connect.NewIndexServiceClient(httpSrv.Client(), httpSrv.URL)
	defer httpSrv.Close()
	if err := fn(ctx, cli); err != nil {
		t.Fatal(err)
	}
}

// basic test for GetContent
func testServiceGetContent(ctx context.Context, cli ocflv0connect.IndexServiceClient) error {
	digest := "7dcc352f96c56dc5b094b2492c2866afeb12136a78f0143431ae247d02f02497bbd733e0536d34ec9703eba14c6017ea9f5738322c1d43169f8c77785947ac31"
	req := connect.NewRequest(&ocflv0.GetContentRequest{Digest: digest})
	rsp, err := cli.GetContent(ctx, req)
	if err != nil {
		return err
	}
	recvd := 0
	for rsp.Receive() {
		n, err := io.Discard.Write(rsp.Msg().Data)
		if err != nil {
			return err
		}
		recvd += n
	}
	if err := rsp.Err(); err != nil {
		return err
	}
	if recvd == 0 {
		return fmt.Errorf("GetContent '%s': no data", digest)
	}
	return nil
}
