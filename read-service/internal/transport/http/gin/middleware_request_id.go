package gin

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestID middleware ensures each request has a request_id.
// In a real setup you'd also propagate trace headers.
// TODO check setup trace
func RequestID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		id := ctx.GetHeader("X-Request-Id")
		if id == "" {
			id = uuid.NewString()
		}
		ctx.Writer.Header().Set("X-Request-Id", id)
		ctx.Set("request_id", id)
		ctx.Next()
	}
}
