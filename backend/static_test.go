package main

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSPAHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "assets"), 0o755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o644)
	js := "console.log('hello')"
	os.WriteFile(filepath.Join(dir, "assets", "app.js"), []byte(js), 0o644)
	f, _ := os.Create(filepath.Join(dir, "assets", "app.js.gz"))
	zw := gzip.NewWriter(f)
	zw.Write([]byte(js))
	zw.Close()
	f.Close()

	r := gin.New()
	r.NoRoute(spaHandler(dir))
	get := func(path, enc string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if enc != "" {
			req.Header.Set("Accept-Encoding", enc)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	w := get("/assets/app.js", "gzip, br")
	if w.Header().Get("Content-Encoding") != "gzip" || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("gzip asset: %v", w.Header())
	}
	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatal(err)
	}
	if body, _ := io.ReadAll(zr); string(body) != js {
		t.Errorf("gzip body: %q", body)
	}
	if w := get("/assets/app.js", ""); w.Header().Get("Content-Encoding") != "" || w.Body.String() != js {
		t.Errorf("plain asset: %v %q", w.Header(), w.Body)
	}
	if w := get("/schedule", ""); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "app") {
		t.Errorf("SPA fallback: %d", w.Code)
	}
	for _, p := range []string{"/.env", "/.git/config", "/wp-login.php"} {
		if w := get(p, ""); w.Code != http.StatusNotFound {
			t.Errorf("%s: %d", p, w.Code)
		}
	}
}
