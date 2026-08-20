package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type apiSuite struct {
	suite.Suite
	cfg *ApiConfig
	rr  *httptest.ResponseRecorder
}

func (s *apiSuite) SetupTest() {
	s.cfg = &ApiConfig{
		Clientset:       fake.NewSimpleClientset(),
		InformerManager: &k8s.InformerManager{Registry: k8s.NewServerRegistry()},
		Namespace:       "ofan-dev",
	}
}

func (s *apiSuite) TestCreate_Valid() {
	body := `{"name":"alpha","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"status":"provisioning"`)
	_, ok := s.cfg.InformerManager.Registry.Get("alpha")
	s.Assert().True(ok)
}

func (s *apiSuite) TestCreate_InvalidPayload() {
	body := `{not json}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestCreate_ValidationError() {
	body := `{"name":"!!!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestCreate_Conflict() {
	body := `{"name":"alpha","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"status":"provisioning"`)
	_, ok := s.cfg.InformerManager.Registry.Get("alpha")
	s.Assert().True(ok)

	body = `{"name":"alpha","password":"secret123"}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusConflict, s.rr.Code)
}

func (s *apiSuite) TestCreate_PortCollision() {
	body := `{"name":"alpha","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)
	s.cfg.InformerManager.Registry.Upsert("alpha", func(st *k8s.ServerState) {
		st.NodePort = 30001
	})

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"status":"provisioning"`)
	_, ok := s.cfg.InformerManager.Registry.Get("alpha")
	s.Assert().True(ok)

	body = `{
  "name": "alpha",
  "password": "secret123",
  "server_opts": {
    "node_port": 30001,
    "config": {
      "core_settings": {
        "server_name": "alpha",
        "world_name": "Dedicated",
        "server_pass": "secret123",
        "server_port": 2456,
        "server_public": false
      }
    }
  }
}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusConflict, s.rr.Code)
}

func (s *apiSuite) TestCreate_ProvisionFails() {
	fakeClient := fake.NewSimpleClientset()
	fakeClient.PrependReactor("create", "deployments",
		func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("boom")
		})
	s.cfg.Clientset = fakeClient

	body := `{"name":"alpha","password":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/create", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerCreateGameServer(s.rr, req)

	s.Require().Equal(http.StatusInternalServerError, s.rr.Code)
	_, ok := s.cfg.InformerManager.Registry.Get("alpha")
	s.Assert().False(ok)
}

func (s *apiSuite) TestDelete_Valid() {
	s.cfg.InformerManager.Registry.Upsert("alpha", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/delete", s.cfg.HandlerDeleteGameServer)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/alpha/delete", strings.NewReader("{}"))
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

	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/alpha/delete", strings.NewReader("{}"))
	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Require().Equal(http.StatusAccepted, s.rr.Code)
	_, ok := s.cfg.InformerManager.Registry.Get("ghost")
	s.Require().False(ok)
}

func (s *apiSuite) TestDelete_NoBody() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/delete", s.cfg.HandlerDeleteGameServer)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers/alpha/delete", http.NoBody)
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
	req := httptest.NewRequest(http.MethodPost, "/api/v1/servers", strings.NewReader("{}"))
	s.rr = httptest.NewRecorder()
	s.cfg.HandlerListGameServers(s.rr, req)
	list := s.cfg.InformerManager.Registry.List()

	s.Require().Equal(http.StatusOK, s.rr.Code)
	s.Require().True(len(list) == 2)
}

func TestApiSuite(t *testing.T) {
	suite.Run(t, new(apiSuite))
}
