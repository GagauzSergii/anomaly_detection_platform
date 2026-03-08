package list_series

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/httpx"
)

type HTTP struct {
	handler *Handler
}

// HTTP is Gin adapter for ListSeries feature.
func NewHTTP(handler *Handler) *HTTP {
	return &HTTP{handler: handler}
}

func (handler *HTTP) Handle(ctx *gin.Context) {
	query := Query{
		Source:     ptrQuery(ctx, "source"),
		MetricName: ptrQuery(ctx, "metric"),
		Env:        ptrQuery(ctx, "env"),
		Region:     ptrQuery(ctx, "region"),
		Limit:      int32(parseInt(ctx.Query("limit"), 200)),
	}

	rows, err := handler.handler.Handle(ctx.Request.Context(), query)
	if err != nil {
		httpx.JSONError(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, rows)
}

func ptrQuery(c *gin.Context, key string) *string {
	v := c.Query(key)
	if v == "" {
		return nil
	}
	return &v
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	i, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return i
}
