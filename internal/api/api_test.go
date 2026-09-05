package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CodeZeroSugar/ofan/internal/auth"
	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/CodeZeroSugar/ofan/internal/k8s"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		Poke:            func() {},
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
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/ghost/delete", body)

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

func (s *apiSuite) TestList_Empty() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/servers", s.cfg.HandlerListGameServers)

	ctx := context.Background()
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")

	body := "{}"
	req := s.reqWithUser(admin, http.MethodGet, "/api/v1/servers", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().Equal("{}", s.rr.Body.String())
}

func (s *apiSuite) TestList_OwnerFilter() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/servers", s.cfg.HandlerListGameServers)

	ctx := context.Background()
	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))
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

	bob, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := "{}"
	req := s.reqWithUser(bob, http.MethodGet, "/api/v1/servers", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	var servers map[string]ServerView
	s.Require().NoError(json.NewDecoder(s.rr.Body).Decode(&servers))

	for _, sv := range servers {
		s.Assert().Equal("bravo", sv.Name)
		s.Assert().Equal("bob", sv.Owner)
	}
}

func (s *apiSuite) TestStart_Valid() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/start", s.cfg.HandlerStartGameServer)

	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)

	err = s.cfg.Store.UpdateState(ctx, "alpha", "stopped")
	s.Require().NoError(err)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/start", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	srv, err := s.cfg.Store.GetServer(ctx, "alpha")
	s.Require().NoError(err)
	s.Assert().True(srv.DesiredState == "running")
}

func (s *apiSuite) TestStart_Conflict() {
	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/start", s.cfg.HandlerStartGameServer)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/start", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusConflict, s.rr.Code)
}

func (s *apiSuite) TestStart_NotFound() {
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/start", s.cfg.HandlerStartGameServer)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/ghost/start", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)
}

func (s *apiSuite) TestStart_Forbidden() {
	ctx := context.Background()
	err := s.cfg.Store.CreateUser(ctx, "bob", "fakehash", false)
	s.Require().NoError(err)
	err = s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)
	err = s.cfg.Store.UpdateState(ctx, "alpha", "stopped")
	s.Require().NoError(err)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/start", s.cfg.HandlerStartGameServer)

	bob, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	req := s.reqWithUser(bob, http.MethodPost, "/api/v1/servers/alpha/start", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func (s *apiSuite) TestStop_Valid() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/stop", s.cfg.HandlerStopGameServer)

	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/stop", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	srv, err := s.cfg.Store.GetServer(ctx, "alpha")
	s.Require().NoError(err)
	s.Assert().True(srv.DesiredState == "stopped")
}

func (s *apiSuite) TestStop_Forbidden() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/stop", s.cfg.HandlerStopGameServer)

	ctx := context.Background()
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))
	bob, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	req := s.reqWithUser(bob, http.MethodPost, "/api/v1/servers/alpha/stop", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func (s *apiSuite) TestStop_NotFound() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/stop", s.cfg.HandlerStopGameServer)

	ctx := context.Background()

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/stop", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)
}

func (s *apiSuite) TestStop_Conflict() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/stop", s.cfg.HandlerStopGameServer)

	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)
	err = s.cfg.Store.UpdateState(ctx, "alpha", "stopped")

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/stop", "{}")

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusConflict, s.rr.Code)

	srv, err := s.cfg.Store.GetServer(ctx, "alpha")
	s.Require().NoError(err)
	s.Assert().True(srv.DesiredState == "stopped")
}

func (s *apiSuite) TestTransfer_Valid() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/transfer", s.cfg.HandlerTransferServer)

	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)

	err = s.cfg.Store.CreateUser(ctx, "bob", "testhash", false)
	s.Require().NoError(err)

	body := `
	{"new_owner":"bob"}
	`
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/transfer", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	owner, err := s.cfg.Store.GetServerOwner(ctx, "alpha")
	s.Assert().Equal("bob", owner)
}

func (s *apiSuite) TestTransfer_BlankOwner() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/transfer", s.cfg.HandlerTransferServer)

	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)

	body := `
	{"new_owner":""}
	`
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/transfer", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestTransfer_ServerMissing() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/transfer", s.cfg.HandlerTransferServer)

	ctx := context.Background()

	err := s.cfg.Store.CreateUser(ctx, "bob", "testhash", false)
	s.Require().NoError(err)

	body := `
	{"new_owner":"bob"}
	`
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/transfer", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)
}

func (s *apiSuite) TestTransfer_TargetMissing() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/transfer", s.cfg.HandlerTransferServer)

	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)

	body := `
	{"new_owner":"ghost"}
	`
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/servers/alpha/transfer", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)
}

