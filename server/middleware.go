package server

import (
	"context"
	"net/http"
	"net/url"

	"github.com/go-chi/chi"
	"github.com/srerickson/ocfl"
)

type ctxKey int

const (
	objectIDKey ctxKey = iota
	versionNumKey
	statePathKey
)

func setContentType(t string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", t)
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(f)
	}
}

func setCtxObjectID(chiParam string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			param := chi.URLParam(r, chiParam)
			if param == "" {
				http.Error(w, "missing object id", http.StatusBadRequest)
				return
			}
			objID, err := url.PathUnescape(param)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			newCtx := context.WithValue(r.Context(), objectIDKey, objID)
			next.ServeHTTP(w, r.WithContext(newCtx))
		}
		return http.HandlerFunc(f)
	}
}

func setCtxVersion(chiParam string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			param := chi.URLParam(r, chiParam)
			if param == "" {
				http.Error(w, "missing version number", http.StatusBadRequest)
				return
			}
			var vnum ocfl.VNum
			if err := ocfl.ParseVNum(param, &vnum); err != nil {
				// TODO log error
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			newCtx := context.WithValue(r.Context(), versionNumKey, vnum)
			next.ServeHTTP(w, r.WithContext(newCtx))
		}
		return http.HandlerFunc(f)
	}
}

func setCtxStatePath(chiParam string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		f := func(w http.ResponseWriter, r *http.Request) {
			objID, ok := r.Context().Value(objectIDKey).(string)
			if !ok {
				http.Error(w, "missing object id", http.StatusBadRequest)
				return
			}
			ver, ok := r.Context().Value(versionNumKey).(ocfl.VNum)
			if !ok {
				http.Error(w, "missing version number", http.StatusBadRequest)
				return
			}
			param := chi.URLParam(r, chiParam)
			if param == "" {
				param = "."
			}
			pth, err := unescapeStatePath(param)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			statePath := StatePath{Path: pth, ObjectID: objID, Version: ver}
			newCtx := context.WithValue(r.Context(), statePathKey, statePath)
			next.ServeHTTP(w, r.WithContext(newCtx))
		}
		return http.HandlerFunc(f)
	}
}
