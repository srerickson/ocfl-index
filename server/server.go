package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/server/assets"
	"github.com/srerickson/ocfl-index/server/templates"
)

const (
	jsonResp       = "application/json; charset=UTF-8"
	htmlResp       = "text/html; charset=UTF-8"
	apiPrefix      = "/api"
	browsePrefix   = "/objects"
	downloadPrefix = "/download"
	assetPrefix    = "/assets"
)

type server struct {
	fsys     ocfl.FS
	root     string
	idx      index.Interface
	tmplRoot *template.Template
	tmplObj  *template.Template
	tmplPath *template.Template
	// mux      *chi.Mux
}

func New(fsys ocfl.FS, root string, idx index.Interface) (http.Handler, error) {
	serv := server{
		fsys: fsys,
		root: root,
		idx:  idx,
	}
	serv.tmplRoot = template.Must(template.New("").Funcs(pageFuncs).ParseFS(templates.FS,
		"base.gohtml",
		"root.gohtml"))
	serv.tmplPath = template.Must(template.New("").Funcs(pageFuncs).ParseFS(templates.FS,
		"base.gohtml",
		"path.gohtml"))
	serv.tmplObj = template.Must(template.New("").Funcs(pageFuncs).ParseFS(templates.FS,
		"base.gohtml",
		"object.gohtml"))

	// routes
	mux := chi.NewRouter()
	mux.Use(middleware.Logger)

	// HTML
	mux.With(setContentType(htmlResp)).Get("/", serv.rootList())
	mux.Route(browsePrefix+"/{objectID}", func(r chi.Router) {
		r.Use(setContentType(htmlResp))
		r.Get("/", serv.getObjectHandler())
		r.Get("/{ver}", serv.showPathHandler())
		r.Get("/{ver}/*", serv.showPathHandler())
	})

	// API
	mux.With(setContentType(jsonResp)).Get("/api", serv.rootList())
	mux.Route(apiPrefix+"/{objectID}", func(r chi.Router) {
		r.Use(setContentType(jsonResp))
		r.Get("/", serv.getObjectHandler())
		r.Get("/{ver}", serv.showPathHandler())
		r.Get("/{ver}/*", serv.showPathHandler())
	})
	mux.Get(downloadPrefix+"/{sum}/{name}", serv.downloadHandler())
	mux.Get(assetPrefix+"/*", http.StripPrefix("/assets/", http.FileServer(http.FS(assets.FS))).ServeHTTP)
	return mux, nil
}

func setContentType(t string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", t)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(f)
	}
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
		p, err := srv.idx.GetContentPath(r.Context(), sum)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		f, err := srv.fsys.OpenFile(r.Context(), path.Join(srv.root, p))
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

func (srv *server) showPathHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := url.QueryUnescape(chi.URLParam(r, "objectID"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ver := chi.URLParam(r, "ver")
		var vnum ocfl.VNum
		if err := ocfl.ParseVNum(ver, &vnum); err != nil {
			http.NotFound(w, r)
			return
		}
		pth := chi.URLParam(r, "*")
		if pth == "" {
			pth = "."
		}
		result, err := srv.idx.GetContent(r.Context(), id, vnum, pth)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		switch w.Header().Get("Content-Type") {
		case jsonResp:
			enc := json.NewEncoder(w)
			enc.SetIndent("", " ")
			if err != enc.Encode(result) {
				panic(err) // FIXME
			}
		default:
			p := ContentPage{
				Content: result,
			}
			if err := srv.tmplPath.ExecuteTemplate(w, "base", p); err != nil {
				panic(err) // FIXME
			}
		}

	}
}

func (srv *server) getObjectHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("here")
		id, err := url.QueryUnescape(chi.URLParam(r, "objectID"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		result, err := srv.idx.GetObject(r.Context(), id)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if len(result.Versions) == 0 {
			http.NotFound(w, r)
			return
		}
		switch w.Header().Get("Content-Type") {
		case jsonResp:
			enc := json.NewEncoder(w)
			enc.SetIndent("", " ")
			if err != enc.Encode(result) {
				panic(err) // FIXME
			}
		default:
			p := objectPage{
				Page:    Page{Title: result.ID},
				Content: result,
			}
			if err := srv.tmplObj.ExecuteTemplate(w, "base", p); err != nil {
				panic(err) // FIXME
			}
		}

	}
}

func (srv *server) rootList() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := srv.idx.AllObjects(r.Context())
		if err != nil {
			panic(err) // FIXME
		}
		switch w.Header().Get("Content-Type") {
		case jsonResp:
			enc := json.NewEncoder(w)
			enc.SetIndent("", " ")
			if err != enc.Encode(result) {
				panic(err) // FIXEM
			}
		default:
			p := RootPage{
				Page:    Page{Title: "Storage Root Index"},
				Content: result,
			}
			err := srv.tmplRoot.ExecuteTemplate(w, "base", &p)
			if err != nil {
				panic(err) // FIXME
			}
		}
	}
}
