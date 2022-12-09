package service

import (
	"context"

	"github.com/bufbuild/connect-go"

	index "github.com/srerickson/ocfl-index"
	api "github.com/srerickson/ocfl-index/gen/ocfl/v1"
	"github.com/srerickson/ocfl-index/gen/ocfl/v1/ocflv1connect"
)

type Service struct {
	index.Index
}

// Service implements the service generated with connect-go
var _ (ocflv1connect.RootIndexServiceHandler) = (*Service)(nil)

func (srv Service) ListObjects(ctx context.Context, rq *connect.Request[api.ListObjectsRequest]) (*connect.Response[api.ListObjectsResponse], error) {
	objects, err := srv.Index.AllObjects(ctx)
	if err != nil {
		return nil, err
	}
	msg := &api.ListObjectsResponse{
		Objects: make([]*api.ListObjectsResponse_Object, len(objects.Objects)),
	}
	for i, meta := range objects.Objects {
		msg.Objects[i] = &api.ListObjectsResponse_Object{
			ObjectId: meta.ID,
			Head:     meta.Head.String()}
	}
	return connect.NewResponse(msg), nil
}
func (srv Service) GetObject(ctx context.Context, rq *connect.Request[api.GetObjectRequest]) (*connect.Response[api.GetObjectResponse], error) {
	obj, err := srv.Index.GetObject(ctx, rq.Msg.ObjectId)
	if err != nil {
		return nil, err
	}
	// FIXME handle versions
	msg := &api.GetObjectResponse{
		ObjectId: obj.ID,
		Head:     obj.Head,
		RootPath: obj.RootPath,
	}
	return connect.NewResponse(msg), nil
}
