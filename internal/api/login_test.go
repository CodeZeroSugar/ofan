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

	s.Assert().Equal(http.StatusOK, s.rr.Code)
	s.Assert().Contains(s.rr.Body.String(), `"token":`)
	s.Assert().Contains(s.rr.Body.String(), `"must_change_password":true`)
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
