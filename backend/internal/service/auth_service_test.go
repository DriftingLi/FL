// Package service 认证服务测试，使用内存 sqlite 数据库，无需外部依赖。
package service

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

const testJWTSecret = "test-secret-key-for-unit-test"

func newAuthSvc(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewAuthService(db, testJWTSecret, time.Hour, "admin123", "tutor123", "student123"), db
}

// --- HashPassword / VerifyPassword ---

func TestHashPassword_Success(t *testing.T) {
	hash, err := HashPassword("mypassword")
	if err != nil {
		t.Fatalf("HashPassword 失败: %v", err)
	}
	if hash == "" || hash == "mypassword" {
		t.Fatalf("哈希值不合法: %q", hash)
	}
}

func TestHashPassword_DifferentSalt(t *testing.T) {
	h1, _ := HashPassword("same")
	h2, _ := HashPassword("same")
	if h1 == h2 {
		t.Fatal("相同密码两次哈希应不同（随机盐）")
	}
}

func TestVerifyPassword_Correct(t *testing.T) {
	hash, _ := HashPassword("correct-pwd")
	if !VerifyPassword("correct-pwd", hash) {
		t.Fatal("正确密码应校验通过")
	}
}

func TestVerifyPassword_Wrong(t *testing.T) {
	hash, _ := HashPassword("correct-pwd")
	if VerifyPassword("wrong-pwd", hash) {
		t.Fatal("错误密码应校验失败")
	}
}

func TestVerifyPassword_EmptyHash(t *testing.T) {
	if VerifyPassword("any", "") {
		t.Fatal("空哈希应校验失败")
	}
}

// --- GenerateToken ---

