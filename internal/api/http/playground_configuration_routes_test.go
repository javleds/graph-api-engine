package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "github.com/javleds/api-engine/internal/api/http"
	platformconfig "github.com/javleds/api-engine/internal/platform/config"
	security "github.com/javleds/api-engine/internal/security/infrastructure"
)

func TestPlaygroundConfigurationShowsEffectiveSafeValues(t *testing.T) {
	session, err := security.NewPlaygroundSessionAuthenticator("admin", "password", []byte("01234567890123456789012345678901"), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	configuration := platformconfig.Default()
	configuration.HTTP.Address = "0.0.0.0:9010"
	configuration.Security.PrincipalsFile = "/run/secrets/principals"
	configuration.Security.Playground.Password = "password"
	handler := (&api.Server{Configuration: &configuration, PlaygroundSession: session}).Handler()

	login := httptest.NewRequest(http.MethodPost, "/playground/api/session", strings.NewReader(`{"username":"admin","password":"password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login: %d %s", loginResponse.Code, loginResponse.Body.String())
	}
	cookie := loginResponse.Result().Cookies()[0]

	request := httptest.NewRequest(http.MethodGet, "/playground/api/configuration", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("configuration: %d %s", response.Code, response.Body.String())
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	current := body["current"]
	httpConfig := current["http"].(map[string]any)
	if httpConfig["address"] != "0.0.0.0:9010" {
		t.Fatalf("address = %v", httpConfig["address"])
	}
	securityConfig := current["security"].(map[string]any)
	if _, found := securityConfig["principalsFile"]; found {
		t.Fatal("principals file was exposed")
	}
	if _, found := securityConfig["playground"]; found {
		t.Fatal("playground credentials were exposed")
	}
}
