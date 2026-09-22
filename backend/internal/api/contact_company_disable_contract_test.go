// 契约测（ADR-0062 决策 9 / CONTEXT.md「授权有效态」）：授权有效态**双向对称**。
//
// 缺陷形状：判据原本只做了一半——存活判据只有学员侧（studentAccountAlive），学员看企业那条
// 走零查询的行级谓词（GrantsPlaintext），于是管理员禁用一家企业（ToggleRecruiterStatus，
// 正是违规处置动作、且已同时吊销其全部会话）之后，此前已获授权的学员
// 在 GET /api/resume/contact-requests 里仍看得到该企业的电话/邮箱/微信明文。
//
// seam = 后端 HTTP 契约层：三条锁一律只断言**响应的外部形状**（不查库），
// 夹具照 contact_see_contract_test.go / contact_contract_test.go / recruiter_disable_contract_test.go。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// 企业明文三段的夹具值：出现在不该出现的响应里即判红。
const (
	disabledCompanyPhone  = "13800007777"
	disabledCompanyEmail  = "disable@example.com"
	disabledCompanyWechat = "company_wx_disable"
)

// 学员明文两段的夹具值（招聘方取明文那半边用）。
const (
	disabledStudentPhone  = "13911112222"
	disabledStudentWechat = "stu_wx_disable"
)

// contactDisableEnv 一条真实链路：路由 + 学员/企业/管理三枚 token + 一条已批准的授权。
type contactDisableEnv struct {
	router       *gin.Engine
	db           *gorm.DB
	studentID    int
	recruiterID  int
	studentToken string
	recruiterTok string
	adminToken   string
}

func newContactDisableEnv(t *testing.T) *contactDisableEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:          "contact-disable-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuDisable", pwd)
	card := model.JobCard{
		UserID: stu.ID, RealName: "李四", ContactPhone: disabledStudentPhone, Wechat: disabledStudentWechat,
		Region: "江苏省/苏州市", ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)),
		Visibility: "open", ResumeFileURL: "/static/uploads/resumes/disable.pdf",
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("建简历卡失败: %v", err)
	}

	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminDisable", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{
		Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")

	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", map[string]any{
		"username": "recruitDisable", "password": "recruit123", "company_name": "停用测试企业",
		"credit_code": "91110000MAdisable", "business_scope": "叉车维修", "contact_name": "联系人丁",
		"contact_phone": disabledCompanyPhone, "contact_email": disabledCompanyEmail,
		"wechat": disabledCompanyWechat,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	var created recruiterCreateResp
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析创建响应失败: %v", err)
	}

	login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login",
		map[string]any{"username": "recruitDisable", "password": "recruit123"})
	if login.Code != http.StatusOK {
		t.Fatalf("招聘者登录失败 %d %s", login.Code, login.Body.String())
	}
	var loginBody loginResp
	if err := json.Unmarshal(login.Body.Bytes(), &loginBody); err != nil {
		t.Fatalf("解析登录响应失败: %v", err)
	}

	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	studentToken, _ := studentSess.Issue(stu.ID, stu.Account, "hrwai_user")

	// 企业发起 → 学员同意：一条 approved 授权
	rec = doWithToken(t, r, loginBody.Data.Token, http.MethodPost, "/api/recruit/contact-requests",
		map[string]any{"student_user_id": stu.ID, "message": "想聊聊岗位"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发起交换申请失败 %d %s", rec.Code, rec.Body.String())
	}
	var reqEnv struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &reqEnv); err != nil {
		t.Fatalf("解析申请响应失败: %v", err)
	}
	rec = doWithToken(t, r, studentToken, http.MethodPost,
		"/api/resume/contact-requests/"+strconv.FormatInt(reqEnv.Data.ID, 10)+"/approve", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员同意失败 %d %s", rec.Code, rec.Body.String())
	}

	return &contactDisableEnv{
		router: r, db: db,
		studentID: stu.ID, recruiterID: created.Data.ID,
		studentToken: studentToken, recruiterTok: loginBody.Data.Token,
		adminToken: adminToken,
	}
}

