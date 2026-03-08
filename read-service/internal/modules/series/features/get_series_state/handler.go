package get_series_state

import "context"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

func (handler *Handler) Handle(ctx context.Context, query Query) (State, bool, error) {
	return handler.repo.Get(ctx, query.Key)
}