func (s *apiSuite) TestTransfer_Forbidden() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/servers/{server_name}/transfer", s.cfg.HandlerTransferServer)

	ctx := context.Background()
	err := s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha"))
	s.Require().NoError(err)

	err = s.cfg.Store.CreateUser(ctx, "bob", "testhash", false)
	s.Require().NoError(err)

	body := `
	{"new_owner":"bob"}
	`
	bob, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	req := s.reqWithUser(bob, http.MethodPost, "/api/v1/servers/alpha/transfer", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func serverLabels(name string) map[string]string {
	return map[string]string{
		"app":               name,
		k8s.LabelManagedBy:  k8s.ManagedByOfan,
		k8s.LabelServerName: name,
	}
}

func (s *apiSuite) TestPurge_Valid() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/system/purge-storage/{server_name}", s.cfg.HandlerDeletePVC)

	ctx := context.Background()
	_, err := s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Create(ctx, &v1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Labels: serverLabels("alpha"), Name: "alpha-pvc"}}, metav1.CreateOptions{})
	s.Require().NoError(err)

	_, err = s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Get(ctx, "alpha-pvc", metav1.GetOptions{})
	s.Require().NoError(err)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{"confirm":true}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/system/purge-storage/alpha", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	_, err = s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Get(ctx, "alpha-pvc", metav1.GetOptions{})
	s.Assert().Error(err)
}

func (s *apiSuite) TestPurge_NoConfirm() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/system/purge-storage/{server_name}", s.cfg.HandlerDeletePVC)

	ctx := context.Background()
	_, err := s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Create(ctx, &v1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Labels: serverLabels("alpha"), Name: "alpha-pvc"}}, metav1.CreateOptions{})
	s.Require().NoError(err)

	_, err = s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Get(ctx, "alpha-pvc", metav1.GetOptions{})
	s.Require().NoError(err)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/system/purge-storage/alpha", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestPurge_ServerExists() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/system/purge-storage/{server_name}", s.cfg.HandlerDeletePVC)

	ctx := context.Background()
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))

	_, err := s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Create(ctx, &v1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Labels: serverLabels("alpha"), Name: "alpha-pvc"}}, metav1.CreateOptions{})
	s.Require().NoError(err)

	_, err = s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Get(ctx, "alpha-pvc", metav1.GetOptions{})
	s.Require().NoError(err)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{"confirm":true}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/system/purge-storage/alpha", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusConflict, s.rr.Code)

	_, err = s.cfg.Clientset.CoreV1().PersistentVolumeClaims(s.cfg.Namespace).Get(ctx, "alpha-pvc", metav1.GetOptions{})
	s.Assert().NoError(err)
}

func (s *apiSuite) TestPurge_NoPvc() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/system/purge-storage/{server_name}", s.cfg.HandlerDeletePVC)

	ctx := context.Background()

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{"confirm":true}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/system/purge-storage/alpha", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)
}

func (s *apiSuite) TestChangePasswordValid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/password", s.cfg.HandlerChangePassword)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "changeme", true))

	bob, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := `{"new_password":"pass123"}`
	req := s.reqWithUser(bob, http.MethodPost, "/api/v1/auth/password", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	s.Assert().False(user.MustChangePassword)
}

func (s *apiSuite) TestChangePassword_Blank() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/password", s.cfg.HandlerChangePassword)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "changeme", true))

	bob, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := `{"new_password":""}`
	req := s.reqWithUser(bob, http.MethodPost, "/api/v1/auth/password", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusBadRequest, s.rr.Code)
	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	s.Assert().True(user.MustChangePassword)
}

func (s *apiSuite) TestLogout() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/logout", s.cfg.HandlerLogout)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "changeme", true))

	body := "{}"
	bob, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	req := s.reqWithUser(bob, http.MethodPost, "/api/v1/auth/logout", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
}

func (s *apiSuite) TestCreateUser_Valid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/create", s.cfg.HandlerCreateUser)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{
	"username":"bob",
	"password":"p",
	"is_admin":false
	}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/create", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusCreated, s.rr.Code)

	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)

	s.Assert().Equal("bob", user.Username)
}

func (s *apiSuite) TestCreateUser_Blank() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/create", s.cfg.HandlerCreateUser)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{
	"username":"",
	"password":""
	}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/create", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestCreateUser_Exists() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/create", s.cfg.HandlerCreateUser)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{
	"username":"bob",
	"password":"p",
	"is_admin":false
	}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/create", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusCreated, s.rr.Code)

	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)

	s.Assert().Equal("bob", user.Username)

	req = s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/create", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusConflict, s.rr.Code)
}