// contactListRow 学员侧交换申请列表的一行（只取本票关心的槽；company_disabled 用指针，
// 以区分「缺席」与「显式假」——契约是 omitempty 的可选槽）。
type contactListRow struct {
	Status          string `json:"status"`
	CompanyName     string `json:"company_name"`
	ContactName     string `json:"contact_name"`
	ContactPhone    string `json:"contact_phone"`
	ContactEmail    string `json:"contact_email"`
	Wechat          string `json:"wechat"`
	CompanyDisabled *bool  `json:"company_disabled"`
}

// studentContactRows 取 GET /api/resume/contact-requests：同时回原始 body
// （明文按 body 判、字段按解析结果判，两件事都只看响应外部形状）。
func studentContactRows(t *testing.T, e *contactDisableEnv) ([]contactListRow, string) {
	t.Helper()
	rec := doWithToken(t, e.router, e.studentToken, http.MethodGet, "/api/resume/contact-requests", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员侧列表应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Items []contactListRow `json:"items"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析学员侧列表失败: %v body=%s", err, rec.Body.String())
	}
	return env.Data.Items, rec.Body.String()
}

// toggleCompany 走真实处置动作：PUT /api/admin/recruiters/{id}/status（禁用与解禁同一个开关）。
func toggleCompany(t *testing.T, e *contactDisableEnv) int16 {
	t.Helper()
	rec := doWithToken(t, e.router, e.adminToken, http.MethodPut,
		"/api/admin/recruiters/"+strconv.Itoa(e.recruiterID)+"/status", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("切换企业启停应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data struct {
			Status int16 `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析切换响应失败: %v", err)
	}
	return env.Data.Status
}

// assertNoCompanyPlaintext 明文三段一律不得出现在响应里。
func assertNoCompanyPlaintext(t *testing.T, label, body string) {
	t.Helper()
	for _, needle := range []string{disabledCompanyPhone, disabledCompanyEmail, disabledCompanyWechat} {
		if strings.Contains(body, needle) {
			t.Fatalf("%s 不应透出企业明文 %q, body=%s", label, needle, body)
		}
	}
}

