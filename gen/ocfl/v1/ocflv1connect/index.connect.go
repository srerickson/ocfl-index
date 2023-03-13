// Code generated by protoc-gen-connect-go. DO NOT EDIT.
//
// Source: ocfl/v1/index.proto

package ocflv1connect

import (
	context "context"
	errors "errors"
	connect_go "github.com/bufbuild/connect-go"
	v1 "github.com/srerickson/ocfl-index/gen/ocfl/v1"
	http "net/http"
	strings "strings"
)

// This is a compile-time assertion to ensure that this generated file and the connect package are
// compatible. If you get a compiler error that this constant is not defined, this code was
// generated with a version of connect newer than the one compiled into your binary. You can fix the
// problem by either regenerating this code with an older version of connect or updating the connect
// version compiled into your binary.
const _ = connect_go.IsAtLeastVersion0_1_0

const (
	// IndexServiceName is the fully-qualified name of the IndexService service.
	IndexServiceName = "ocfl.v1.IndexService"
)

// IndexServiceClient is a client for the ocfl.v1.IndexService service.
type IndexServiceClient interface {
	// Get index status, counts, and storage root details
	GetStatus(context.Context, *connect_go.Request[v1.GetStatusRequest]) (*connect_go.Response[v1.GetStatusResponse], error)
	// Start an asynchronous indexing process to scan the storage and ingest
	// object inventories. IndexAll returns immediately with a status indicating
	// whether the indexing process was started.
	IndexAll(context.Context, *connect_go.Request[v1.IndexAllRequest]) (*connect_go.Response[v1.IndexAllResponse], error)
	// Index inventories for the specified object ids. Unlike IndexAll, IndexIDs
	// after the object_ids have been indexed.
	IndexIDs(context.Context, *connect_go.Request[v1.IndexIDsRequest]) (*connect_go.Response[v1.IndexIDsResponse], error)
	// OCFL Objects in the index
	ListObjects(context.Context, *connect_go.Request[v1.ListObjectsRequest]) (*connect_go.Response[v1.ListObjectsResponse], error)
	// Details on a specific OCFL object in the index
	GetObject(context.Context, *connect_go.Request[v1.GetObjectRequest]) (*connect_go.Response[v1.GetObjectResponse], error)
	// Query the logical state of an OCFL object version
	GetObjectState(context.Context, *connect_go.Request[v1.GetObjectStateRequest]) (*connect_go.Response[v1.GetObjectStateResponse], error)
	// Stream log messages from indexing tasks
	FollowLogs(context.Context, *connect_go.Request[v1.FollowLogsRequest]) (*connect_go.ServerStreamForClient[v1.FollowLogsResponse], error)
}

// NewIndexServiceClient constructs a client for the ocfl.v1.IndexService service. By default, it
// uses the Connect protocol with the binary Protobuf Codec, asks for gzipped responses, and sends
// uncompressed requests. To use the gRPC or gRPC-Web protocols, supply the connect.WithGRPC() or
// connect.WithGRPCWeb() options.
//
// The URL supplied here should be the base URL for the Connect or gRPC server (for example,
// http://api.acme.com or https://acme.com/grpc).
func NewIndexServiceClient(httpClient connect_go.HTTPClient, baseURL string, opts ...connect_go.ClientOption) IndexServiceClient {
	baseURL = strings.TrimRight(baseURL, "/")
	return &indexServiceClient{
		getStatus: connect_go.NewClient[v1.GetStatusRequest, v1.GetStatusResponse](
			httpClient,
			baseURL+"/ocfl.v1.IndexService/GetStatus",
			opts...,
		),
		indexAll: connect_go.NewClient[v1.IndexAllRequest, v1.IndexAllResponse](
			httpClient,
			baseURL+"/ocfl.v1.IndexService/IndexAll",
			opts...,
		),
		indexIDs: connect_go.NewClient[v1.IndexIDsRequest, v1.IndexIDsResponse](
			httpClient,
			baseURL+"/ocfl.v1.IndexService/IndexIDs",
			opts...,
		),
		listObjects: connect_go.NewClient[v1.ListObjectsRequest, v1.ListObjectsResponse](
			httpClient,
			baseURL+"/ocfl.v1.IndexService/ListObjects",
			opts...,
		),
		getObject: connect_go.NewClient[v1.GetObjectRequest, v1.GetObjectResponse](
			httpClient,
			baseURL+"/ocfl.v1.IndexService/GetObject",
			opts...,
		),
		getObjectState: connect_go.NewClient[v1.GetObjectStateRequest, v1.GetObjectStateResponse](
			httpClient,
			baseURL+"/ocfl.v1.IndexService/GetObjectState",
			opts...,
		),
		followLogs: connect_go.NewClient[v1.FollowLogsRequest, v1.FollowLogsResponse](
			httpClient,
			baseURL+"/ocfl.v1.IndexService/FollowLogs",
			opts...,
		),
	}
}

