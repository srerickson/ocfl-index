package server

import (
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/srerickson/ocfl"
	index "github.com/srerickson/ocfl-index"
)

// Page is a top-level webpage model with arbitrary content
type Page struct {
	Title string
	Nav   StatePath
	Body  any
}

// StatePath represents path in the logical state of an object
type StatePath struct {
	ObjectID string
	Version  ocfl.VNum
	Path     string
}

// RootBody is content on the index's root page.
// It includes the description and a list of objects.
type RootBody struct {
	Description string
	Objects     []*index.ObjectMeta
}

// ObjectBody is content on an object page.
// It includes object metadata and list of versions
type ObjectBody struct {
	ID       string
	Head     string
	RootPath string
	Versions []*index.VersionMeta `json:"versions"`
}

// StatePathBody represents represents content from
// the logical state of an object.
type StatePathBody struct {
	Path    StatePath // node path
	Sum     string    // node value
	IsDir   bool      // node value
	DirTree *DirTree  // node children
}

// DirTree is the page model for the dirtree component
type DirTree struct {
	Parent   *StatePath
	Children []index.DirEntry
}

func objectsPath() string {
	return objectsPrefix
}

func objectPath(id string) string {
	return objectsPrefix + "/" + url.QueryEscape(id)
}

func versionPath(id string, vnum ocfl.VNum) string {
	return path.Join(objectPath(id), vnum.String(), ".")
}

// absolute path to for the state. If "child" names are given,
// they are joined as path elements to the st.Path
func statePath(st StatePath, child ...string) string {
	pth := path.Join(st.Path, strings.Join(child, "/"))
	return path.Join(versionPath(st.ObjectID, st.Version), escapeStatePath(pth))
}

// absolute path to the dirtree partial. If "child" names are given,
// they are joined as path elements to the st.Path
func dirtreePath(sum string, st StatePath, child ...string) string {
	pth := path.Join(st.Path, strings.Join(child, "/"))
	return path.Join(
		dirtreePrefix,
		sum,
		url.QueryEscape(st.ObjectID),
		st.Version.String(),
		escapeStatePath(pth))
}

// absolute path to download a file
func downloadPath(sum string, name string) string {
	name = url.PathEscape(path.Base(name))
	return path.Join(downloadPrefix, sum, name)
}

// return the first 12 characters of sum
func short_sum(sum string) string {
	if len(sum) <= 12 {
		return sum
	}
	return sum[:12]
}

// unescapeStatePath does url.PathUnescape for each path element in p, preserving
// any path separators ('/'). An error is returned in url.PathUnescape returns
// an error for any path element.
func unescapeStatePath(p string) (string, error) {
	parts := strings.Split(p, "/")
	newParts := make([]string, len(parts))
	for i, p := range parts {
		np, err := url.PathUnescape(p)
		if err != nil {
			return "", err
		}
		newParts[i] = np
	}
	return strings.Join(newParts, "/"), nil
}

// escapeStatePath does url.PathEscape for each path element in p, preserving
// any path separators ('/').
func escapeStatePath(p string) string {
	parts := strings.Split(p, "/")
	newParts := make([]string, len(parts))
	for i, p := range parts {
		newParts[i] = url.PathEscape(p)
	}
	return strings.Join(newParts, "/")
}

func formatDate(dt time.Time) string {
	return dt.Format("Jan 2, 2006")
}
