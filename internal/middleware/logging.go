package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		reqID := uuid.New().String()
		c.Set("request_id", reqID)
		c.Header("X-Request-Id", reqID)

		var bodyLog string
		if c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "DELETE" {
			if c.Request.Body != nil {
				b, _ := io.ReadAll(c.Request.Body)
				c.Request.Body = io.NopCloser(bytes.NewBuffer(b))
				// логируем как есть
				bodyLog = string(b)
			}
		}

		c.Next()

		end := time.Now()
		dur := end.Sub(start).Milliseconds()

		uid := ""
		if v, ok := c.Get("user_id"); ok {
			if s, ok := v.(string); ok && s != "" {
				uid = s
			}
		}

		log := map[string]interface{}{
			"request_id":  reqID,
			"method":      c.Request.Method,
			"endpoint":    c.FullPath(),
			"status_code": c.Writer.Status(),
			"duration_ms": dur,
			"user_id":     nil,
			"timestamp":   start.UTC().Format(time.RFC3339),
		}
		if uid != "" {
			log["user_id"] = uid
		}
		if bodyLog != "" {
			log["request_body"] = bodyLog
		}

		out, _ := json.Marshal(log)
		println(string(out))
	}
}
