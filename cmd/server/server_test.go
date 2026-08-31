package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"testing"

	"github.com/CodeZeroSugar/ofan/internal/api"
	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes/fake"
)

type apiSuite struct {
	suite.Suite
	cfg *api.ApiConfig
	rr  *httptest.ResponseRecorder
	s   *server
}

func testConfigJSON(name string) string {
	return fmt.Sprintf(`{"core_settings":{"server_name":%q}}`, name)
}

func (s *apiSuite) SetupTest() {
	adminHash, err := auth.HashPassword("testpass")
	s.Require().NoError(err)

	store, err := db.NewStore(context.Background(), "file::memory:", "admin", adminHash)
	s.Require().NoError(err)
	s.T().Cleanup(func() { store.Close() })

	s.cfg = &api.ApiConfig{
		Clientset:       fake.NewSimpleClientset(),
		InformerManager: &k8s.InformerManager{Registry: k8s.NewServerRegistry()},
		Namespace:       "ofan-dev",
		Store:           store,
		Auth:            auth.NewManager(store, []byte("testsecret")),
		Poke:            func() {},
	}
}

func (s *apiSuite) reqWithUser(user *db.User, method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(auth.WithUser(req.Context(), user))
}

func (s *apiSuite) TestResetDatabase() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	mux := http.NewServeMux()
	cfg := loadConfig()
	cfg.RootPass = "testpass"
	srv, _ := newServer("8080", s.cfg, &cfg, cancel)
	s.s = srv

	mux.HandleFunc("POST /api/v1/system/reset", s.s.handlerResetDatabase)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))
	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "caroline", "pass123", false))

	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "bravo", "bob", testConfigJSON("alpha")))
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "charlie", "caroline", testConfigJSON("alpha")))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/system/reset", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	users, _ := s.cfg.Store.ListUsers(ctx)
	s.Assert().True(len(users) == 1)

	servers, _ := s.cfg.Store.ListServerConfigs(ctx)
	s.Assert().True(len(servers) == 0)
}

func TestApiSuite(t *testing.T) {
	suite.Run(t, new(apiSuite))
}