// indexServiceClient implements IndexServiceClient.
type indexServiceClient struct {
	getStatus      *connect_go.Client[v1.GetStatusRequest, v1.GetStatusResponse]
	indexAll       *connect_go.Client[v1.IndexAllRequest, v1.IndexAllResponse]
	indexIDs       *connect_go.Client[v1.IndexIDsRequest, v1.IndexIDsResponse]
	listObjects    *connect_go.Client[v1.ListObjectsRequest, v1.ListObjectsResponse]
	getObject      *connect_go.Client[v1.GetObjectRequest, v1.GetObjectResponse]
	getObjectState *connect_go.Client[v1.GetObjectStateRequest, v1.GetObjectStateResponse]
	followLogs     *connect_go.Client[v1.FollowLogsRequest, v1.FollowLogsResponse]
}

// GetStatus calls ocfl.v1.IndexService.GetStatus.
func (c *indexServiceClient) GetStatus(ctx context.Context, req *connect_go.Request[v1.GetStatusRequest]) (*connect_go.Response[v1.GetStatusResponse], error) {
	return c.getStatus.CallUnary(ctx, req)
}

// IndexAll calls ocfl.v1.IndexService.IndexAll.
func (c *indexServiceClient) IndexAll(ctx context.Context, req *connect_go.Request[v1.IndexAllRequest]) (*connect_go.Response[v1.IndexAllResponse], error) {
	return c.indexAll.CallUnary(ctx, req)
}

// IndexIDs calls ocfl.v1.IndexService.IndexIDs.
func (c *indexServiceClient) IndexIDs(ctx context.Context, req *connect_go.Request[v1.IndexIDsRequest]) (*connect_go.Response[v1.IndexIDsResponse], error) {
	return c.indexIDs.CallUnary(ctx, req)
}

// ListObjects calls ocfl.v1.IndexService.ListObjects.
func (c *indexServiceClient) ListObjects(ctx context.Context, req *connect_go.Request[v1.ListObjectsRequest]) (*connect_go.Response[v1.ListObjectsResponse], error) {
	return c.listObjects.CallUnary(ctx, req)
}

// GetObject calls ocfl.v1.IndexService.GetObject.
func (c *indexServiceClient) GetObject(ctx context.Context, req *connect_go.Request[v1.GetObjectRequest]) (*connect_go.Response[v1.GetObjectResponse], error) {
	return c.getObject.CallUnary(ctx, req)
}

// GetObjectState calls ocfl.v1.IndexService.GetObjectState.
func (c *indexServiceClient) GetObjectState(ctx context.Context, req *connect_go.Request[v1.GetObjectStateRequest]) (*connect_go.Response[v1.GetObjectStateResponse], error) {
	return c.getObjectState.CallUnary(ctx, req)
}

// FollowLogs calls ocfl.v1.IndexService.FollowLogs.
func (c *indexServiceClient) FollowLogs(ctx context.Context, req *connect_go.Request[v1.FollowLogsRequest]) (*connect_go.ServerStreamForClient[v1.FollowLogsResponse], error) {
	return c.followLogs.CallServerStream(ctx, req)
}

// IndexServiceHandler is an implementation of the ocfl.v1.IndexService service.
type IndexServiceHandler interface {
	// Get index status, counts, and storage root details
	GetStatus(context.Context, *connect_go.Request[v1.GetStatusRequest]) (*connect_go.Response[v1.GetStatusResponse], error)
	// Start an asynchronous indexing process to scan the storage and ingest
	// object inventories. IndexAll returns immediately with a status indicating
	// whether the indexing process was started.
	IndexAll(context.Context, *connect_go.Request[v1.IndexAllRequest]) (*connect_go.Response[v1.IndexAllResponse], error)
	// Index inventories for the specified object ids. Unlike IndexAll, IndexIDs
	// after the object_ids have been indexed.
	IndexIDs(context.Context, *connect_go.Request[v1.IndexIDsRequest]) (*connect_go.Response[v1.IndexIDsResponse], error)
	// OCFL Objects in the index
	ListObjects(context.Context, *connect_go.Request[v1.ListObjectsRequest]) (*connect_go.Response[v1.ListObjectsResponse], error)
	// Details on a specific OCFL object in the index
	GetObject(context.Context, *connect_go.Request[v1.GetObjectRequest]) (*connect_go.Response[v1.GetObjectResponse], error)
	// Query the logical state of an OCFL object version
	GetObjectState(context.Context, *connect_go.Request[v1.GetObjectStateRequest]) (*connect_go.Response[v1.GetObjectStateResponse], error)
	// Stream log messages from indexing tasks
	FollowLogs(context.Context, *connect_go.Request[v1.FollowLogsRequest], *connect_go.ServerStream[v1.FollowLogsResponse]) error
}

