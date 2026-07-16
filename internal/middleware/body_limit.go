package middleware

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kyangconn/music-online-go/internal/handler"
)

// JSONBodyLimitMiddleware bounds JSON request bodies before binding. Multipart
// uploads have a separate aggregate limit based on the configured file sizes.
func JSONBodyLimitMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil || !isJSONMediaType(c.GetHeader("Content-Type")) {
			c.Next()
			return
		}
		if c.Request.ContentLength > maxBytes {
			handler.Error(c, http.StatusRequestEntityTooLarge, "JSON request body is too large")
			c.Abort()
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxBytes+1))
		_ = c.Request.Body.Close()
		if err != nil {
			handler.BadRequest(c, "Invalid JSON request body")
			c.Abort()
			return
		}
		if int64(len(body)) > maxBytes {
			handler.Error(c, http.StatusRequestEntityTooLarge, "JSON request body is too large")
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		c.Request.ContentLength = int64(len(body))
		c.Next()
	}
}

func isJSONMediaType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	return strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json")
}
