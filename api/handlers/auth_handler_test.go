package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/geolens/geolens/internal/auth"
	"github.com/gin-gonic/gin"
)

func testAuthRouter(t *testing.T) (*gin.Engine, *auth.Engine) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	e, err := auth.NewEngine(filepath.Join(t.TempDir(), "auth.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Seed("admin@example.com", "password123", auth.RoleAdmin, "demo-account"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Register("user@example.com", "password123"); err != nil {
		t.Fatal(err)
	}
	s := &Server{auth: e, adminToken: "adm_test_token"}
	r := gin.New()
	r.POST("/api/v1/auth/login", s.authLogin)
	r.POST("/api/v1/auth/register", s.authRegister)
	r.GET("/api/v1/auth/me", s.RequireAuth(), s.authMe)
	r.POST("/api/v1/auth/logout", s.RequireAuth(), s.authLogout)
	r.GET("/protected", s.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"account": currentAccount(c)})
	})
	r.GET("/admin", s.RequireAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r, e
}

func jsonRequest(r *gin.Engine, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func loginToken(t *testing.T, r *gin.Engine, email, password string) string {
	t.Helper()
	w := jsonRequest(r, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": email, "password": password}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("登录应成功，实际 %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Token == "" {
		t.Fatalf("登录响应缺少 token: %s", w.Body.String())
	}
	return out.Token
}

func TestProtectedEndpointsRequireSession(t *testing.T) {
	r, _ := testAuthRouter(t)

	if w := jsonRequest(r, http.MethodGet, "/protected", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问受保护端点应 401，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodGet, "/protected", nil,
		map[string]string{"Authorization": "Bearer forged"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("伪造 token 应 401，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodGet, "/protected", nil,
		map[string]string{"Authorization": "Basic admin"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("非 Bearer 形式应 401，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "admin@example.com", "password": "wrong"}, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("错误密码应 401，实际 %d", w.Code)
	}

	token := loginToken(t, r, "admin@example.com", "password123")
	w := jsonRequest(r, http.MethodGet, "/protected", nil,
		map[string]string{"Authorization": "Bearer " + token})
	if w.Code != http.StatusOK {
		t.Fatalf("合法会话应 200，实际 %d", w.Code)
	}
}

func TestAccountIDBoundToSession(t *testing.T) {
	r, e := testAuthRouter(t)
	adminToken := loginToken(t, r, "admin@example.com", "password123")

	userToken := loginToken(t, r, "user@example.com", "password123")
	expected := ""
	for _, u := range e.Users() {
		if u.Email == "user@example.com" {
			expected = u.AccountID
		}
	}
	if expected == "" {
		t.Fatal("未找到普通用户")
	}

	// 客户端伪造 account_id 不得改变服务端使用的账户。
	w := jsonRequest(r, http.MethodGet, "/protected?account_id=someone-else", nil,
		map[string]string{"Authorization": "Bearer " + userToken})
	var got struct {
		Account string `json:"account"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Account != expected || got.Account == "demo-account" {
		t.Fatalf("账户必须来自会话，实际 %q，期望 %q", got.Account, expected)
	}

	// 两个用户的会话各自绑定自己的账户。
	adminW := jsonRequest(r, http.MethodGet, "/protected", nil,
		map[string]string{"Authorization": "Bearer " + adminToken})
	var adminGot struct {
		Account string `json:"account"`
	}
	_ = json.Unmarshal(adminW.Body.Bytes(), &adminGot)
	if adminGot.Account != "demo-account" || adminGot.Account == got.Account {
		t.Fatalf("不同用户应绑定不同账户: admin=%q user=%q", adminGot.Account, got.Account)
	}
}

func TestAdminBoundary(t *testing.T) {
	r, _ := testAuthRouter(t)
	userToken := loginToken(t, r, "user@example.com", "password123")
	adminToken := loginToken(t, r, "admin@example.com", "password123")

	if w := jsonRequest(r, http.MethodGet, "/admin", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("未登录访问管理端点应 401，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodGet, "/admin", nil,
		map[string]string{"Authorization": "Bearer " + userToken}); w.Code != http.StatusForbidden {
		t.Fatalf("普通用户访问管理端点应 403，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodGet, "/admin", nil,
		map[string]string{"Authorization": "Bearer " + adminToken}); w.Code != http.StatusOK {
		t.Fatalf("admin 角色应放行，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodGet, "/admin", nil,
		map[string]string{"X-Admin-Token": "adm_test_token"}); w.Code != http.StatusOK {
		t.Fatalf("X-Admin-Token 机器凭据应放行，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodGet, "/admin", nil,
		map[string]string{"X-Admin-Token": "wrong"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("错误 admin token 应 401，实际 %d", w.Code)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	r, _ := testAuthRouter(t)
	token := loginToken(t, r, "admin@example.com", "password123")
	h := map[string]string{"Authorization": "Bearer " + token}

	if w := jsonRequest(r, http.MethodGet, "/api/v1/auth/me", nil, h); w.Code != http.StatusOK {
		t.Fatalf("有效会话应能读取 /me，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodPost, "/api/v1/auth/logout", nil, h); w.Code != http.StatusOK {
		t.Fatalf("登出应成功，实际 %d", w.Code)
	}
	if w := jsonRequest(r, http.MethodGet, "/api/v1/auth/me", nil, h); w.Code != http.StatusUnauthorized {
		t.Fatalf("登出后同一 token 应立即失效，实际 %d", w.Code)
	}
}

func TestRegisterCreatesIndependentAccount(t *testing.T) {
	r, e := testAuthRouter(t)
	before := len(e.Users())
	w := jsonRequest(r, http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "new@example.com", "password": "password123"}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("注册应成功，实际 %d: %s", w.Code, w.Body.String())
	}
	var out struct {
		Token string `json:"token"`
		User  struct {
			Role      string `json:"role"`
			AccountID string `json:"account_id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.User.Role != string(auth.RoleUser) || out.User.AccountID == "demo-account" {
		t.Fatalf("新用户应为普通角色且拥有独立账户: %+v", out.User)
	}
	if out.Token == "" {
		t.Fatal("注册后应自动登录")
	}
	// 重复邮箱必须冲突，而不是覆盖原账号。
	if w := jsonRequest(r, http.MethodPost, "/api/v1/auth/register",
		map[string]string{"email": "new@example.com", "password": "password123"}, nil); w.Code != http.StatusConflict {
		t.Fatalf("重复邮箱应 409，实际 %d", w.Code)
	}
	if len(e.Users()) != before+1 {
		t.Fatal("重复注册不应改变用户数")
	}
}
