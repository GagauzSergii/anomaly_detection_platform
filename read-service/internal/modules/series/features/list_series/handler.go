package list_series

import "context"

// Handler is the application layer for ListSeries query.
type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Handle(ctx context.Context, q Query) ([]Row, error) {
	return h.repo.List(ctx, Filter{
		Source:     q.Source,
		MetricName: q.MetricName,
		Env:        q.Env,
		Region:     q.Region,
		Limit:      q.Limit,
	})
}
