// Package auth 提供 GeoLens 的最小用户/会话体系。
//
// 密码用 bcrypt 加盐哈希，会话令牌为 crypto/rand 生成的随机串，仅保存在内存
// （进程重启后需重新登录）。用户持久化到 JSON 文件，会话不持久化，避免令牌
// 落盘带来的泄露面。生产环境应替换为独立的身份服务 + 短期 JWT/刷新令牌 +
// 登录限流与邮箱验证，本实现只保证本地可运行且鉴权边界真实生效。
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Role 决定能否调用管理端点。
type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

// SessionTTL 会话有效期。
const SessionTTL = 24 * time.Hour

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailTaken         = errors.New("email already registered")
	ErrWeakPassword       = errors.New("password must be at least 8 characters")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidSession     = errors.New("invalid or expired session")
	ErrForbidden          = errors.New("admin role required")
)

// User 是持久化用户。PasswordHash 永不出现在 API 响应里（见 Public）。
type User struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	Role         Role   `json:"role"`
	AccountID    string `json:"account_id"`
	PasswordHash string `json:"password_hash"`
	CreatedAt    string `json:"created_at"`
}

// PublicUser 是可安全返回给前端的用户视图。
type PublicUser struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      Role   `json:"role"`
	AccountID string `json:"account_id"`
}

// Public 去掉密码哈希。
func (u *User) Public() PublicUser {
	return PublicUser{ID: u.ID, Email: u.Email, Role: u.Role, AccountID: u.AccountID}
}

type session struct {
	userID    string
	expiresAt time.Time
}

type fileData struct {
	Users []*User `json:"users"`
}

// Engine 线程安全的用户注册与会话管理。
type Engine struct {
	mu       sync.RWMutex
	filePath string
	users    map[string]*User // 小写 email -> user
	sessions map[string]session
	now      func() time.Time
}

// NewEngine 创建引擎。filePath 为空时纯内存（测试用）。
func NewEngine(filePath string, clock func() time.Time) (*Engine, error) {
	if clock == nil {
		clock = time.Now
	}
	e := &Engine{
		filePath: filePath,
		users:    map[string]*User{},
		sessions: map[string]session{},
		now:      clock,
	}
	if filePath != "" {
		if err := e.load(); err != nil {
			return nil, err
		}
	}
	return e, nil
}

func (e *Engine) load() error {
	b, err := os.ReadFile(e.filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var data fileData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	for _, u := range data.Users {
		if u == nil || u.Email == "" {
			continue
		}
		e.users[strings.ToLower(u.Email)] = u
	}
	return nil
}

func (e *Engine) saveLocked() error {
	if e.filePath == "" {
		return nil
	}
	data := fileData{Users: make([]*User, 0, len(e.users))}
	for _, u := range e.users {
		data.Users = append(data.Users, u)
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(e.filePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(e.filePath, b, 0600)
}

func normalizeEmail(email string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(email))
	if !strings.Contains(v, "@") || !strings.Contains(strings.SplitN(v, "@", 2)[1], ".") {
		return "", ErrInvalidEmail
	}
	return v, nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return ErrWeakPassword
	}
	return nil
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Seed 在用户不存在时创建初始账号；已存在则原样返回，不覆盖既有密码。
func (e *Engine) Seed(email, password string, role Role, accountID string) (PublicUser, error) {
	norm, err := normalizeEmail(email)
	if err != nil {
		return PublicUser{}, err
	}
	if err := validatePassword(password); err != nil {
		return PublicUser{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if existing, ok := e.users[norm]; ok {
		return existing.Public(), nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return PublicUser{}, err
	}
	if accountID == "" {
		accountID = "acct_" + uuid.NewString()
	}
	u := &User{
		ID: uuid.NewString(), Email: norm, Role: role, AccountID: accountID,
		PasswordHash: string(hash), CreatedAt: e.now().UTC().Format(time.RFC3339),
	}
	e.users[norm] = u
	if err := e.saveLocked(); err != nil {
		delete(e.users, norm)
		return PublicUser{}, err
	}
	return u.Public(), nil
}

// Register 创建普通用户，账号 ID 由服务端生成，客户端不可指定。
func (e *Engine) Register(email, password string) (PublicUser, error) {
	norm, err := normalizeEmail(email)
	if err != nil {
		return PublicUser{}, err
	}
	if err := validatePassword(password); err != nil {
		return PublicUser{}, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.users[norm]; exists {
		return PublicUser{}, ErrEmailTaken
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return PublicUser{}, err
	}
	u := &User{
		ID: uuid.NewString(), Email: norm, Role: RoleUser,
		AccountID: "acct_" + uuid.NewString(), PasswordHash: string(hash),
		CreatedAt: e.now().UTC().Format(time.RFC3339),
	}
	e.users[norm] = u
	if err := e.saveLocked(); err != nil {
		delete(e.users, norm)
		return PublicUser{}, err
	}
	return u.Public(), nil
}

// Login 校验密码并签发会话令牌。错误信息统一，避免账号枚举。
func (e *Engine) Login(email, password string) (string, PublicUser, error) {
	norm := strings.ToLower(strings.TrimSpace(email))
	e.mu.Lock()
	defer e.mu.Unlock()
	u, ok := e.users[norm]
	if !ok || bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)) != nil {
		return "", PublicUser{}, ErrInvalidCredentials
	}
	token, err := randomToken()
	if err != nil {
		return "", PublicUser{}, err
	}
	e.sessions[token] = session{userID: u.ID, expiresAt: e.now().Add(SessionTTL)}
	return token, u.Public(), nil
}

// Resolve 校验会话并返回用户；过期会话会被清除。
func (e *Engine) Resolve(token string) (PublicUser, error) {
	if token == "" {
		return PublicUser{}, ErrInvalidSession
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	s, ok := e.sessions[token]
	if !ok {
		return PublicUser{}, ErrInvalidSession
	}
	if e.now().After(s.expiresAt) {
		delete(e.sessions, token)
		return PublicUser{}, ErrInvalidSession
	}
	for _, u := range e.users {
		if u.ID == s.userID {
			return u.Public(), nil
		}
	}
	delete(e.sessions, token)
	return PublicUser{}, ErrInvalidSession
}

// Logout 立即撤销会话。
func (e *Engine) Logout(token string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.sessions, token)
}

// Users 返回所有用户的公开视图（管理用途）。
func (e *Engine) Users() []PublicUser {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]PublicUser, 0, len(e.users))
	for _, u := range e.users {
		out = append(out, u.Public())
	}
	return out
}

// Count 返回用户数（自检用）。
func (e *Engine) Count() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.users)
}
