// Package handler_test session_test.go - 可撤销会话 HTTP 契约测试
//
// 通过测试路由验证：
//   - 登录响应下发 access_token 与 httpOnly refresh cookie
//   - refresh 端点轮换 cookie 并签发新 access token
//   - 撤销会话后，即使 access token 未过期也立即失效
//   - logout 只撤销当前设备，logout-all 撤销所有设备
package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testPassword = "password123"

// loginAndCaptureCookies 注册并登录用户，返回 access token 与 Set-Cookie 头。
func loginAndCaptureCookies(t *testing.T, username string) (string, string) {
	t.Helper()
	// 注册（用户已存在时忽略冲突，测试可能复用同一用户名）。
	registerBody := fmt.Sprintf(`{"username":"%s","email":"%s@test.com","password":"%s"}`, username, username, testPassword)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/register", strings.NewReader(registerBody))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusCreated && w.Code != http.StatusConflict {
		t.Fatalf("register %s: expected 201 or 409, got %d: %s", username, w.Code, w.Body.String())
	}

	body := fmt.Sprintf(`{"username":"%s","password":"%s"}`, username, testPassword)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("login response parse: %v", err)
	}
	setCookie := w.Header().Get("Set-Cookie")
	if !strings.Contains(setCookie, "mo_refresh=") {
		t.Fatalf("login must set refresh cookie, got %q", setCookie)
	}
	if !strings.Contains(setCookie, "HttpOnly") {
		t.Fatal("refresh cookie must be HttpOnly")
	}
	return resp.Data.AccessToken, setCookie
}

func TestLoginSetsHttpOnlyRefreshCookieAndProfileWorks(t *testing.T) {
	token, _ := loginAndCaptureCookies(t, "cookie-login")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("profile: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRefreshEndpointRotatesCookieAndAccessToken(t *testing.T) {
	_, firstCookie := loginAndCaptureCookies(t, "refresh-user")

	// 用第一次登录的 refresh cookie 调用 refresh。
	refresh := func(cookie string) (*httptest.ResponseRecorder, string) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", nil)
		req.Header.Set("Cookie", cookie)
		testRouter.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("refresh: expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				AccessToken string `json:"access_token"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("refresh response parse: %v", err)
		}
		if resp.Data.AccessToken == "" {
			t.Fatal("refresh must return an access token")
		}
		newCookie := w.Header().Get("Set-Cookie")
		if newCookie == "" || !strings.Contains(newCookie, "mo_refresh=") {
			t.Fatalf("refresh must rotate the cookie, got %q", newCookie)
		}
		return w, newCookie
	}

	w, rotatedCookie := refresh(firstCookie)
	// 新 cookie 与旧 cookie 的 token 值不同（轮换）。
	if strings.Split(rotatedCookie, ";")[0] == strings.Split(firstCookie, ";")[0] {
		t.Fatal("refresh cookie must be rotated to a new value")
	}

	// 用刷新后返回的 access token 请求 profile，确认可用。
	_, newToken := refreshWithBody(t, rotatedCookie)
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+newToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("profile with rotated token: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

// refreshWithBody 用 cookie 刷新并返回新的 access token。
func refreshWithBody(t *testing.T, cookie string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", nil)
	req.Header.Set("Cookie", cookie)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("refresh response parse: %v", err)
	}
	return w, resp.Data.AccessToken
}

func TestRefreshAcceptsTokenInRequestBody(t *testing.T) {
	_, cookie := loginAndCaptureCookies(t, "body-token-user")
	token := strings.SplitN(cookie, "mo_refresh=", 2)[1]
	token = strings.Split(token, ";")[0]

	// 从 Set-Cookie 头提取 refresh token 值，通过请求体提交（API 客户端场景）。
	body := fmt.Sprintf(`{"refresh_token":"%s"}`, token)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("refresh via body: expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogoutRevokesSessionAndAccessTokenImmediately(t *testing.T) {
	token, cookie := loginAndCaptureCookies(t, "logout-session")

	// 登出前 profile 可用。
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("profile before logout: expected 200, got %d", w.Code)
	}

	// 登出（携带 access token + refresh cookie）。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", cookie)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// access token 立即失效（服务端会话已撤销）。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("profile after logout: expected 401, got %d: %s", w.Code, w.Body.String())
	}

	// 旧 refresh cookie 也无法再刷新。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", nil)
	req.Header.Set("Cookie", cookie)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after logout: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogoutWorksViaRefreshCookieWhenAccessTokenIsGone(t *testing.T) {
	_, cookie := loginAndCaptureCookies(t, "expired-token-logout")

	// 模拟 access token 已过期/丢失：只带 refresh cookie 调 logout。
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/logout", nil)
	req.Header.Set("Cookie", cookie)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout with cookie only: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 会话已被撤销：cookie 无法再刷新。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/users/refresh", nil)
	req.Header.Set("Cookie", cookie)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("refresh after cookie logout: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogoutAllRevokesEveryDevice(t *testing.T) {
	firstToken, firstCookie := loginAndCaptureCookies(t, "logout-all-user")
	secondToken, _ := loginAndCaptureCookies(t, "logout-all-user")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/logout-all", nil)
	req.Header.Set("Authorization", "Bearer "+firstToken)
	req.Header.Set("Cookie", firstCookie)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("logout-all: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 第二个设备的 access token 也立即失效。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+secondToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("profile after logout-all: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChangePasswordRevokesOtherSessionsAtHTTPLevel(t *testing.T) {
	token, cookie := loginAndCaptureCookies(t, "change-pass-session")
	otherToken, _ := loginAndCaptureCookies(t, "change-pass-session")

	// 修改密码（保持当前会话）。
	body := `{"old_password":"password123","new_password":"password456"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/change-password", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Cookie", cookie)
	req.Header.Set("Content-Type", "application/json")
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("change-password: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 当前会话仍有效。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("current session after password change: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// 其他会话已撤销。
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	testRouter.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("other session after password change: expected 401, got %d: %s", w.Code, w.Body.String())
	}
}
