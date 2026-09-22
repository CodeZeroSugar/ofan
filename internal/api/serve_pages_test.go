package api

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

func (s *apiSuite) TestPageHandler_Servers() {
	ctx := context.Background()

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "secret123", false))

	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "bravo", "bob", testConfigJSON("bravo")))

	s.cfg.InformerManager.Registry.Upsert("alpha", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})
	s.cfg.InformerManager.Registry.Upsert("bravo", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})

	admin, err := s.cfg.Store.GetUserByUsername(ctx, "admin")
	s.Require().NoError(err)
	req := s.reqWithUser(admin, http.MethodGet, "/servers", "{}")

	s.rr = httptest.NewRecorder()
	s.cfg.HandlerServersPage(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), "alpha")
	s.Assert().Contains(s.rr.Body.String(), "bravo")
	s.Assert().Contains(s.rr.Body.String(), "running")
}
