package list_anomalies

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/dto"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/httpx"
)

type HTTP struct{ handler *Handler }

func NewHTTP(handler *Handler) *HTTP { return &HTTP{handler: handler} }

// Handle processes the ListAnomalies HTTP request.
// @Summary List detected anomalies
// @Description List detected anomalies from the Postgres read model, filterable by various dimensions.
// @Tags anomalies
// @Accept json
// @Produce json
// @Param from query int false "From Timestamp"
// @Param to query int false "To Timestamp"
// @Param source query string false "Source System"
// @Param metric query string false "Metric Name"
// @Param env query string false "Environment"
// @Param region query string false "Region"
// @Param instance query string false "Instance ID"
// @Param limit query int false "Limit" default(100)
// @Param offset query int false "Offset" default(0)
// @Success 200 {array} dto.AnomalyResponse
// @Router /v1/anomalies [get]
func (h *HTTP) Handle(c *gin.Context) {
	qp := c.Query

	q := Query{
		FromTS:     ptrInt64(qp("from")),
		ToTS:       ptrInt64(qp("to")),
		Source:     ptrString(qp("source")),
		MetricName: ptrString(qp("metric")),
		Env:        ptrString(qp("env")),
		Region:     ptrString(qp("region")),
		InstanceID: ptrString(qp("instance")),
		Limit:      int32(parseInt(qp("limit"), 100)),
		Offset:     int32(parseInt(qp("offset"), 0)),
	}

	rows, err := h.handler.Handle(c.Request.Context(), q)
	if err != nil {
		httpx.JSONError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, rows)
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrInt64(s string) *int64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
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
