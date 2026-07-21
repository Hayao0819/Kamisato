package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/Hayao0819/Kamisato/ayato/auth"
)

func TestDeviceApproveRedirectsToGitHub(t *testing.T) {
	handler, devices, signer := deviceHandler(t)
	if err := devices.CreateDevice("dc-3", "WXZB-CDFG", deviceCodeTTL); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	router := gin.New()
	router.GET("/approve", handler.DeviceApproveHandler)

	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/approve?user_code=wxzbcdfg", nil),
	)
	if response.Code != http.StatusFound {
		t.Fatalf("approve = %d, want 302: %s", response.Code, response.Body.String())
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if !strings.Contains(location.Host, "github.com") {
		t.Fatalf("Location host = %q, want github.com", location.Host)
	}
	var nonce string
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == oauthStateCookieName {
			nonce = cookie.Value
		}
	}
	if nonce == "" {
		t.Fatal("approve must set the binding cookie")
	}
	claims, err := signer.VerifyTyp(location.Query().Get("state"), auth.TypState)
	if err != nil {
		t.Fatalf("verify state: %v", err)
	}
	if !claims.Device ||
		claims.UserCode != "WXZB-CDFG" ||
		claims.Binding != auth.HashHex(nonce) {
		t.Fatalf("state = %+v, want device code bound to the cookie", claims)
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/approve?user_code=ZZZZ-ZZZZ", nil),
	)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown approve = %d, want 404", response.Code)
	}
}

func TestFinishAndDenyDeviceLogin(t *testing.T) {
	handler, devices, _ := deviceHandler(t)
	if err := devices.CreateDevice("dc-approve", "APPR-OVED", deviceCodeTTL); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/cb", nil)
	handler.finishDeviceLogin(
		context,
		&auth.Claims{Device: true, UserCode: "APPR-OVED"},
		githubUser{ID: testAdminID, Login: "alice"},
	)
	if response.Code != http.StatusOK {
		t.Fatalf("finishDeviceLogin = %d, want 200", response.Code)
	}
	if poll := pollDevice(t, handler, "dc-approve"); poll.Code != http.StatusOK {
		t.Fatalf("after approval, poll = %d, want 200", poll.Code)
	}

	if err := devices.CreateDevice("dc-deny", "DENY-CODE", deviceCodeTTL); err != nil {
		t.Fatalf("CreateDevice: %v", err)
	}
	response = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/cb", nil)
	handler.denyDeviceLogin(
		context,
		&auth.Claims{Device: true, UserCode: "DENY-CODE"},
	)
	if poll := pollDevice(t, handler, "dc-deny"); deviceErr(t, poll) != "access_denied" {
		t.Fatalf("after denial, poll = %q, want access_denied", deviceErr(t, poll))
	}
}