// NewIndexServiceHandler builds an HTTP handler from the service implementation. It returns the
// path on which to mount the handler and the handler itself.
//
// By default, handlers support the Connect, gRPC, and gRPC-Web protocols with the binary Protobuf
// and JSON codecs. They also support gzip compression.
func NewIndexServiceHandler(svc IndexServiceHandler, opts ...connect_go.HandlerOption) (string, http.Handler) {
	mux := http.NewServeMux()
	mux.Handle("/ocfl.v1.IndexService/GetStatus", connect_go.NewUnaryHandler(
		"/ocfl.v1.IndexService/GetStatus",
		svc.GetStatus,
		opts...,
	))
	mux.Handle("/ocfl.v1.IndexService/IndexAll", connect_go.NewUnaryHandler(
		"/ocfl.v1.IndexService/IndexAll",
		svc.IndexAll,
		opts...,
	))
	mux.Handle("/ocfl.v1.IndexService/IndexIDs", connect_go.NewUnaryHandler(
		"/ocfl.v1.IndexService/IndexIDs",
		svc.IndexIDs,
		opts...,
	))
	mux.Handle("/ocfl.v1.IndexService/ListObjects", connect_go.NewUnaryHandler(
		"/ocfl.v1.IndexService/ListObjects",
		svc.ListObjects,
		opts...,
	))
	mux.Handle("/ocfl.v1.IndexService/GetObject", connect_go.NewUnaryHandler(
		"/ocfl.v1.IndexService/GetObject",
		svc.GetObject,
		opts...,
	))
	mux.Handle("/ocfl.v1.IndexService/GetObjectState", connect_go.NewUnaryHandler(
		"/ocfl.v1.IndexService/GetObjectState",
		svc.GetObjectState,
		opts...,
	))
	mux.Handle("/ocfl.v1.IndexService/FollowLogs", connect_go.NewServerStreamHandler(
		"/ocfl.v1.IndexService/FollowLogs",
		svc.FollowLogs,
		opts...,
	))
	return "/ocfl.v1.IndexService/", mux
}

// UnimplementedIndexServiceHandler returns CodeUnimplemented from all methods.
type UnimplementedIndexServiceHandler struct{}

func (UnimplementedIndexServiceHandler) GetStatus(context.Context, *connect_go.Request[v1.GetStatusRequest]) (*connect_go.Response[v1.GetStatusResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("ocfl.v1.IndexService.GetStatus is not implemented"))
}

func (UnimplementedIndexServiceHandler) IndexAll(context.Context, *connect_go.Request[v1.IndexAllRequest]) (*connect_go.Response[v1.IndexAllResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("ocfl.v1.IndexService.IndexAll is not implemented"))
}

func (UnimplementedIndexServiceHandler) IndexIDs(context.Context, *connect_go.Request[v1.IndexIDsRequest]) (*connect_go.Response[v1.IndexIDsResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("ocfl.v1.IndexService.IndexIDs is not implemented"))
}

func (UnimplementedIndexServiceHandler) ListObjects(context.Context, *connect_go.Request[v1.ListObjectsRequest]) (*connect_go.Response[v1.ListObjectsResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("ocfl.v1.IndexService.ListObjects is not implemented"))
}

func (UnimplementedIndexServiceHandler) GetObject(context.Context, *connect_go.Request[v1.GetObjectRequest]) (*connect_go.Response[v1.GetObjectResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("ocfl.v1.IndexService.GetObject is not implemented"))
}

func (UnimplementedIndexServiceHandler) GetObjectState(context.Context, *connect_go.Request[v1.GetObjectStateRequest]) (*connect_go.Response[v1.GetObjectStateResponse], error) {
	return nil, connect_go.NewError(connect_go.CodeUnimplemented, errors.New("ocfl.v1.IndexService.GetObjectState is not implemented"))
}

func (UnimplementedIndexServiceHandler) FollowLogs(context.Context, *connect_go.Request[v1.FollowLogsRequest], *connect_go.ServerStream[v1.FollowLogsResponse]) error {
	return connect_go.NewError(connect_go.CodeUnimplemented, errors.New("ocfl.v1.IndexService.FollowLogs is not implemented"))
}