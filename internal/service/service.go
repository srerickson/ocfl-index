package service

import (
	"context"

	"github.com/bufbuild/connect-go"
	"google.golang.org/genproto/googleapis/type/datetime"

	index "github.com/srerickson/ocfl-index"
	api "github.com/srerickson/ocfl-index/gen/ocfl/v1"
	"github.com/srerickson/ocfl-index/gen/ocfl/v1/ocflv1connect"
)

type Service struct {
	*index.Index
}

// Service implements the service generated with connect-go
var _ (ocflv1connect.RootIndexServiceHandler) = (*Service)(nil)

func (srv Service) ListObjects(ctx context.Context, rq *connect.Request[api.ListObjectsRequest]) (*connect.Response[api.ListObjectsResponse], error) {
	objects, err := srv.Index.ListObjects(ctx)
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
	msg := &api.GetObjectResponse{
		ObjectId: obj.ID,
		RootPath: obj.RootPath,
	}
	msg.Versions = make([]*api.GetObjectResponse_Version, len(obj.Versions))
	for i, v := range obj.Versions {
		v.Created.Year()
		msg.Versions[i] = &api.GetObjectResponse_Version{
			Num:     v.Version.String(),
			Message: v.Message,
			Created: &datetime.DateTime{
				Year:    int32(v.Created.Year()),
				Month:   int32(v.Created.Month()),
				Day:     int32(v.Created.Day()),
				Hours:   int32(v.Created.Hour()),
				Minutes: int32(v.Created.Minute()),
				Seconds: int32(v.Created.Second()),
			},
		}
		if v.User != nil {
			msg.Versions[i].User = &api.GetObjectResponse_Version_User{
				Address: v.User.Address,
				Name:    v.User.Name,
			}
		}
	}
	return connect.NewResponse(msg), nil
}
