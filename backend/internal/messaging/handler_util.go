package messaging

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/echayko/leadrula/backend/pkg/httpx"
)

const maxAttachmentBytes = 10 << 20 // 10MB per file

var allowedAttachmentTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
	"application/pdf": true, "text/plain": true,
	"application/msword": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
	"application/vnd.ms-excel":                                                true,
}

func isMultipart(r *http.Request) bool {
	return strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data")
}

// parseMessageUploads reads multipart "files" entries with per-file limits.
func parseMessageUploads(r *http.Request) ([]UploadFile, error) {
	if r.MultipartForm == nil {
		if err := r.ParseMultipartForm(maxAttachmentBytes); err != nil {
			return nil, httpx.Validation("invalid upload")
		}
	}
	if r.MultipartForm == nil {
		return nil, nil
	}
	headers := r.MultipartForm.File["files"]
	var out []UploadFile
	for _, fh := range headers {
		if fh.Size > maxAttachmentBytes {
			return nil, httpx.Validation("each attachment must be 10MB or smaller")
		}
		ct := fh.Header.Get("Content-Type")
		if !allowedAttachmentTypes[ct] {
			return nil, httpx.Validation("unsupported attachment type: " + ct)
		}
		f, err := fh.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, maxAttachmentBytes))
		f.Close()
		if err != nil {
			return nil, err
		}
		out = append(out, UploadFile{Filename: fh.Filename, ContentType: ct, Size: fh.Size, Data: data})
	}
	return out, nil
}

func threadTyperName(ctx context.Context, s *Service, userID int64) string {
	var name string
	_ = s.pool.QueryRow(ctx, `SELECT COALESCE(full_name,'') FROM users WHERE id=$1`, userID).Scan(&name)
	return name
}

func typingPayload(threadPublicID string, userID int64, userName string) json.RawMessage {
	b, _ := json.Marshal(map[string]any{"thread_id": threadPublicID, "user_id": userID, "user_name": userName})
	return b
}
