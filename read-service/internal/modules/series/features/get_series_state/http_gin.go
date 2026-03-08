package get_series_state

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	_ "github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/dto"
	"github.com/sergii-gagauz/anomaly_detection_platform/read-service/internal/shared/httpx"
)

type HTTP struct {
	handler *Handler
}

// ATG Update: New constructor for the HTTP adapter.
func NewHTTP(handler *Handler) *HTTP {
	return &HTTP{handler: handler}
}

// Handle processes the GetSeriesState HTTP request.
// @Summary Retrieve telemetry state
// @Description Retrieve detailed telemetry state (parameters and latest values) for a specific metric series by its key.
// @Tags metrics
// @Accept json
// @Produce json
// @Param key path string true "Composite Key: source|metric|env|region"
// @Success 200 {object} dto.SeriesStateResponse
// @Router /v1/series/state/{key} [get]
func (h *HTTP) Handle(ctx *gin.Context) {
	keyStr := ctx.Param("key")
	// Expected format: source|metric|env|region
	parts := strings.Split(keyStr, "|")
	if len(parts) != 4 {
		httpx.JSONError(ctx, http.StatusBadRequest, "invalid key format, expected source|metric|env|region")
		return
	}

	state, found, err := h.handler.Handle(ctx.Request.Context(), Query{
		Key: Key{
			Source:     parts[0],
			MetricName: parts[1],
			Env:        parts[2],
			Region:     parts[3],
		},
	})

	if err != nil {
		slog.ErrorContext(ctx.Request.Context(), "Failed to fetch series state",
			slog.String("key", keyStr),
			slog.String("error", err.Error()),
		)
		httpx.JSONError(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	if !found {
		httpx.JSONError(ctx, http.StatusNotFound, keyStr)
		return
	}

	ctx.JSON(http.StatusOK, state)
}
