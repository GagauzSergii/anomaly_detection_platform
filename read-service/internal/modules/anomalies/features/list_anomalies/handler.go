package list_anomalies

import "context"

// Handler is application layer for ListAnomalies query.
type Handler struct{ repo Repository }

func NewHandler(repo Repository) *Handler { return &Handler{repo: repo} }

func (h *Handler) Handle(ctx context.Context, q Query) ([]Row, error) {
	return h.repo.List(ctx, Filter{
		FromTS: q.FromTS,
		ToTS:   q.ToTS,

		Source:     q.Source,
		MetricName: q.MetricName,
		Env:        q.Env,
		Region:     q.Region,
		InstanceID: q.InstanceID,

		Limit:  q.Limit,
		Offset: q.Offset,
	})
}
