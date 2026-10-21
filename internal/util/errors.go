package util

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ErrorResp struct {
	ErrorCode string                 `json:"error_code"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

func Error(c *gin.Context, status int, code string, msg string, details map[string]interface{}) {
	resp := ErrorResp{
		ErrorCode: code,
		Message:   msg,
	}
	if details != nil {
		resp.Details = details
	}
	c.JSON(status, resp)
}

// ошибка валидации с перечислением полей и нарушений
func ValidationDetails(c *gin.Context, violations map[string]string) {
	details := map[string]interface{}{}
	for f, m := range violations {
		details[f] = m
	}
	Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "ошибка валидации входных данных", details)
}
