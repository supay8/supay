package http

import (
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/brandsrx/supay/internal/storage"
)

func SignedLocalDownload(local *storage.LocalObjectStorage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		key, filename := q.Get("key"), q.Get("filename")
		if err := local.VerifySignedURL(key, filename, q.Get("expires"), q.Get("signature"), time.Now()); err != nil {
			http.NotFound(w, r)
			return
		}
		body, info, err := local.Get(r.Context(), key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", info.ContentType)
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
		_, _ = io.Copy(w, body)
	}
}
