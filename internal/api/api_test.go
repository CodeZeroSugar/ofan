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

func TestApiSuite(t *testing.T) {
	suite.Run(t, new(apiSuite))
}