// 锁① + 锁②：企业被禁用后学员侧不再透出明文；解禁后当场恢复。
//
// 两半合起来才叫「判据看的是当前状态」：只写①，实现可以靠「禁用时顺手把授权行也吊销」蒙过去；
// 只写②，则可能压根没判禁用。②的另一条断言（status 仍 approved、条目仍在）钉住
// 「授权存在 ≠ 授权可用」——处置不改写授权事实，只摘掉可用性。
func TestContactCompanyDisableContract(t *testing.T) {
	e := newContactDisableEnv(t)

	// 基线：未禁用时学员侧确实看得到明文。不先钉这一条，下面的「不含」就是恒真的空话。
	rows, body := studentContactRows(t, e)
	if len(rows) != 1 {
		t.Fatalf("夹具应有 1 条申请, 实际 %d body=%s", len(rows), body)
	}
	for _, needle := range []string{disabledCompanyPhone, disabledCompanyEmail, disabledCompanyWechat} {
		if !strings.Contains(body, needle) {
			t.Fatalf("启用中的企业应向已授权学员透出明文 %q, body=%s", needle, body)
		}
	}
	if rows[0].Status != string(service.ContactGrantApproved) {
		t.Fatalf("夹具应为 approved, 实际 %s", rows[0].Status)
	}
	if rows[0].CompanyDisabled != nil {
		t.Fatalf("企业可用时不应出现 company_disabled, 实际 %v", *rows[0].CompanyDisabled)
	}

	// —— 处置：管理员禁用该企业（「处置不延伸到联系面即留空洞」正是本票的靶子）——
	if got := toggleCompany(t, e); got != 0 {
		t.Fatalf("禁用后管理面应回 status=0, 实际 %d", got)
	}

	rows, body = studentContactRows(t, e)
	assertNoCompanyPlaintext(t, "企业被禁用后的学员侧交换申请列表", body)
	if len(rows) != 1 {
		t.Fatalf("禁用企业不该让申请条目消失（那是授权事实），实际 %d body=%s", len(rows), body)
	}
	if rows[0].Status != string(service.ContactGrantApproved) {
		t.Fatalf("禁用企业不改写授权事实，status 仍应为 approved, 实际 %s", rows[0].Status)
	}
	if rows[0].CompanyName != "停用测试企业" {
		t.Fatalf("企业名不是 L3 明文，禁用后仍应回填, 实际 %q", rows[0].CompanyName)
	}
	if rows[0].CompanyDisabled == nil || !*rows[0].CompanyDisabled {
		t.Fatalf("明文位置要有具名说明「企业已停用」而不是静默空白，实际 company_disabled=%v", rows[0].CompanyDisabled)
	}

	// 对称的另一半：企业自己（手上那枚未过期 access 仍过 JWTAuth）也不再能取学员明文。
	rec := doWithToken(t, e.router, e.recruiterTok, http.MethodGet,
		"/api/recruit/resumes/"+strconv.Itoa(e.studentID)+"/contact", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("被禁用企业读学员明文应 403, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), disabledStudentPhone) ||
		strings.Contains(rec.Body.String(), disabledStudentWechat) {
		t.Fatalf("被禁用企业不应再拿到学员明文, body=%s", rec.Body.String())
	}

	// —— 锁②：解禁后明文当场恢复（同一枚 token、不重新登录）⇒ 判据读当前状态而非历史快照 ——
	if got := toggleCompany(t, e); got != 1 {
		t.Fatalf("解禁后管理面应回 status=1, 实际 %d", got)
	}
	rows, body = studentContactRows(t, e)
	for _, needle := range []string{disabledCompanyPhone, disabledCompanyEmail, disabledCompanyWechat} {
		if !strings.Contains(body, needle) {
			t.Fatalf("解禁后应恢复透出 %q, body=%s", needle, body)
		}
	}
	if len(rows) != 1 || rows[0].CompanyDisabled != nil {
		t.Fatalf("解禁后不应再报 company_disabled, 实际 %v", rows)
	}
	rec = doWithToken(t, e.router, e.recruiterTok, http.MethodGet,
		"/api/recruit/resumes/"+strconv.Itoa(e.studentID)+"/contact", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("解禁后企业取学员明文应恢复 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), disabledStudentPhone) {
		t.Fatalf("解禁后学员明文应恢复, body=%s", rec.Body.String())
	}
}

// 锁③：企业侧取学员明文时「学员已注销即降级」不回归（ADR-0053 §3 原有那半边）。
//
// 拆成两例，因为它们钉的是两条不同的判据：
//  1. 授权行仍在、只有账号行没了 ⇒ 走 studentAccountAlive 这一维，且**原因必须仍具名为「学员已注销」**
//     （对称化时最容易把它并成笼统的「无授权」）；整链注销走真实动作那半边已由
//     contact_contract_test.go 第 11 步锁住（DeleteAccount 会连带删掉授权行）。
func TestContactStudentGoneStillDegradesPlaintextContract(t *testing.T) {
	e := newContactDisableEnv(t)

	rec := doWithToken(t, e.router, e.recruiterTok, http.MethodGet,
		"/api/recruit/resumes/"+strconv.Itoa(e.studentID)+"/contact", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("注销前取明文应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), disabledStudentPhone) {
		t.Fatalf("注销前应透出学员明文, body=%s", rec.Body.String())
	}

	// 只摘掉学员账号行（保留那条 approved）⇒ 唯一还能挡住明文的就只剩存活判据这一维。
	if err := e.db.Where("id = ?", e.studentID).Delete(&model.HrwaiUser{}).Error; err != nil {
		t.Fatalf("摘掉学员账号行失败: %v", err)
	}

	rec = doWithToken(t, e.router, e.recruiterTok, http.MethodGet,
		"/api/recruit/resumes/"+strconv.Itoa(e.studentID)+"/contact", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("学员已注销时取明文应 403, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), disabledStudentPhone) ||
		strings.Contains(rec.Body.String(), disabledStudentWechat) {
		t.Fatalf("学员注销后不应再透出学员明文, body=%s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "注销") {
		t.Fatalf("降级原因要具名为「学员已注销」，不得被对称化并成笼统的无授权, body=%s", rec.Body.String())
	}
}
