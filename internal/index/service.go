package index

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-logr/logr"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/srerickson/ocfl"
	api "github.com/srerickson/ocfl-index/gen/ocfl/v0"
	"github.com/srerickson/ocfl-index/gen/ocfl/v0/ocflv0connect"
)

const downloadPrefix = "/download"

// Service implements the gRPC services
type Service struct {
	*Index
}

// Service implements the service generated with connect-go
var _ (ocflv0connect.IndexServiceHandler) = (*Service)(nil)

func (srv Service) GetContent(ctx context.Context, rq *connect.Request[api.GetContentRequest], stream *connect.ServerStream[api.GetContentResponse]) error {
	name, err := srv.Index.GetContentPath(ctx, rq.Msg.Digest)
	if err != nil {
		return err
	}
	f, err := srv.OpenFile(ctx, name)
	if err != nil {
		return err
	}
	defer f.Close()
	buff := make([]byte, 1024*64)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := f.Read(buff)
		if n > 0 {
			msg := &api.GetContentResponse{Data: buff[:n]}
			if stream.Send(msg); err != nil {
				return err
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (srv Service) GetSummary(ctx context.Context, _ *connect.Request[api.GetSummaryRequest]) (*connect.Response[api.GetSummaryResponse], error) {
	summ, err := srv.Index.GetStoreSummary(ctx)
	if err != nil {
		return nil, err
	}
	return asSummaryResponse(summ), nil
}

func (srv Service) ListObjects(ctx context.Context, rq *connect.Request[api.ListObjectsRequest]) (*connect.Response[api.ListObjectsResponse], error) {
	var sort ObjectSort
	if rq.Msg.OrderBy != nil {
		switch rq.Msg.OrderBy.Field {
		case api.ListObjectsRequest_Sort_FIELD_UNSPECIFIED:
			fallthrough
		case api.ListObjectsRequest_Sort_FIELD_ID:
			sort = SortID
		case api.ListObjectsRequest_Sort_FIELD_V1_CREATED:
			sort = SortV1Created
		case api.ListObjectsRequest_Sort_FIELD_HEAD_CREATED:
			sort = SortHeadCreated
		}
		switch rq.Msg.OrderBy.Order {
		case api.ListObjectsRequest_Sort_ORDER_DESC:
			sort = sort | DESC
		}
	}
	objects, err := srv.Index.ListObjects(ctx, sort, int(rq.Msg.PageSize), rq.Msg.PageToken)
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

// HTTPHandler returns new http.Handler for the index service
func (srv Service) HTTPHandler() http.Handler {
	mux := chi.NewRouter()
	mux.Use(RequestLogger(srv.log))
	mux.Mount(ocflv0connect.NewIndexServiceHandler(srv))
	mux.Get(downloadPrefix+"/{sum}/{name}", srv.downloadHandler())
	return mux
}

func RequestLogger(logger logr.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			t1 := time.Now()
			defer func() {
				req := r.Method + " " + r.RequestURI
				logger.Info(req,
					"status", ww.Status(),
					"bytesWritten", ww.BytesWritten(),
					"time", time.Since(t1))
			}()
			next.ServeHTTP(ww, r)
		}
		return http.HandlerFunc(fn)
	}
}

func (srv Service) downloadHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sum := chi.URLParam(r, "sum")
		if sum == "" {
			http.NotFound(w, r)
			return
		}
		name := chi.URLParam(r, "name")
		if name == "" {
			http.NotFound(w, r)
			return
		}
		p, err := srv.Index.GetContentPath(r.Context(), sum)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := srv.Index.OpenFile(r.Context(), p)
		if err != nil {
			panic(err)
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		defer f.Close()
		if _, err = io.Copy(w, f); err != nil {
			panic(err) // FIXME
		}

	}
}

func (srv Service) GetObjectState(ctx context.Context, rq *connect.Request[api.GetObjectStateRequest]) (*connect.Response[api.GetObjectStateResponse], error) {
	var vnum ocfl.VNum
	if v := rq.Msg.Version; v != "" {
		if err := ocfl.ParseVNum(rq.Msg.Version, &vnum); err != nil {
			return nil, err
		}
	}
	list, err := srv.Index.GetObjectState(ctx, rq.Msg.ObjectId, vnum, rq.Msg.BasePath, rq.Msg.Recursive, int(rq.Msg.PageSize), rq.Msg.PageToken)
	if err != nil {
		return nil, err
	}
	return asGetObjectStateResponse(list), nil

}

func asGetObjectStateResponse(inf *PathInfo) *connect.Response[api.GetObjectStateResponse] {
	msg := &api.GetObjectStateResponse{
		Digest:        inf.Sum,
		Isdir:         inf.IsDir,
		Size:          inf.Size,
		NextPageToken: inf.NextCursor,
		Children:      make([]*api.GetObjectStateResponse_Item, len(inf.Children)),
	}
	for i, p := range inf.Children {
		msg.Children[i] = &api.GetObjectStateResponse_Item{
			Name:   p.Name,
			Size:   p.Size,
			Isdir:  p.IsDir,
			Digest: p.Sum,
		}
	}
	return connect.NewResponse(msg)

}
func asSummaryResponse(summ StoreSummary) *connect.Response[api.GetSummaryResponse] {
	msg := &api.GetSummaryResponse{
		Description: summ.Description,
		Spec:        summ.Spec.String(),
		NumObjects:  int32(summ.NumObjects),
		RootPath:    summ.RootPath,
		IndexedAt:   timestamppb.New(summ.IndexedAt),
	}
	return connect.NewResponse(msg)
}

// func toObjectListRequest(rq ObjectListRequest) *connect.Request[api.ListObjectsRequest] {
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

func asObjectListResponse(objects *ObjectList) *connect.Response[api.ListObjectsResponse] {
	msg := &api.ListObjectsResponse{
		Objects:       make([]*api.ListObjectsResponse_Object, len(objects.Objects)),
		NextPageToken: objects.NextCursor,
	}
	for i, obj := range objects.Objects {
		msg.Objects[i] = &api.ListObjectsResponse_Object{
			ObjectId:    obj.ID,
			Head:        obj.Head.String(),
			V1Created:   timestamppb.New(obj.V1Created),
			HeadCreated: timestamppb.New(obj.HeadCreated),
		}
	}
	return connect.NewResponse(msg)
}

func asGetObjectResponse(obj *Object) *connect.Response[api.GetObjectResponse] {
	msg := &api.GetObjectResponse{
		ObjectId: obj.ID,
		RootPath: obj.RootPath,
	}
	msg.Versions = make([]*api.GetObjectResponse_Version, len(obj.Versions))
	for i, v := range obj.Versions {
		v.Created.Year()
		msg.Versions[i] = &api.GetObjectResponse_Version{
			Num:     v.Num.String(),
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
