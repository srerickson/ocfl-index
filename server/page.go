package server

import (
	"net/url"
	"path"

	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
)

type page struct {
	Title  string
	ID     string    // object ID for the page
	VNum   ocfl.VNum // version number (if any)
	Path   string    // logical path (if any)
	Parent string    // parent directory

	Objects  []*index.ObjectMeta
	Versions []*index.VersionMeta
	Content  *index.ContentMeta
}

func (p page) ObjectPath(id string) string {
	return "/" + url.QueryEscape(id)
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
