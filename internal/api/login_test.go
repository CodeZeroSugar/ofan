package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/CodeZeroSugar/ofan/internal/auth"
)

func (s *apiSuite) TestLogin_Valid() {
	body := `{"username": "admin", "password": "testpass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerLogin(s.rr, req)

	cookie := s.rr.Header().Get("Set-Cookie")
	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"token":`)
	s.Assert().Contains(s.rr.Body.String(), `"must_change_password":true`)
	s.Assert().NotEqual("", cookie)
}

func (s *apiSuite) TestLoginLogout_ClearCookie() {
	body := `{"username": "admin", "password": "testpass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerLogin(s.rr, req)
	cookie := s.rr.Header().Get("Set-Cookie")
	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"token":`)
	s.Assert().Contains(s.rr.Body.String(), `"must_change_password":true`)
	s.Assert().NotEqual("", cookie)

	ctx := context.Background()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/auth/logout", s.cfg.HandlerLogout)

	body = "{}"
	admin, _ := s.cfg.Store.GetUserByUsername(ctx, "admin")
	req = s.reqWithUser(admin, http.MethodPost, "/api/v1/auth/logout", body)

	s.rr = httptest.NewRecorder()
	mux.ServeHTTP(s.rr, req)
	cookie = s.rr.Header().Get("Set-Cookie")

	s.Assert().Equal("ofan_session=; Path=/; Max-Age=0", cookie)
	s.Assert().Equal(http.StatusOK, s.rr.Code)
}

func (s *apiSuite) TestLogin_BadPassword() {
	body := `{"username": "admin", "password": "testingpass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerLogin(s.rr, req)

	s.Assert().Equal(http.StatusUnauthorized, s.rr.Code)
}

func (s *apiSuite) TestLogin_UnknownUser() {
	body := `{"username": "bob", "password": "testingpass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerLogin(s.rr, req)

	s.Assert().Equal(http.StatusUnauthorized, s.rr.Code)
}

func (s *apiSuite) TestLogin_MalformedJSON() {
	body := `{not json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerLogin(s.rr, req)

	s.Assert().Equal(http.StatusBadRequest, s.rr.Code)
}

func (s *apiSuite) TestLogin_Suspended() {
	ctx := context.Background()
	testHash, err := auth.HashPassword("testpass")
	s.Require().NoError(err)
	s.Require().NoError(s.cfg.Store.CreateUser(ctx, "bob", testHash, false))
	s.Require().NoError(s.cfg.Store.SuspendUser(ctx, "bob"))

	body := `{"username": "bob", "password": "testpass"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	s.rr = httptest.NewRecorder()

	s.cfg.HandlerLogin(s.rr, req)

	s.Assert().Equal(http.StatusForbidden, s.rr.Code)
}
