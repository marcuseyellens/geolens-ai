package auth

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRegisterLoginResolve(t *testing.T) {
	e, err := NewEngine(filepath.Join(t.TempDir(), "auth.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	u, err := e.Register("Alice@Example.com", "password123")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if u.Email != "alice@example.com" || u.Role != RoleUser {
		t.Fatalf("邮箱应归一化且默认为普通用户: %+v", u)
	}
	if u.AccountID == "" || u.ID == "" {
		t.Fatal("账号 ID 与用户 ID 必须由服务端生成")
	}

	if _, _, err := e.Login("alice@example.com", "wrong-password"); err != ErrInvalidCredentials {
		t.Fatalf("错误密码应被拒绝: %v", err)
	}
	if _, _, err := e.Login("unknown@example.com", "password123"); err != ErrInvalidCredentials {
		t.Fatalf("未知账号应返回同一错误，避免账号枚举: %v", err)
	}

	token, got, err := e.Login("ALICE@EXAMPLE.COM", "password123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if token == "" || got.AccountID != u.AccountID {
		t.Fatalf("会话或账户绑定错误: token=%q user=%+v", token, got)
	}
	resolved, err := e.Resolve(token)
	if err != nil || resolved.ID != u.ID {
		t.Fatalf("会话解析失败: %v %+v", err, resolved)
	}

	e.Logout(token)
	if _, err := e.Resolve(token); err != ErrInvalidSession {
		t.Fatalf("登出后会话必须立即失效: %v", err)
	}
}

func TestValidationAndDuplicate(t *testing.T) {
	e, err := NewEngine("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Register("not-an-email", "password123"); err != ErrInvalidEmail {
		t.Errorf("非法邮箱应拒绝: %v", err)
	}
	if _, err := e.Register("a@b.co", "short"); err != ErrWeakPassword {
		t.Errorf("弱密码应拒绝: %v", err)
	}
	if _, err := e.Register("a@b.co", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Register("A@B.CO", "password123"); err != ErrEmailTaken {
		t.Errorf("大小写不同的同一邮箱应视为重复: %v", err)
	}
}

func TestSessionExpiry(t *testing.T) {
	current := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	e, err := NewEngine("", func() time.Time { return current })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Seed("admin@example.com", "password123", RoleAdmin, "acct-admin"); err != nil {
		t.Fatal(err)
	}
	token, _, err := e.Login("admin@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	current = current.Add(SessionTTL + time.Second)
	if _, err := e.Resolve(token); err != ErrInvalidSession {
		t.Fatalf("过期会话应失效: %v", err)
	}
}

func TestSeedDoesNotOverwriteAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "auth.json")
	e1, err := NewEngine(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	first, err := e1.Seed("demo@example.com", "password123", RoleAdmin, "acct-demo")
	if err != nil {
		t.Fatal(err)
	}
	// 用不同密码再次 Seed 不得覆盖既有凭据。
	second, err := e1.Seed("demo@example.com", "another-password", RoleAdmin, "acct-other")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.AccountID != "acct-demo" {
		t.Fatalf("Seed 不应覆盖既有用户: %+v", second)
	}
	if _, _, err := e1.Login("demo@example.com", "another-password"); err != ErrInvalidCredentials {
		t.Fatal("Seed 不应改掉原密码")
	}

	e2, err := NewEngine(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e2.Count() != 1 {
		t.Fatalf("用户应持久化，实际 %d", e2.Count())
	}
	if _, _, err := e2.Login("demo@example.com", "password123"); err != nil {
		t.Fatalf("重载后应能用原密码登录: %v", err)
	}
}
