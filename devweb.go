package main

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Development only: with WASAPING_WEB_DIR set to the web/ folder of the
// source, the page is read from disk instead of the copy built into the
// program, and it reloads itself when a file changes (/api/dev).
func devWebDir() string {
	dir := os.Getenv("WASAPING_WEB_DIR")
	if dir == "" {
		return ""
	}
	if st, err := os.Stat(filepath.Join(dir, "index.html")); err != nil || st.IsDir() {
		return ""
	}
	return dir
}

func devFiles(dir string) http.Handler {
	h := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

// handleDev returns the time of the newest file of the page.
func handleDev(w http.ResponseWriter, r *http.Request) {
	var last time.Time
	filepath.WalkDir(devWebDir(), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if i, err := d.Info(); err == nil && i.ModTime().After(last) {
				last = i.ModTime()
			}
		}
		return nil
	})
	writeJSON(w, map[string]any{"changed": last.UnixMilli()})
}
