package service

import (
	"context"
	"fmt"
	"time"

	"github.com/bufbuild/connect-go"
	"google.golang.org/protobuf/types/known/timestamppb"

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
	newRQ, err := asObjectListQuery(rq)
	if err != nil {
		return nil, err
	}
	objects, err := srv.Index.ListObjects(ctx, newRQ)
	if err != nil {
		return nil, err
	}
	return asObjectListResponse(objects), nil
}

func (srv Service) GetObject(ctx context.Context, rq *connect.Request[api.GetObjectRequest]) (*connect.Response[api.GetObjectResponse], error) {
	obj, err := srv.Index.GetObject(ctx, rq.Msg.ObjectId)
	if err != nil {
		return nil, err
	}
	return asGetObjectResponse(obj), nil
}

func asObjectListQuery(rq *connect.Request[api.ListObjectsRequest]) (index.ObjectListQuery, error) {
	var newRQ index.ObjectListQuery
	newRQ.Limit = int(rq.Msg.Limit)
	newRQ.Order = index.ObjectListOrder(rq.Msg.Order)
	switch rq.Msg.Order {
	case api.ListObjectsRequest_ORDER_ASC_HEAD_CREATED:
		fallthrough
	case api.ListObjectsRequest_ORDER_DESC_HEAD_CREATED:
		fallthrough
	case api.ListObjectsRequest_ORDER_ASC_V1_CREATED:
		fallthrough
	case api.ListObjectsRequest_ORDER_DESC_V1_CREATED:
		t, err := time.Parse(time.RFC3339, rq.Msg.Cursor)
		if err != nil {
			return newRQ, fmt.Errorf("invalid cursor: %w", err)
		}
		newRQ.Cursor = t
	case api.ListObjectsRequest_ORDER_ASC_ID:
		fallthrough
	case api.ListObjectsRequest_ORDER_DESC_ID:
		newRQ.Cursor = rq.Msg.Cursor
	}
	return newRQ, nil
}

// func toObjectListRequest(rq index.ObjectListRequest) *connect.Request[api.ListObjectsRequest] {
// 	newRQ := &api.ListObjectsRequest{
// 		Limit: int32(rq.Limit),
// 		Order: api.ListObjectsRequest_Order(rq.Limit),
// 	}
// 	switch c := rq.Cursor.(type) {
// 	case time.Time:
// 		newRQ.Cursor = c.Format(time.RFC3339)
// 	case string:
// 		newRQ.Cursor = c
// 	}
// 	return connect.NewRequest(newRQ)
// }

func asObjectListResponse(objects index.ObjectList) *connect.Response[api.ListObjectsResponse] {
	msg := &api.ListObjectsResponse{
		Objects: make([]*api.ListObjectsResponse_Object, len(objects)),
	}
	for i, meta := range objects {
		msg.Objects[i] = &api.ListObjectsResponse_Object{
			ObjectId: meta.ID,
			Head:     meta.Head.String()}
	}
	return connect.NewResponse(msg)
}

func asGetObjectResponse(obj *index.ObjectDetails) *connect.Response[api.GetObjectResponse] {
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
			Created: timestamppb.New(v.Created),
		}
		if v.User != nil {
			msg.Versions[i].User = &api.GetObjectResponse_Version_User{
				Address: v.User.Address,
				Name:    v.User.Name,
			}
		}
	}
	return connect.NewResponse(msg)
}
