package main

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// spaHandler serves the built Mini App: existing files as-is (or their pre-compressed .gz
// copy when the client accepts gzip), every other app path falls back to index.html so
// client-side routes survive a reload. Probes for dotfiles and scripts get a plain 404.
func spaHandler(dir string) gin.HandlerFunc {
	root, _ := filepath.Abs(dir)
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if isProbe(p) {
			c.Status(http.StatusNotFound)
			return
		}
		file := filepath.Join(root, filepath.Clean("/"+p))
		if st, err := os.Stat(file); err == nil && !st.IsDir() {
			// Vite fingerprints everything under /assets, so it can be cached forever.
			if strings.HasPrefix(p, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			serveMaybeGzipped(c, file)
			return
		}
		// index.html must never be cached, or clients keep requesting stale asset names.
		c.Header("Cache-Control", "no-cache")
		serveMaybeGzipped(c, filepath.Join(root, "index.html"))
	}
}

// isProbe reports paths no part of the app uses: dotfiles (/.env, /.git/config) and
// server-side scripts (/wp-login.php). Answering them with index.html made scanners
// believe they exist.
func isProbe(p string) bool {
	for _, seg := range strings.Split(p, "/") {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".php" || ext == ".asp" || ext == ".aspx" || ext == ".cgi"
}

func serveMaybeGzipped(c *gin.Context, file string) {
	c.Header("Vary", "Accept-Encoding")
	if strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
		if _, err := os.Stat(file + ".gz"); err == nil {
			// Set the type of the original file; http.ServeFile would otherwise label it gzip.
			if t := mime.TypeByExtension(filepath.Ext(file)); t != "" {
				c.Header("Content-Type", t)
			}
			c.Header("Content-Encoding", "gzip")
			c.File(file + ".gz")
			return
		}
	}
	c.File(file)
}
