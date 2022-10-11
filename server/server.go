package server

import (
	"context"
	"database/sql"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
	"github.com/srerickson/ocfl-index/server/assets"
	"github.com/srerickson/ocfl-index/server/templates"
	"github.com/srerickson/ocfl-index/sqlite"
)

type server struct {
	db       *sql.DB
	idx      index.Interface
	tmplRoot *template.Template
	tmplObj  *template.Template
	tmplDir  *template.Template
	mux      *chi.Mux
}

func Start(ctx context.Context, dbName string, addr string) error {
	var serv server
	var err error
	serv.tmplRoot = template.Must(template.ParseFS(templates.FS,
		"base.gohtml",
		"root.gohtml"))
	serv.tmplDir = template.Must(template.ParseFS(templates.FS,
		"base.gohtml",
		"dirlist.gohtml"))
	serv.tmplObj = template.Must(template.ParseFS(templates.FS,
		"base.gohtml",
		"object.gohtml"))
	serv.db, err = sql.Open("sqlite", "file:"+dbName)
	if err != nil {
		return err
	}
	defer serv.db.Close()
	serv.idx = sqlite.New(serv.db)
	serv.mux = chi.NewRouter()
	serv.mux.Use(middleware.Logger)
	serv.mux.Get("/assets/*", serv.assetsHandler(assets.FS))
	serv.mux.Get("/", serv.listObjectsHandler())
	serv.mux.Route("/{objectID}", func(r chi.Router) {
		r.Get("/", serv.showObjectHandler())
		r.Get("/{ver}", serv.showContentHandler())
		r.Get("/{ver}/*", serv.showContentHandler())
	})
	return http.ListenAndServe(addr, serv.mux)
}

func (srv *server) showContentHandler() func(http.ResponseWriter, *http.Request) {
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
		p := page{
			Title:   "Object Path",
			ID:      result.ID,
			VNum:    result.Version,
			Path:    result.Path,
			Parent:  path.Dir(result.Path),
			Content: result.Content,
		}
		if err := srv.tmplDir.ExecuteTemplate(w, "base", p); err != nil {
			panic(err) // FIXME
		}

		// enc := json.NewEncoder(w)
		// enc.SetIndent("", " ")
		// if err != enc.Encode(result) {
		// 	panic(err) // FIXME
		// }
	}
}

func (srv *server) showObjectHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
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
		p := page{
			Title:    "Object",
			ID:       id,
			Versions: result.Versions,
		}
		if err := srv.tmplObj.ExecuteTemplate(w, "base", p); err != nil {
			panic(err) // FIXME
		}
		// enc := json.NewEncoder(w)
		// enc.SetIndent("", " ")
		// if err != enc.Encode(result) {
		// 	panic(err) // FIXME
		// }
	}
}

func (srv *server) listObjectsHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := srv.idx.AllObjects(r.Context())
		if err != nil {
			panic(err) // FIXME
		}
		p := page{
			Title:   "Storage Root Index",
			Objects: result.Objects,
		}
		if err := srv.tmplRoot.ExecuteTemplate(w, "base", &p); err != nil {
			panic(err) // FIXME
		}
		// enc := json.NewEncoder(w)
		// enc.SetIndent("", " ")
		// if err != enc.Encode(result) {
		// 	panic(err) // FIXEM
		// }
	}
}

func (srv *server) assetsHandler(fsys fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pth := chi.URLParam(r, "*")
		if pth == "" {
			http.NotFound(w, r)
			return
		}
		f, err := fsys.Open(pth)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		if err != nil {
			panic(err) // FIXME
		}
	}
}
