package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/kubernetes/fake"
)

type apiSuite struct {
	suite.Suite
	cfg *ApiConfig
	rr  *httptest.ResponseRecorder
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

	s.cfg = &ApiConfig{
		Clientset:       fake.NewSimpleClientset(),
		InformerManager: &k8s.InformerManager{Registry: k8s.NewServerRegistry()},
		Namespace:       "ofan-dev",
		Store:           store,
		Auth:            auth.NewManager(store, []byte("testsecret")),
	}
}

func (s *apiSuite) reqWithUser(user *db.User, method, target, body string) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(auth.WithUser(req.Context(), user))
}

func (s *apiSuite) TestCreate_Valid() {
	body := `{"name":"alpha","password":"secret123"}`
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/create", body)
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"status":"provisioning"`)
	_, ok := s.cfg.InformerManager.Registry.Get("alpha")
	s.Assert().True(ok)
}

func (s *apiSuite) TestCreate_InvalidPayload() {
	body := `{not json}`
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/create", body)
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestCreate_ValidationError() {
	body := `{"name":"!!!"}`
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/create", body)
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestCreate_Conflict() {
	body := `{"name":"alpha","password":"secret123"}`
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/create", body)
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"status":"provisioning"`)
	_, ok := s.cfg.InformerManager.Registry.Get("alpha")
	s.Assert().True(ok)

	body = `{"name":"alpha","password":"secret123"}`
	req = s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/create", body)
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusConflict, s.rr.Code)
}

func (s *apiSuite) TestDelete_Valid() {
	s.cfg.InformerManager.Registry.Upsert("alpha", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/delete", s.cfg.HandlerDeleteGameServer)

	body := "{}"
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/delete", body)

	s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"status":"deleting"`)
	st, ok := s.cfg.InformerManager.Registry.Get("alpha")
	s.Require().True(ok)
	s.Assert().Equal("deleting", st.Status)
}

func (s *apiSuite) TestDelete_Unknown() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/delete", s.cfg.HandlerDeleteGameServer)

	body := "{}"
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/create", body)

	s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Require().Equal(http.StatusNotFound, s.rr.Code)
	_, ok := s.cfg.InformerManager.Registry.Get("ghost")
	s.Require().False(ok)
}

func (s *apiSuite) TestDelete_NoBody() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/delete", s.cfg.HandlerDeleteGameServer)

	body := ""
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/delete", body)
	req.Body = http.NoBody

	s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	_, ok := s.cfg.InformerManager.Registry.Get("ghost")
	s.Require().False(ok)
}

func (s *apiSuite) TestList() {
	s.cfg.InformerManager.Registry.Upsert("alpha", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})
	s.cfg.InformerManager.Registry.Upsert("bravo", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})
	body := "{}"
	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/create", body)

	s.rr = httptest.NewRecorder()
	s.cfg.HandlerListGameServers(s.rr, req)
	list := s.cfg.InformerManager.Registry.List()

	s.Require().Equal(http.StatusOK, s.rr.Code)
	s.Require().True(len(list) == 2)

	s.Assert().Contains(s.rr.Body.String(), `"alpha"`)
	s.Assert().Contains(s.rr.Body.String(), `"bravo"`)
}

func TestApiSuite(t *testing.T) {
	suite.Run(t, new(apiSuite))
}
