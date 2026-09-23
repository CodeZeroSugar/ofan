package api

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
)

func (s *apiSuite) TestPageHandler_ServersValid() {
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

func (s *apiSuite) TestPageHandler_ServersEmpty() {
	ctx := context.Background()
	admin, err := s.cfg.Store.GetUserByUsername(ctx, "admin")
	s.Require().NoError(err)
	req := s.reqWithUser(admin, http.MethodGet, "/servers", "{}")

	s.rr = httptest.NewRecorder()
	s.cfg.HandlerServersPage(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), "No servers to display")
}

func (s *apiSuite) TestPageHandler_ServersNonAdminCtx() {
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

	bob, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	req := s.reqWithUser(bob, http.MethodGet, "/servers", "{}")

	s.rr = httptest.NewRecorder()
	s.cfg.HandlerServersPage(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().NotContains(s.rr.Body.String(), "alpha")
	s.Assert().Contains(s.rr.Body.String(), "bravo")
	s.Assert().Contains(s.rr.Body.String(), "running")
}

func (s *apiSuite) TestPageHandler_ServersNoUser() {
	ctx := context.Background()

	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))

	s.cfg.InformerManager.Registry.Upsert("alpha", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})

	req := httptest.NewRequest(http.MethodGet, "/servers", http.NoBody)

	s.rr = httptest.NewRecorder()
	s.cfg.HandlerServersPage(s.rr, req)

	s.Assert().Equal(http.StatusUnauthorized, s.rr.Code)
}

func (s *apiSuite) TestPageHandler_ServersStoreError() {
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

	s.Require().NoError(s.cfg.Store.Close())

	s.rr = httptest.NewRecorder()
	s.cfg.HandlerServersPage(s.rr, req)

	s.Assert().Equal(http.StatusInternalServerError, s.rr.Code)
}

func (s *apiSuite) TestPageHandler_LoginNothing() {
	req := httptest.NewRequest(http.MethodGet, "/login", http.NoBody)
	s.rr = httptest.NewRecorder()
	s.cfg.HandlerLoginPage(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().Equal(s.rr.Header().Get("Content-Type"), "text/html; charset=utf-8")
	s.Assert().Contains(s.rr.Body.String(), `hx-post="/api/v1/auth/login"`)
	s.Assert().Contains(s.rr.Body.String(), "json-enc")
	s.Assert().Contains(s.rr.Body.String(), `name="username"`)
	s.Assert().Contains(s.rr.Body.String(), `name="password"`)
	s.Assert().Contains(s.rr.Body.String(), `id="change-password-form"`)
	s.Assert().Contains(s.rr.Body.String(), `visibility: hidden`)
}