func (s *apiSuite) TestDeleteUser_Valid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/delete/{username}", s.cfg.HandlerDeleteUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/delete/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)
}

func (s *apiSuite) TestDeleteUser_Self() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/delete/{username}", s.cfg.HandlerDeleteUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", true))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/delete/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func (s *apiSuite) TestDeleteUser_OwnsServers() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/delete/{username}", s.cfg.HandlerDeleteUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", true))
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "bob", testConfigJSON("alpha")))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/delete/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusConflict, s.rr.Code)
}

func (s *apiSuite) TestDeleteUser_NotFound() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/delete/{username}", s.cfg.HandlerDeleteUser)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/delete/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)
}

func (s *apiSuite) TestSuspendUser_Valid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/suspend/{username}", s.cfg.HandlerSuspendUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/suspend/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	s.Assert().True(user.IsSuspended)
}

func (s *apiSuite) TestSuspendUser_Self() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/suspend/{username}", s.cfg.HandlerSuspendUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", true))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/suspend/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func (s *apiSuite) TestUnsuspendUser_Valid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/unsuspend/{username}", s.cfg.HandlerUnsuspendUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))
	s.Require().NoError(s.cfg.Store.SuspendUser(ctx, "bob"))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/unsuspend/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	s.Assert().False(user.IsSuspended)
}

func (s *apiSuite) TestPromoteUser_Valid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/promote/{username}", s.cfg.HandlerPromoteUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/promote/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	s.Assert().True(user.IsAdmin)
}

func (s *apiSuite) TestPromoteDemote_IsRoot() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/promote/{username}", s.cfg.HandlerPromoteUser)
	mux.HandleFunc("POST /api/v1/admin/users/demote/{username}", s.cfg.HandlerDemoteUser)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/promote/admin", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)

	req = s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/demote/admin", body)
	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func (s *apiSuite) TestDemoteUser_Valid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/demote/{username}", s.cfg.HandlerDemoteUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", true))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/demote/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	s.Assert().False(user.IsAdmin)
}

func (s *apiSuite) TestDemoteUser_Self() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/demote/{username}", s.cfg.HandlerDemoteUser)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", true))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/demote/bob", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func (s *apiSuite) TestPromoteDemote_NotFound() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/promote/{username}", s.cfg.HandlerPromoteUser)
	mux.HandleFunc("POST /api/v1/admin/users/demote/{username}", s.cfg.HandlerDemoteUser)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/promote/ghost", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)

	req = s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/demote/ghost", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusNotFound, s.rr.Code)
}

func (s *apiSuite) TestResetPassword_Valid() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/reset_password", s.cfg.HandlerResetPassword)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{"username":"bob","temp_password":"t"}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/reset_password", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	user, err := s.cfg.Store.GetUserByUsername(ctx, "bob")
	s.Require().NoError(err)
	s.Assert().True(user.MustChangePassword)
}

func (s *apiSuite) TestResetPassword_Blank() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/admin/users/reset_password", s.cfg.HandlerResetPassword)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{"username":"","temp_password":""}`
	req := s.reqWithUser(admin, http.MethodPost, "/api/v1/admin/users/reset_password", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestListUsers() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/users", s.cfg.HandlerListUsers)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodGet, "/api/v1/admin/users", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	var users []db.User
	s.Require().NoError(json.NewDecoder(s.rr.Body).Decode(&users))

	var names []string
	for _, u := range users {
		names = append(names, u.Username)
	}

	s.Assert().Contains(names, "admin")
	s.Assert().Contains(names, "bob")
}

func (s *apiSuite) TestListUsersEmpty() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/admin/users", s.cfg.HandlerListUsers)

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	body := `{}`
	req := s.reqWithUser(admin, http.MethodGet, "/api/v1/admin/users", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusOK, s.rr.Code)

	var users []db.User
	s.Require().NoError(json.NewDecoder(s.rr.Body).Decode(&users))

	var names []string
	for _, u := range users {
		names = append(names, u.Username)
	}

	s.Assert().Contains(names, "admin")
	s.Assert().True(len(names) == 1)
}

func (s *apiSuite) TestDeletePVC_AdminNoOwn() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/servers/{server_name}/delete", s.cfg.HandlerDeleteGameServer)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", true))

	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := `{
	"delete_storage":true
	}`
	req := s.reqWithUser(admin, http.MethodGet, "/api/v1/servers/alpha/delete", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), "must transfer ownership first")
}

