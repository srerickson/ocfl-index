package server

import (
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/gen/ocfl/v1/ocflv1connect"
	"github.com/srerickson/ocfl-index/internal/service"
)

const (
	jsonResp       = "application/json; charset=UTF-8"
	htmlResp       = "text/html; charset=UTF-8"
	apiPrefix      = "/api"
	objectsPrefix  = "/objects"
	downloadPrefix = "/download"
	assetPrefix    = "/assets"
	dirtreePrefix  = "/component/dirtree"
)

type server struct {
	*index.Index
}

// NewHandler returns new http.Handler for the index service
func NewHandler(idx *index.Index) (http.Handler, error) {
	serv := server{Index: idx}
	mux := chi.NewRouter()
	mux.Use(middleware.Logger)
	mux.Mount(ocflv1connect.NewRootIndexServiceHandler(service.Service{Index: idx}))
	mux.Get(downloadPrefix+"/{sum}/{name}", serv.downloadHandler())
	return mux, nil
}

func (srv *server) downloadHandler() func(http.ResponseWriter, *http.Request) {
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
		p, err := srv.GetContentPath(r.Context(), sum)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := srv.OpenFile(r.Context(), p)
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