func TestAuthService_GenerateToken(t *testing.T) {
	svc, _ := newAuthSvc(t)
	token, err := svc.GenerateToken(42, "alice", "student")
	if err != nil {
		t.Fatalf("GenerateToken 失败: %v", err)
	}
	if token == "" {
		t.Fatal("token 不应为空")
	}
	// 解析验证 claims
	claims := &middleware.Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(tk *jwt.Token) (interface{}, error) {
		return []byte(testJWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		t.Fatalf("token 解析失败: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "alice" || claims.Role != "student" {
		t.Fatalf("claims 不匹配: %+v", claims)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt 不应为空")
	}
}

// --- StudentLogin ---

func TestStudentLogin_Success(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("pwd123")
	testutil.SeedStudent(t, tdb, "student1", hash)

	result, err := svc.StudentLogin("student1", "pwd123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if result.Username != "student1" || result.Role != "student" {
		t.Fatalf("登录结果不匹配: %+v", result)
	}
	if result.Token == "" {
		t.Fatal("token 不应为空")
	}
}

func TestStudentLogin_WrongPassword(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("pwd123")
	testutil.SeedStudent(t, tdb, "student1", hash)

	_, err := svc.StudentLogin("student1", "wrong")
	if err == nil || err.Error() != "用户名或密码错误" {
		t.Fatalf("应返回密码错误, got %v", err)
	}
}

func TestStudentLogin_NotFound(t *testing.T) {
	svc, _ := newAuthSvc(t)
	_, err := svc.StudentLogin("nobody", "pwd")
	if err == nil || err.Error() != "用户名或密码错误" {
		t.Fatalf("应返回用户名或密码错误, got %v", err)
	}
}

func TestStudentLogin_Disabled(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("pwd123")
	s := testutil.SeedStudent(t, tdb, "disabled", hash)
	s.Status = 0 // 禁用
	tdb.Save(s)

	_, err := svc.StudentLogin("disabled", "pwd123")
	if err == nil || err.Error() != "账号已被禁用，请联系管理员" {
		t.Fatalf("应返回禁用错误, got %v", err)
	}
}

// --- StudentRegister ---

func TestStudentRegister_Success(t *testing.T) {
	svc, _ := newAuthSvc(t)
	// 新签名：username 由手机号自动生成
	result, err := svc.StudentRegister("13800138000", "pwd", "新生", "", "")
	if err != nil {
		t.Fatalf("注册失败: %v", err)
	}
	if result["username"] != "13800138000" || result["name"] != "新生" {
		t.Fatalf("注册结果不匹配: %+v", result)
	}
	// 验证可用手机号登录
	login, err := svc.StudentLogin("13800138000", "pwd")
	if err != nil {
		t.Fatalf("注册后应可登录: %v", err)
	}
	if login.UserID == 0 {
		t.Fatal("UserID 不应为 0")
	}
}

func TestStudentRegister_Duplicate(t *testing.T) {
	svc, _ := newAuthSvc(t)
	_, _ = svc.StudentRegister("13800138000", "pwd", "dup1", "", "")
	_, err := svc.StudentRegister("13800138000", "pwd", "dup2", "", "")
	if err == nil || err.Error() != "手机号已被注册" {
		t.Fatalf("应返回手机号已被注册, got %v", err)
	}
}

// --- AdminLogin ---

func TestAdminLogin_Success(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("adminpwd")
	testutil.SeedAdmin(t, tdb, "admin1", hash)

	result, err := svc.AdminLogin("admin1", "adminpwd")
	if err != nil {
		t.Fatalf("管理员登录失败: %v", err)
	}
	if result.Role != "admin" {
		t.Fatalf("角色应为 admin, got %s", result.Role)
	}
}

func TestAdminLogin_WrongPassword(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("adminpwd")
	testutil.SeedAdmin(t, tdb, "admin1", hash)

	_, err := svc.AdminLogin("admin1", "wrong")
	if err == nil || err.Error() != "管理员账号或密码错误" {
		t.Fatalf("应返回管理员账号或密码错误, got %v", err)
	}
}

func TestAdminLogin_NotFound(t *testing.T) {
	svc, _ := newAuthSvc(t)
	_, err := svc.AdminLogin("ghost", "pwd")
	if err == nil || err.Error() != "管理员账号或密码错误" {
		t.Fatalf("应返回管理员账号或密码错误, got %v", err)
	}
}

// --- TutorLogin ---

func TestTutorLogin_Success(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("tutorpwd")
	testutil.SeedTutor(t, tdb, "tutor1", hash)

	result, err := svc.TutorLogin("tutor1", "tutorpwd")
	if err != nil {
		t.Fatalf("导师登录失败: %v", err)
	}
	if result.Role != "tutor" {
		t.Fatalf("角色应为 tutor, got %s", result.Role)
	}
}

func TestTutorLogin_WrongPassword(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("tutorpwd")
	testutil.SeedTutor(t, tdb, "tutor1", hash)

	_, err := svc.TutorLogin("tutor1", "wrong")
	if err == nil || err.Error() != "导师账号或密码错误" {
		t.Fatalf("应返回导师账号或密码错误, got %v", err)
	}
}

func TestTutorLogin_Disabled(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("tutorpwd")
	tu := testutil.SeedTutor(t, tdb, "disabled", hash)
	tu.Status = 0
	tdb.Save(tu)

	_, err := svc.TutorLogin("disabled", "tutorpwd")
	if err == nil || err.Error() != "账号已被禁用，请联系管理员" {
		t.Fatalf("应返回禁用错误, got %v", err)
	}
}

func TestTutorRegister_Success(t *testing.T) {
	svc, _ := newAuthSvc(t)
	result, err := svc.TutorRegister("newtutor", "pwd", "导师")
	if err != nil {
		t.Fatalf("导师注册失败: %v", err)
	}
	if result["username"] != "newtutor" {
		t.Fatalf("注册结果不匹配: %+v", result)
	}
}

func TestTutorRegister_Duplicate(t *testing.T) {
	svc, _ := newAuthSvc(t)
	_, _ = svc.TutorRegister("dup", "pwd", "t1")
	_, err := svc.TutorRegister("dup", "pwd", "t2")
	if err == nil || err.Error() != "用户名已被注册" {
		t.Fatalf("应返回用户名已被注册, got %v", err)
	}
}

// --- EnsureDefaultUsers ---

func TestEnsureDefaultUsers_CreatesDefault(t *testing.T) {
	svc, _ := newAuthSvc(t)
	if err := svc.EnsureDefaultUsers(); err != nil {
		t.Fatalf("EnsureDefaultUsers 失败: %v", err)
	}
	// 验证默认导师可登录
	result, err := svc.TutorLogin("tutor", "tutor123")
	if err != nil {
		t.Fatalf("默认导师登录失败: %v", err)
	}
	if result.Name != "导师" {
		t.Fatalf("默认导师名称应为 导师, got %s", result.Name)
	}
}

func TestEnsureDefaultUsers_Idempotent(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	if err := svc.EnsureDefaultUsers(); err != nil {
		t.Fatalf("第一次调用失败: %v", err)
	}
	var count int64
	tdb.Model(&model.Student{}).Count(&count) // 仅为引用 DB
	if err := svc.EnsureDefaultUsers(); err != nil {
		t.Fatalf("第二次调用（幂等）失败: %v", err)
	}
	// 验证仍只有一个 tutor 账号
	var tutorCount int64
	tdb.Table("tutor").Where("username = ?", "tutor").Count(&tutorCount)
	if tutorCount != 1 {
		t.Fatalf("幂等调用后应仍只有 1 个 tutor, got %d", tutorCount)
	}
}