func (s *apiSuite) TestDelete_NonOwner() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/servers/{server_name}/delete", s.cfg.HandlerDeleteGameServer)

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))

	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))

	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "bob")
	body := `{
	"delete_storage":true
	}`
	req := s.reqWithUser(admin, http.MethodGet, "/api/v1/servers/alpha/delete", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}

func (s *apiSuite) TestGetGameServer() {
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/servers/{server_name}", s.cfg.HandlerGetGameServer)

	tests := []struct {
		name           string
		readBy         string
		srvName        string
		isRow          bool
		isReg          bool
		expectedStatus int
		expectedHealth string
	}{
		{
			name:           "owner reads own",
			readBy:         "bob",
			srvName:        "bravo",
			isRow:          true,
			isReg:          true,
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
		},
		{
			name:           "admin reads others",
			readBy:         "admin",
			srvName:        "bravo",
			isRow:          true,
			isReg:          true,
			expectedStatus: http.StatusOK,
			expectedHealth: "healthy",
		},
		{
			name:           "non-owner denied",
			readBy:         "bob",
			srvName:        "alpha",
			isRow:          true,
			isReg:          true,
			expectedStatus: http.StatusForbidden,
			expectedHealth: "",
		},
		{
			name:           "unknown name",
			readBy:         "bob",
			srvName:        "ghost",
			isRow:          false,
			isReg:          false,
			expectedStatus: http.StatusNotFound,
			expectedHealth: "",
		},
		{
			name:           "orphan (rowless entry)",
			readBy:         "bob",
			srvName:        "charlie",
			isRow:          false,
			isReg:          true,
			expectedStatus: http.StatusNotFound,
			expectedHealth: "",
		},
		{
			name:           "zombie row",
			readBy:         "bob",
			srvName:        "delta",
			isRow:          true,
			isReg:          false,
			expectedStatus: http.StatusOK,
			expectedHealth: "degraded",
		},
	}

	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", "pass123", false))
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "alpha", "admin", testConfigJSON("alpha")))
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "bravo", "bob", testConfigJSON("bravo")))
	s.Require().NoError(s.cfg.Store.CreateServer(ctx, "delta", "bob", testConfigJSON("delta")))

	s.cfg.InformerManager.Registry.Upsert("alpha", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})
	s.cfg.InformerManager.Registry.Upsert("bravo", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})
	s.cfg.InformerManager.Registry.Upsert("charlie", func(st *k8s.ServerState) {
		st.Namespace = "ofan-dev"
		st.Status = "running"
	})

	for _, tc := range tests {
		s.rr = httptest.NewRecorder()
		switch {
		case tc.isRow:
			reader, _ := s.cfg.Store.GetUserByUsername(ctx, tc.readBy)
			req := s.reqWithUser(reader, http.MethodGet, fmt.Sprintf("/api/v1/servers/%s", tc.srvName), "{}")
			mux.ServeHTTP(s.rr, req)
			s.Assert().Equal(tc.expectedStatus, s.rr.Code)

			if tc.expectedStatus != http.StatusOK {
				continue
			}

			var view ServerView
			s.Require().NoError(json.NewDecoder(s.rr.Body).Decode(&view))
			s.Assert().NotNil(view.ServerState)
			s.Assert().Equal(tc.expectedHealth, view.Health)
			s.Assert().True(view.Uptime > 0)
		default:
			reader, _ := s.cfg.Store.GetUserByUsername(ctx, tc.readBy)
			req := s.reqWithUser(reader, http.MethodGet, fmt.Sprintf("/api/v1/servers/%s", tc.srvName), "{}")
			mux.ServeHTTP(s.rr, req)
			s.Assert().Equal(tc.expectedStatus, s.rr.Code)
		}
	}
}

func (s *apiSuite) TestDeriveHealth() {
	tests := []struct {
		name     string
		status   string
		desired  string
		failures int
		expected string
	}{
		{
			name:     "healthy",
			status:   "provisioning",
			desired:  "running",
			failures: 0,
			expected: "healthy",
		},
		{
			name:     "failed",
			status:   "provisioning",
			desired:  "running",
			failures: 5,
			expected: "failed",
		},
		{
			name:     "healthy deleting",
			status:   "deleting",
			desired:  "deleting",
			failures: 0,
			expected: "healthy",
		},
		{
			name:     "degraded",
			status:   "unknown",
			desired:  "sdfasdfad",
			failures: 0,
			expected: "degraded",
		},
	}

	for _, tc := range tests {
		s.Assert().Equal(tc.expected, deriveHealth(tc.status, tc.desired, tc.failures))
	}
}

func TestApiSuite(t *testing.T) {
	suite.Run(t, new(apiSuite))
}
