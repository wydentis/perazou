package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// spaHandler serves the built Mini App: existing files as-is, every other non-API path
// falls back to index.html so client-side routes survive a reload.
func spaHandler(dir string) gin.HandlerFunc {
	root, _ := filepath.Abs(dir)
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api/") || (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		file := filepath.Join(root, filepath.Clean("/"+p))
		if st, err := os.Stat(file); err == nil && !st.IsDir() {
			// Vite fingerprints everything under /assets, so it can be cached forever.
			if strings.HasPrefix(p, "/assets/") {
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			}
			c.File(file)
			return
		}
		// index.html must never be cached, or clients keep requesting stale asset names.
		c.Header("Cache-Control", "no-cache")
		c.File(filepath.Join(root, "index.html"))
	}
}
