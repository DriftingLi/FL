// Package service 认证服务测试，使用内存 sqlite 数据库，无需外部依赖。
package service

import (
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

const testJWTSecret = "test-secret-key-for-unit-test"

func newAuthSvc(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	return NewAuthService(db, security.NewSession(testJWTSecret, time.Hour, security.CookieConfig{}), "admin123", "tutor123", "student123", zap.NewNop()), db
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

// --- HrwaiLogin ---

func TestHrwaiLogin_Success(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("pwd123")
	testutil.SeedStudent(t, tdb, "student1", hash)

	result, err := svc.HrwaiLogin("acct_student1", "pwd123")
	if err != nil {
		t.Fatalf("登录失败: %v", err)
	}
	if result.Account != "acct_student1" || result.Username != "student1" || result.Role != HrwaiRole {
		t.Fatalf("登录结果不匹配: %+v", result)
	}
	if result.Token == "" {
		t.Fatal("token 不应为空")
	}
}

func TestHrwaiLogin_WrongPassword(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("pwd123")
	testutil.SeedStudent(t, tdb, "student1", hash)

	_, err := svc.HrwaiLogin("acct_student1", "wrong")
	if err == nil || err.Error() != "账号或密码错误" {
		t.Fatalf("应返回密码错误, got %v", err)
	}
}

func TestHrwaiLogin_NotFound(t *testing.T) {
	svc, _ := newAuthSvc(t)
	_, err := svc.HrwaiLogin("nobody", "pwd")
	if err == nil || err.Error() != "账号或密码错误" {
		t.Fatalf("应返回用户名或密码错误, got %v", err)
	}
}

func TestHrwaiLogin_Disabled(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("pwd123")
	s := testutil.SeedStudent(t, tdb, "disabled", hash)
	s.Status = 0 // 禁用
	tdb.Save(s)

	_, err := svc.HrwaiLogin("acct_disabled", "pwd123")
	if err == nil || err.Error() != "账号已被禁用，请联系管理员" {
		t.Fatalf("应返回禁用错误, got %v", err)
	}
}

// --- 密码修改 ---

func TestUpdatePassword(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("old123")
	s := testutil.SeedStudent(t, tdb, "pwduser", hash)
	if err := svc.UpdatePassword(s.ID, "new123"); err != nil {
		t.Fatalf("修改密码失败: %v", err)
	}
	if _, err := svc.HrwaiLogin("acct_pwduser", "new123"); err != nil {
		t.Fatalf("新密码应可登录: %v", err)
	}
	if err := svc.UpdatePassword(s.ID, "123"); err == nil {
		t.Error("过短密码应报错")
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
	if result.Username != "tutor" {
		t.Fatalf("默认导师账号应为 tutor, got %s", result.Username)
	}
}

func TestEnsureDefaultUsers_Idempotent(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	if err := svc.EnsureDefaultUsers(); err != nil {
		t.Fatalf("第一次调用失败: %v", err)
	}
	var count int64
	tdb.Model(&model.HrwaiUser{}).Count(&count) // 仅为引用 DB
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

// --- GetProfile (/auth/me 资料组装) ---

func newGetProfileSvc(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	svc, tdb := newAuthSvc(t)
	svc.SetProfileReviewService(NewProfileReviewService(tdb, nil, nil, zap.NewNop()))
	return svc, tdb
}

func TestGetProfile_HrwaiUser(t *testing.T) {
	svc, tdb := newGetProfileSvc(t)
	hash, _ := HashPassword("pwd123")
	u := testutil.SeedStudent(t, tdb, "alice", hash)
	u.Username = "小爱"
	u.AvatarURL = "https://example.com/avatar.png"
	u.Email = "alice@example.com"
	u.Company = "和润"
	tdb.Save(u)

	dto := svc.GetProfile(u.ID, HrwaiRole, u.Account)
	if dto.UserID != u.ID || dto.Account != "acct_alice" || dto.Role != HrwaiRole {
		t.Fatalf("基础字段异常: %+v", dto)
	}
	if dto.Username == nil || *dto.Username != "小爱" {
		t.Fatalf("昵称字段异常: %+v", dto.Username)
	}
	if dto.UID == nil || *dto.UID != FormatUID(u.UID) {
		t.Fatalf("uid 字段异常: %+v", dto.UID)
	}
	if dto.AvatarURL == nil || *dto.AvatarURL != "https://example.com/avatar.png" {
		t.Fatalf("头像字段异常: %+v", dto.AvatarURL)
	}
	if dto.Phone == nil || *dto.Phone != "test_alice" {
		t.Fatalf("手机号字段异常: %+v", dto.Phone)
	}
	if dto.Email == nil || *dto.Email != "alice@example.com" {
		t.Fatalf("邮箱字段异常: %+v", dto.Email)
	}
	if dto.Company == nil || *dto.Company != "和润" {
		t.Fatalf("公司字段异常: %+v", dto.Company)
	}
	if dto.HasPassword == nil || !*dto.HasPassword {
		t.Fatalf("已设置密码时应 has_password=true: %+v", dto.HasPassword)
	}
	if dto.PendingProfileChange != nil && *dto.PendingProfileChange != nil {
		t.Fatalf("无待审资料时应为 nil: %+v", dto.PendingProfileChange)
	}
}

func TestGetProfile_HasPasswordFalse(t *testing.T) {
	svc, tdb := newGetProfileSvc(t)
	u := testutil.SeedStudent(t, tdb, "nopwd", "") // 未设置密码

	dto := svc.GetProfile(u.ID, HrwaiRole, u.Account)
	if dto.HasPassword == nil || *dto.HasPassword {
		t.Fatalf("未设置密码时应 has_password=false: %+v", dto.HasPassword)
	}
}

func TestGetProfile_PendingReview(t *testing.T) {
	svc, tdb := newGetProfileSvc(t)
	hash, _ := HashPassword("pwd123")
	u := testutil.SeedStudent(t, tdb, "pending", hash)
	req, err := svc.reviewSvc.CreateRequest(u.ID, ProfileFieldNickname, "新昵称")
	if err != nil {
		t.Fatalf("提交待审请求失败: %v", err)
	}

	dto := svc.GetProfile(u.ID, HrwaiRole, u.Account)
	if dto.PendingProfileChange == nil || *dto.PendingProfileChange == nil {
		t.Fatalf("应有待审资料对象: %v", dto.PendingProfileChange)
	}
	pending := *dto.PendingProfileChange
	if pending.ID != req.ID || pending.Status != ProfileStatusPending {
		t.Fatalf("待审资料异常: %+v", pending)
	}
}

func TestGetProfile_Tutor(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("tutorpwd")
	tu := testutil.SeedTutor(t, tdb, "tutor1", hash)

	dto := svc.GetProfile(tu.TutorID, "tutor", tu.Username)
	if dto.Name == nil || *dto.Name != "tutor1" {
		t.Fatalf("导师姓名异常: %+v", dto)
	}
}

func TestGetProfile_Admin(t *testing.T) {
	svc, tdb := newAuthSvc(t)
	hash, _ := HashPassword("adminpwd")
	a := testutil.SeedAdmin(t, tdb, "admin1", hash)

	dto := svc.GetProfile(a.AdminID, "admin", a.Username)
	if dto.Name == nil || *dto.Name != "admin1" {
		t.Fatalf("管理员姓名异常: %+v", dto)
	}
}

func TestGetProfile_UserNotFound(t *testing.T) {
	svc, _ := newGetProfileSvc(t)
	dto := svc.GetProfile(999, HrwaiRole, "ghost")
	if dto.Name != nil {
		t.Fatalf("用户不存在时不应有 name 字段: %+v", dto.Name)
	}
	if dto.HasPassword != nil {
		t.Fatal("用户不存在时不应有 has_password 字段")
	}
}
