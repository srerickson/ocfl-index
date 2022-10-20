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
	objectsPrefix  = "/objects"
	downloadPrefix = "/download"
	assetPrefix    = "/assets"
	dirtreePrefix  = "/component/dirtree"
)

type server struct {
	fsys         ocfl.FS
	root         string
	idx          index.Interface
	tmplRoot     *template.Template
	tmplObj      *template.Template
	tmpStatePath *template.Template
	// mux      *chi.Mux
}

func New(fsys ocfl.FS, root string, idx index.Interface) (http.Handler, error) {
	serv := server{
		fsys: fsys,
		root: root,
		idx:  idx,
	}
	// template functions
	pageFuncs := map[string]any{
		"objects_path":  objectsPath,
		"object_path":   objectPath,   // server route
		"version_path":  versionPath,  // server route
		"state_path":    statePath,    // server route
		"dirtree_path":  dirtreePath,  // server route
		"download_path": downloadPath, // server route
		"short_sum":     short_sum,
		"format_date":   formatDate,
	}

	serv.tmplRoot = template.Must(template.New("").Funcs(pageFuncs).ParseFS(templates.FS,
		"base.gohtml",
		"root.gohtml"))
	serv.tmplObj = template.Must(template.New("").Funcs(pageFuncs).ParseFS(templates.FS,
		"base.gohtml",
		"object.gohtml"))
	serv.tmpStatePath = template.Must(template.New("").Funcs(pageFuncs).ParseFS(templates.FS,
		"base.gohtml",
		"statepath.gohtml"))

	// routes
	mux := chi.NewRouter()
	mux.Use(middleware.Logger)

	// HTML
	mux.Route(objectsPrefix, func(r chi.Router) {
		r.Use(setContentType(htmlResp))
		r.Get("/", serv.rootList())
		r.Route("/{objectID}", func(r chi.Router) {
			r.Use(setCtxObjectID("objectID"))
			r.Get("/", serv.getObjectHandler())
			r.Route("/{ver}", func(r chi.Router) {
				r.Use(setCtxVersion("ver"))
				r.With(setCtxStatePath("*")).
					Get("/*", serv.showContentHandler())
			})
		})
	})

	// API
	mux.Route(apiPrefix, func(r chi.Router) {
		r.Use(setContentType(jsonResp))
		r.Get("/", serv.rootList())
		r.Route("/{objectID}", func(r chi.Router) {
			r.Use(setCtxObjectID("objectID"))
			r.Get("/", serv.getObjectHandler())
			r.Route("/{ver}", func(r chi.Router) {
				r.Use(setCtxVersion("ver"))
				r.With(setCtxStatePath("*")).
					Get("/*", serv.showContentHandler())
			})
		})
	})
	mux.Get(downloadPrefix+"/{sum}/{name}", serv.downloadHandler())
	mux.With(
		setCtxObjectID("objectID"),
		setCtxVersion("ver"),
		setCtxStatePath("*"),
	).Get(dirtreePrefix+"/{sum}/{objectID}/{ver}/*", serv.partialDir())
	mux.Get(assetPrefix+"/*", http.StripPrefix("/assets/", http.FileServer(http.FS(assets.FS))).ServeHTTP)
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

func (srv *server) showContentHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		st, ok := r.Context().Value(statePathKey).(StatePath)
		if !ok {
			panic("state path missing") // FIXME
		}
		result, err := srv.idx.GetContent(r.Context(), st.ObjectID, st.Version, st.Path)
		if err != nil {
			log.Println(err)
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
			p := Page{
				Title: path.Base(st.Path),
				Nav:   st,
			}
			body := StatePathBody{
				Path:  st,
				Sum:   result.Sum,
				IsDir: result.IsDir,
			}
			if result.IsDir {
				body.DirTree = &DirTree{
					Parent:   &body.Path,
					Children: result.Children,
				}
			}
			p.Body = body
			if err := srv.tmpStatePath.ExecuteTemplate(w, "base", p); err != nil {
				panic(err) // FIXME
			}
		}
	}
}

func (srv *server) getObjectHandler() func(http.ResponseWriter, *http.Request) {
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
		switch w.Header().Get("Content-Type") {
		case jsonResp:
			enc := json.NewEncoder(w)
			enc.SetIndent("", " ")
			if err != enc.Encode(result) {
				panic(err) // FIXME
			}
		default:
			p := Page{
				Title: result.ID,
				Nav: StatePath{
					ObjectID: id,
				},
				Body: ObjectBody{
					ID:       id,
					RootPath: result.RootPath,
					Head:     result.Head,
					Versions: result.Versions,
				},
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
			p := Page{
				Title: "Storage Root Index",
				Body:  RootBody(*result),
			}
			err := srv.tmplRoot.ExecuteTemplate(w, "base", &p)
			if err != nil {
				panic(err) // FIXME
			}
		}
	}
}

func (srv *server) partialDir() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var sum string
		if sum = chi.URLParam(r, "sum"); sum == "" {
			http.NotFound(w, r)
			return
		}
		parentPath := r.Context().Value(statePathKey).(StatePath)
		children, err := srv.idx.GetDirChildren(r.Context(), sum)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		d := DirTree{
			Parent:   &parentPath,
			Children: children,
		}
		err = srv.tmpStatePath.ExecuteTemplate(w, "dirtree", &d)
		if err != nil {
			panic(err) // FIXME
		}
	}
}
