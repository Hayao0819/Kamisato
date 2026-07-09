package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/domain"
	"github.com/Hayao0819/Kamisato/ayato/stream"
)

// nopSeekCloser adds a no-op Close to a bytes.Reader so it satisfies
// io.ReadSeekCloser for stream.NewFileStream.
type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

func bufferToReadSeekCloser(buf *bytes.Buffer) nopSeekCloser {
	return nopSeekCloser{bytes.NewReader(buf.Bytes())}
}

// The file server must mark content-immutable package archives cacheable forever
// while keeping the mutable repo DB revalidating.
func TestRepoFileCacheControl(t *testing.T) {
	cases := []struct {
		file      string
		wantCache string
	}{
		{"foo-1.0-1-x86_64.pkg.tar.zst", "public, max-age=31536000, immutable"},
		{"foo-1.0-1-x86_64.pkg.tar.zst.sig", "public, max-age=31536000, immutable"},
		{"core.db", "no-cache"},
		{"core.db.tar.gz", "no-cache"},
	}

	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			ctrl, mockSvc, h := setup(t)
			defer ctrl.Finish()

			// h.cfg is nil, so the handler tries SignedURL first; return "" to fall
			// through to the streaming path where the cache headers are set.
			mockSvc.EXPECT().SignedURL("core", "x86_64", tc.file).Return("", nil)
			fs := stream.NewFileStream(tc.file, "application/octet-stream", nopSeekCloser{bytes.NewReader([]byte("data"))})
			mockSvc.EXPECT().GetFileWithMeta("core", "x86_64", tc.file).Return(fs, domain.FileMeta{ETag: `"v1"`}, nil)

			r := gin.New()
			r.GET("/repo/:repo/:arch/:file", h.RepoFileHandler)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/repo/core/x86_64/"+tc.file, nil))

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
			}
			if got := w.Header().Get("Cache-Control"); got != tc.wantCache {
				t.Fatalf("Cache-Control = %q, want %q", got, tc.wantCache)
			}
		})
	}
}
