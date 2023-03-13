package index

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/bufbuild/connect-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/srerickson/ocfl"
	api "github.com/srerickson/ocfl-index/gen/ocfl/v1"
	"github.com/srerickson/ocfl-index/gen/ocfl/v1/ocflv1connect"
)

const downloadPrefix = "/download"

// Service implements the gRPC services
type Service struct {
	Log       logr.Logger
	FS        ocfl.FS
	RootPath  string
	Index     *Indexer
	Async     *Async
	ParseConc int
	ScanConc  int
}

// Service implements the service generated with connect-go
var _ (ocflv1connect.IndexServiceHandler) = (*Service)(nil)

func (srv Service) IndexAll(ctx context.Context, rq *connect.Request[api.IndexAllRequest]) (*connect.Response[api.IndexAllResponse], error) {
	opts := &IndexOptions{
		FS:        srv.FS,
		RootPath:  srv.RootPath,
		ParseConc: srv.ParseConc,
		ScanConc:  srv.ScanConc,
	}
	task := func(ctx context.Context, w io.Writer) error {
		opts.Log = stdr.New(log.New(w, "", 0))
		return srv.Index.Index(ctx, opts)
	}
	added, _ := srv.Async.TryNow("indexing", task)
	if !added {
		return nil, errors.New("an indexing task is already running")
	}
	return connect.NewResponse(&api.IndexAllResponse{}), nil
}

func (srv Service) IndexIDs(ctx context.Context, rq *connect.Request[api.IndexIDsRequest]) (*connect.Response[api.IndexIDsResponse], error) {
	// todo check max number of ids
	opts := &IndexOptions{
		FS:        srv.FS,
		RootPath:  srv.RootPath,
		ParseConc: srv.ParseConc,
		ScanConc:  srv.ScanConc,
		ObjectIDs: rq.Msg.ObjectIds,
	}
	task := func(ctx context.Context, w io.Writer) error {
		opts.Log = stdr.New(log.New(w, "", 0))
		return srv.Index.Index(ctx, opts)
	}
	added, taskErr := srv.Async.TryNow("indexing", task)
	if !added {
		return nil, errors.New("an indexing task is already running")
	}
	if err := <-taskErr; err != nil {
		return nil, err
	}
	return connect.NewResponse(&api.IndexIDsResponse{}), nil
	// return srv.Async.MonitorOn(ctx, rq, stream, taskErr)
}

func (srv Service) GetStatus(ctx context.Context, _ *connect.Request[api.GetStatusRequest]) (*connect.Response[api.GetStatusResponse], error) {
	summ, err := srv.Index.GetStoreSummary(ctx)
	if err != nil {
		return nil, err
	}
	return asSummaryResponse(summ), nil
}

func (srv Service) ListObjects(ctx context.Context, rq *connect.Request[api.ListObjectsRequest]) (*connect.Response[api.ListObjectsResponse], error) {
	objects, err := srv.Index.ListObjects(ctx, SortID, int(rq.Msg.PageSize), rq.Msg.PageToken)
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

func (srv Service) FollowLogs(ctx context.Context, rq *connect.Request[api.FollowLogsRequest], stream *connect.ServerStream[api.FollowLogsResponse]) error {
	return srv.Async.MonitorOn(ctx, rq, stream, nil)
}

// HTTPHandler returns new http.Handler for the index service
func (srv Service) HTTPHandler() http.Handler {
	mux := chi.NewRouter()
	mux.Use(RequestLogger(srv.Log))
	mux.Mount(ocflv1connect.NewIndexServiceHandler(srv))
	mux.Get(downloadPrefix+"/{sum}", srv.downloadHandler())
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
		ctx := r.Context()
		sum := chi.URLParam(r, "sum")
		if sum == "" {
			http.NotFound(w, r)
			return
		}
		name := chi.URLParam(r, "name")
		if name == "" {
			name = sum
		}
		p, err := srv.Index.GetContentPath(ctx, sum)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := srv.FS.OpenFile(ctx, path.Join(srv.RootPath, p))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
		defer f.Close()
		if _, err = io.Copy(w, f); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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
		HasSize:       inf.HasSize,
		NextPageToken: inf.NextCursor,
		Children:      make([]*api.GetObjectStateResponse_Item, len(inf.Children)),
	}
	for i, p := range inf.Children {
		msg.Children[i] = &api.GetObjectStateResponse_Item{
			Name:    p.Name,
			Size:    p.Size,
			HasSize: p.HasSize,
			Isdir:   p.IsDir,
			Digest:  p.Sum,
		}
	}
	return connect.NewResponse(msg)

}
func asSummaryResponse(summ StoreSummary) *connect.Response[api.GetStatusResponse] {
	msg := &api.GetStatusResponse{
		StoreDescription: summ.Description,
		StoreSpec:        summ.Spec.String(),
		NumInventories:   int32(summ.NumObjects),
		StoreRootPath:    summ.RootPath,
		// IndexedAt:        timestamppb.New(summ.IndexedAt),
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
