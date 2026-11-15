package healthcheck

import (
	"context"
)

func (r *Repo) Ping(ctx context.Context, requestId string) error {
	return r.sharedHolder.Sql.Ping()
}
