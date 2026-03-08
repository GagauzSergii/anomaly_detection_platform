package httpx

import "github.com/gin-gonic/gin"

// JSONError returns a consistent error format.
func JSONError(ctx *gin.Context, statusCode int, message string) {
	ctx.AbortWithStatusJSON(statusCode, gin.H{"message": message})
}
