package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/geolens/geolens/internal/auth"
	"github.com/gin-gonic/gin"
)

const (
	ctxAuthUser    = "auth_user"
	ctxAuthAccount = "auth_account"
	ctxAuthRole    = "auth_role"
)

// bearerToken 从 Authorization 头解析 Bearer 令牌；非 Bearer 形式一律视为无效。
func bearerToken(c *gin.Context) string {
	raw := c.GetHeader("Authorization")
	if raw == "" || !strings.HasPrefix(raw, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
}

// RequireAuth 要求有效的登录会话，并把用户身份与账户 ID 写入 context。
// account_id 之后一律由服务端从会话取得，客户端传参不再被信任。
func (s *Server) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.auth == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth runtime not configured"})
			return
		}
		u, err := s.auth.Resolve(bearerToken(c))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Set(ctxAuthUser, u)
		c.Set(ctxAuthAccount, u.AccountID)
		c.Set(ctxAuthRole, string(u.Role))
		c.Next()
	}
}

// RequireAdmin 要求管理员角色；兼容既有的 X-Admin-Token 机器调用。
func (s *Server) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.adminToken != "" && c.GetHeader("X-Admin-Token") == s.adminToken {
			c.Set(ctxAuthRole, string(auth.RoleAdmin))
			c.Set(ctxAuthAccount, "admin")
			c.Next()
			return
		}
		if s.auth == nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth runtime not configured"})
			return
		}
		u, err := s.auth.Resolve(bearerToken(c))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		if u.Role != auth.RoleAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": auth.ErrForbidden.Error()})
			return
		}
		c.Set(ctxAuthUser, u)
		c.Set(ctxAuthAccount, u.AccountID)
		c.Set(ctxAuthRole, string(u.Role))
		c.Next()
	}
}

// currentAccount 返回会话绑定的账户 ID。只应挂在 RequireAuth 之后。
func currentAccount(c *gin.Context) string {
	if v, ok := c.Get(ctxAuthAccount); ok {
		if id, ok := v.(string); ok && id != "" {
			return id
		}
	}
	return "demo-account"
}

// currentUser 返回会话用户，未认证时返回 nil。
func currentUser(c *gin.Context) *auth.PublicUser {
	v, ok := c.Get(ctxAuthUser)
	if !ok {
		return nil
	}
	u, ok := v.(auth.PublicUser)
	if !ok {
		return nil
	}
	return &u
}

func (s *Server) authUnavailable(c *gin.Context) bool {
	if s.auth == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth runtime not configured"})
		return true
	}
	return false
}

// POST /api/v1/auth/login —— 公开端点。
func (s *Server) authLogin(c *gin.Context) {
	if s.authUnavailable(c) {
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
		return
	}
	token, u, err := s.auth.Login(body.Email, body.Password)
	if err != nil {
		// 统一错误，避免账号枚举。
		c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrInvalidCredentials.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token": token, "user": u,
		"expires_at": time.Now().Add(auth.SessionTTL).UTC().Format(time.RFC3339),
	})
}

// POST /api/v1/auth/register —— 公开端点，注册后自动登录。
func (s *Server) authRegister(c *gin.Context) {
	if s.authUnavailable(c) {
		return
	}
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password required"})
		return
	}
	u, err := s.auth.Register(body.Email, body.Password)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, auth.ErrEmailTaken) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	token, _, err := s.auth.Login(body.Email, body.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"token": token, "user": u})
}

// POST /api/v1/auth/logout —— 需要会话。
func (s *Server) authLogout(c *gin.Context) {
	s.auth.Logout(bearerToken(c))
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// GET /api/v1/auth/me —— 需要会话，前端启动时用于恢复登录态。
func (s *Server) authMe(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"user": currentUser(c)})
}
