package api

import (
	"context"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/db"
)

func (c *ApiConfig) buildViewMap(ctx context.Context, userCtx *db.User) (map[string]ServerView, error) {
	viewMap := make(map[string]ServerView)
	stateList := c.InformerManager.Registry.List()
	if len(stateList) == 0 {
		return viewMap, nil
	}

	srvRecords, err := c.Store.ListServerConfigs(ctx)
	if err != nil {
		return viewMap, err
	}

	rowMap := make(map[string]db.ServerRecord, len(srvRecords))
	for _, r := range srvRecords {
		rowMap[r.Name] = r
	}

	for _, s := range stateList {
		rec := rowMap[s.Name]
		created := rec.CreatedAt
		if created.IsZero() {
			created = s.CreatedAt
		}
		viewMap[s.Name] = ServerView{
			ServerState:         s,
			DesiredState:        rec.DesiredState,
			Health:              deriveHealth(s.Status, rec.DesiredState, s.PodWaiting, rec.ConsecutiveFailures),
			ConsecutiveFailures: rec.ConsecutiveFailures,
			Uptime:              time.Since(created),
			Owner:               rec.Owner,
		}
	}

	if userCtx.IsRoot || userCtx.IsAdmin {
		return viewMap, nil
	}

	for n, s := range viewMap {
		if s.Owner != userCtx.Username {
			delete(viewMap, n)
		}
	}

	return viewMap, nil
}
