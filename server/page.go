package server

import (
	"net/url"
	"path"
	"text/template"

	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
)

type Page struct {
	Title string
}

type RootPage struct {
	Page
	Content *index.ListObjectsResult
}

type objectPage struct {
	Page
	Content *index.ObjectResult
}

type ContentPage struct {
	Page
	Content *index.PathResult
}

var pageFuncs template.FuncMap = map[string]any{
	"object_path":  objectPath,
	"version_path": versionPath,
	"content_path": contentPath,
}

func objectPath(id string) string {
	return browsePrefix + "/" + url.QueryEscape(id)
}

func versionPath(id string, vnum ocfl.VNum) string {
	return path.Join(objectPath(id), vnum.String(), ".")
}

func contentPath(id string, vnum ocfl.VNum, names ...string) string {
	return path.Join(versionPath(id, vnum), path.Join(names...))
}
