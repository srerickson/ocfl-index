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

func TestServiceGetObejct(t *testing.T) {
	runServiceTest(t, testGetObjectSimpleRequest)
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

// ListObjectsRequest
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

// GetStatusRequest
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

// GetObjectRequest
func testGetObjectSimpleRequest(t *testing.T, ctx context.Context, cli ocflv1connect.IndexServiceClient) {
	req := connect.NewRequest(&api.GetObjectRequest{ObjectId: "ark:/12345/bcd987"})
	rsp, err := cli.GetObject(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	expEq(t, "object_id", rsp.Msg.ObjectId, "ark:/12345/bcd987")
	expEq(t, "algorith", rsp.Msg.DigestAlgorithm, "sha512")
	expEq(t, "object root", rsp.Msg.RootPath, "ark%3A%2F12345%2Fbcd987")
	expEq(t, "spec", rsp.Msg.Spec, "1.0")
	expEq(t, "number version", len(rsp.Msg.Versions), 3)
}

func expEq(t *testing.T, desc string, got, expect any) {
	t.Helper()
	if !reflect.DeepEqual(got, expect) {
		t.Fatalf("%s: got='%v', expected='%v'", desc, got, expect)
	}
}
