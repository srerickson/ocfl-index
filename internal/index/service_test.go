package index_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/bufbuild/connect-go"
	api "github.com/srerickson/ocfl-index/gen/ocfl/v1"
	"github.com/srerickson/ocfl-index/gen/ocfl/v1/ocflv1connect"
)

func TestServiceGetStatus(t *testing.T) {
	runServiceTest(t, testGetStatusSimpleRequest)
}

func TestServiceListObject(t *testing.T) {
	runServiceTest(t, testListObjectsRequest)
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

func testGetStatusSimpleRequest(t *testing.T, ctx context.Context, cli ocflv1connect.IndexServiceClient) {
	req := connect.NewRequest(&api.GetStatusRequest{})
	rsp, err := cli.GetStatus(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	exp := api.GetStatusResponse{
		Status:           "ready",
		StoreSpec:        "1.0",
		StoreRootPath:    "simple-root",
		StoreDescription: "",
		NumObjectPaths:   3,
		NumInventories:   3,
	}
	expEq(t, "status", rsp.Msg.Status, exp.Status)
	expEq(t, "store spec", rsp.Msg.StoreSpec, exp.StoreSpec)
	expEq(t, "store root", rsp.Msg.StoreRootPath, exp.StoreRootPath)
	expEq(t, "store description", rsp.Msg.StoreDescription, exp.StoreDescription)
	expEq(t, "number inventories", rsp.Msg.NumInventories, exp.NumInventories)
	expEq(t, "number objects", rsp.Msg.NumObjectPaths, exp.NumObjectPaths)
}
func expEq(t *testing.T, desc string, got, expect any) {
	t.Helper()
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("%s: got='%v', expected='%v'", desc, got, expect)
	}
}
