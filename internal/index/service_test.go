package index_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/bufbuild/connect-go"
	api "github.com/srerickson/ocfl-index/gen/ocfl/v1"
	"github.com/srerickson/ocfl-index/gen/ocfl/v1/ocflv1connect"
)

func TestService(t *testing.T) {
	t.Run("ListObjects", func(t *testing.T) {
		runServiceTest(t, testListObjectsRequest)
	})
}

// Helpers below

type serviceTestFunc func(t *testing.T, ctx context.Context, cli ocflv1connect.IndexServiceClient)

func runServiceTest(t *testing.T, fn serviceTestFunc) {
	ctx := context.Background()
	service, err := newTestService(ctx, "simple-root")
	if err != nil {
		t.Fatal(err)
	}
	httpSrv := httptest.NewTLSServer(service.HTTPHandler())
	cli := ocflv1connect.NewIndexServiceClient(httpSrv.Client(), httpSrv.URL)
	defer httpSrv.Close()
	fn(t, ctx, cli)
}

// testListObjectsRequest
func testListObjectsRequest(t *testing.T, ctx context.Context, cli ocflv1connect.IndexServiceClient) {
	req := connect.NewRequest(&api.ListObjectsRequest{})
	rsp, err := cli.ListObjects(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(rsp.Msg.Objects) == 0 {
		t.Fatal(errors.New("expected some objects"))
	}
}
