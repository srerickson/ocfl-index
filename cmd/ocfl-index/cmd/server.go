/*
Copyright © 2022
*/
package cmd

import (
	"context"
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/muesli/coral"
	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
)

type page struct {
	// Title   string
	ID     string    // object ID for the page
	VNum   ocfl.VNum // version number (if any)
	Path   string    // logical path (if any)
	Parent string    // parent directory

	Objects  []*index.ObjectMeta
	Versions []*index.VersionMeta
	Content  *index.ContentMeta
}

func (p page) ObjectPath(id string) string {
	return url.QueryEscape(id)
}

func (p page) VersionPath(id string, vnum ocfl.VNum) string {
	v := "HEAD"
	if vnum != ocfl.Head {
		v = vnum.String()
	}
	return path.Join(p.ObjectPath(id), v, ".")
}

func (p page) ContentPath(id string, vnum ocfl.VNum, names ...string) string {
	return path.Join(p.VersionPath(id, vnum), path.Join(names...))
}

type server struct {
	db       *sql.DB
	idx      index.Interface
	tmplRoot *template.Template
	tmplObj  *template.Template
	tmplDir  *template.Template
	mux      *chi.Mux
}

func (serv *server) start(ctx context.Context, dbName string, addr string) error {
	var err error
	serv.tmplRoot = template.Must(template.ParseFS(index.TemplateFS,
		"templates/base.gohtml",
		"templates/root.gohtml"))
	serv.tmplDir = template.Must(template.ParseFS(index.TemplateFS,
		"templates/base.gohtml",
		"templates/dirlist.gohtml"))
	serv.tmplObj = template.Must(template.ParseFS(index.TemplateFS,
		"templates/base.gohtml",
		"templates/object.gohtml"))

	serv.db, err = sql.Open("sqlite", "file:"+dbName)
	if err != nil {
		return err
	}
	defer serv.db.Close()
	serv.idx, err = prepareIndex(ctx, serv.db)
	if err != nil {
		return err
	}
	serv.mux = chi.NewRouter()

	serv.mux.Use(middleware.Logger)
	serv.mux.Get("/", serv.listObjects())
	serv.mux.Route("/{objectID}", func(r chi.Router) {
		r.Get("/", serv.showObject())
		r.Get("/{ver}", serv.showContent())
		r.Get("/{ver}/*", serv.showContent())
	})

	return http.ListenAndServe(addr, serv.mux)
}

func (srv *server) showContent() func(http.ResponseWriter, *http.Request) {
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

func (srv *server) showObject() func(http.ResponseWriter, *http.Request) {
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

func (srv *server) listObjects() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		result, err := srv.idx.AllObjects(r.Context())
		if err != nil {
			panic(err) // FIXME
		}
		p := page{
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

var serveCmd = &coral.Command{
	Use:   "serve",
	Short: "serve content",
	Long:  ``,
	Run: func(cmd *coral.Command, args []string) {
		srv := server{}
		if err := srv.start(cmd.Context(), dbName, "localhost:8800"); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
