// 契约测试 #375：联系方式交换闭环。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

func TestContactContract_FullFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	deps := newContractDeps(t, db, cfg)
	r := NewRouter(deps)

	// 学员与简历
	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuContact", pwd)
	// 给简历
	card := model.JobCard{UserID: stu.ID, RealName: "张三丰", ContactPhone: "13800009999", Wechat: "zhang_wx", Region: "江苏苏州精确", ResumeFileURL: "/static/uploads/resumes/a.pdf", Visibility: "open", ExpectedRegions: model.JSONB([]byte(`["江苏苏州"]`))}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}

	// 管理员建企业招聘者
	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminContact", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	createRecruiter := func(username string) (int, string) {
		body := map[string]any{
			"username": username, "password": "recruit123", "company_name": "测试企业-" + username, "credit_code": "91110000MA" + username, "business_scope": "叉车维修", "contact_name": "联系人-" + username, "contact_phone": "13800001111", "contact_email": username + "@example.com",
		}
		rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
		if rec.Code != http.StatusCreated {
			t.Fatalf("建招聘者 %s 失败 %d %s", username, rec.Code, rec.Body.String())
		}
		var created recruiterCreateResp
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("parse created: %v", err)
		}
		rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": username, "password": "recruit123"})
		if rec2.Code != http.StatusOK {
			t.Fatalf("login %s fail %d %s", username, rec2.Code, rec2.Body.String())
		}
		var resp loginResp
		if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
			t.Fatalf("parse login: %v", err)
		}
		return created.Data.ID, resp.Data.Token
	}
	recruiterAID, recruiterAToken := createRecruiter("recruitContactA")
	recruiterBID, recruiterBToken := createRecruiter("recruitContactB")
	_ = recruiterBID
	_ = recruiterAID
	_ = recruiterBToken

	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	studentToken, _ := studentSess.Issue(stu.ID, stu.Account, "hrwai_user")

	// 1. 附言必填：空应 400
	rec := doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": ""})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("空附言应 400, 实际 %d %s", rec.Code, rec.Body.String())
	}
	// 超长 201 字应 400
	longMsg := strings.Repeat("a", 201)
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": longMsg})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("超长附言应 400, 实际 %d", rec.Code)
	}
	// 正常申请
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": "您好，想了解叉车维修岗位"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("正常申请应 201, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var createdReq struct {
		Code int `json:"code"`
		Data struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createdReq); err != nil {
		t.Fatalf("parse createdReq: %v", err)
	}
	if createdReq.Data.Status != "pending" {
		t.Fatalf("新申请应 pending, 实际 %s", createdReq.Data.Status)
	}
	reqID := createdReq.Data.ID

	// 2. pending 唯一：重复提交应 400
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": "再次申请"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("重复 pending 应 400, 实际 %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "已存在") {
		t.Fatalf("重复 pending 文案应提及已存在, 实际 %s", rec.Body.String())
	}

	// 3. 学员侧收到站内信，不含企业电话
	// 直接查 notifications 表
	var notifs []model.Notification
	if err := db.Where("user_id = ?", stu.ID).Order("created_at DESC").Find(&notifs).Error; err != nil {
		t.Fatalf("query notifs: %v", err)
	}
	if len(notifs) == 0 {
		t.Fatalf("学员应收到站内信")
	}
	latest := notifs[0]
	if strings.Contains(latest.Content, "13800001111") {
		t.Fatalf("站内信不应包含企业联系电话, 实际 %s", latest.Content)
	}
	if !strings.Contains(latest.Content, "测试企业-recruitContactA") {
		t.Fatalf("站内信应含企业名, 实际 %s", latest.Content)
	}
	if latest.Link != "/training/resume" {
		t.Fatalf("link 应为 /training/resume, 实际 %s", latest.Link)
	}

	// 学员侧列表：应含该申请，企业名、联系人、附言，不含电话
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/contact-requests", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员列表应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var listResp struct {
		Code int `json:"code"`
		Data struct {
			Items []map[string]any `json:"items"`
			Total int64            `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("parse listResp: %v", err)
	}
	if listResp.Data.Total != 1 {
		t.Fatalf("学员收到的申请应 1, 实际 %d", listResp.Data.Total)
	}
	item := listResp.Data.Items[0]
	if _, hasPhone := item["contact_phone"]; hasPhone {
		t.Fatalf("学员侧申请不应含 contact_phone, 实际 %v", item)
	}
	if item["company_name"] == nil || item["contact_name"] == nil {
		t.Fatalf("学员侧应含 company_name/contact_name, 实际 %v", item)
	}
	rawStudentList := rec.Body.String()
	if strings.Contains(rawStudentList, "13800001111") {
		t.Fatalf("学员侧列表不应含企业电话")
	}

	// 招聘方我的申请列表：应同步为 pending
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/contact-requests", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘方列表应 200, 实际 %d", rec.Code)
	}
	var recList struct {
		Code int `json:"code"`
		Data struct {
			Items []struct {
				Status string `json:"status"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &recList); err != nil {
		t.Fatalf("parse recList: %v", err)
	}
	if recList.Data.Items[0].Status != "pending" {
		t.Fatalf("招聘方列表应 pending, 实际 %s", recList.Data.Items[0].Status)
	}

	// 4. 明文读取在授权前应失败（403）
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID)+"/contact", nil)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest {
		t.Fatalf("未授权读取明文应 403, 实际 %d %s", rec.Code, rec.Body.String())
	}
	// L2 阶段脱敏仍成立（即使未授权，脱敏接口不应含明文）
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID), nil)
	if strings.Contains(rec.Body.String(), "13800009999") || strings.Contains(rec.Body.String(), "zhang_wx") {
		t.Fatalf("L2 脱敏不应含明文 phone/wechat, body=%s", rec.Body.String())
	}

	// 5. 学员同意
	rec = doWithToken(t, r, studentToken, http.MethodPost, "/api/resume/contact-requests/"+strconv.Itoa(int(reqID))+"/approve", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("同意应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	// 招聘方列表同步为 approved
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/contact-requests", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &recList); err != nil {
		t.Fatalf("parse after approve: %v", err)
	}
	if recList.Data.Items[0].Status != "approved" {
		t.Fatalf("同意后招聘方应 approved, 实际 %s", recList.Data.Items[0].Status)
	}
	// 招聘方收到邮件（此处无法直接验证邮件，但服务层已调用 mailer，若为 LogMailSender 则日志有；我们只验证状态同步）

	// 6. 明文读取成功（approved → 读取成功）
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID)+"/contact", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("授权后读取明文应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var contactResp struct {
		Code int `json:"code"`
		Data struct {
			ContactPhone  string `json:"contact_phone"`
			Wechat        string `json:"wechat"`
			RealName      string `json:"real_name"`
			ResumeFileURL string `json:"resume_file_url"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &contactResp); err != nil {
		t.Fatalf("parse contactResp: %v", err)
	}
	if contactResp.Data.ContactPhone != "13800009999" || contactResp.Data.Wechat != "zhang_wx" || contactResp.Data.RealName != "张三丰" {
		t.Fatalf("明文联系方式错误 %+v", contactResp.Data)
	}
	if contactResp.Data.ResumeFileURL != "/static/uploads/resumes/a.pdf" {
		t.Fatalf("PDF 地址错误 %q", contactResp.Data.ResumeFileURL)
	}
	// L2 脱敏在授权后仍成立（脱敏接口仍不含明文）
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID), nil)
	if strings.Contains(rec.Body.String(), "13800009999") {
		t.Fatalf("授权后 L2 仍应脱敏，不含明文 phone")
	}

	// 7. 撤回后读取立即失败（实时校验，无缓存）
	rec = doWithToken(t, r, studentToken, http.MethodPost, "/api/resume/contact-requests/"+strconv.Itoa(int(reqID))+"/revoke", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("撤回应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu.ID)+"/contact", nil)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest {
		t.Fatalf("撤回后读取应 403, 实际 %d", rec.Code)
	}

	// 8. 冷却期：被撤回后 30 天内不能再申请
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": "再次申请冷却期"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("冷却期内应 400, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "冷却") {
		t.Fatalf("冷却期文案应提及冷却, 实际 %s", rec.Body.String())
	}
	// 手动把 decided_at 改为 31 天前，模拟过期，之后应可再申请
	oldDecided := time.Now().AddDate(0, 0, -31)
	if err := db.Model(&model.ContactRequest{}).Where("id = ?", reqID).Update("decided_at", oldDecided).Error; err != nil {
		t.Fatalf("update decided_at: %v", err)
	}
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": "冷却后再次申请"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("冷却期后应可再申请, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var secondReq struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &secondReq); err != nil {
		t.Fatalf("parse secondReq: %v", err)
	}
	secondID := secondReq.Data.ID

	// 9. 日限：单个企业每日 20 条
	// 将已有申请的 created_at 改为昨天，避免影响今天计数
	yesterday := time.Now().AddDate(0, 0, -1)
	_ = db.Model(&model.ContactRequest{}).Where("recruiter_id = ?", recruiterBID).Update("created_at", yesterday).Error
	// 用 recruiterB 连续创建 20 条（对不同学员）
	for i := 0; i < 20; i++ {
		tmpStu := testutil.SeedStudent(t, db, "stuDaily"+strconv.Itoa(i), pwd)
		tmpCard := model.JobCard{UserID: tmpStu.ID, RealName: "临时", Visibility: "open", ExpectedRegions: model.JSONB([]byte(`[]`))}
		_ = db.Create(&tmpCard).Error
		rec = doWithToken(t, r, recruiterBToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": tmpStu.ID, "message": "日限测试"})
		if rec.Code != http.StatusCreated {
			t.Fatalf("日限内第 %d 条应 201, 实际 %d %s", i+1, rec.Code, rec.Body.String())
		}
	}
	// 第 21 条应被拒
	extraStu := testutil.SeedStudent(t, db, "stuDailyExtra", pwd)
	extraCard := model.JobCard{UserID: extraStu.ID, RealName: "临时", Visibility: "open", ExpectedRegions: model.JSONB([]byte(`[]`))}
	_ = db.Create(&extraCard).Error
	rec = doWithToken(t, r, recruiterBToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": extraStu.ID, "message": "超出日限"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("超出日限应 400, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "上限") {
		t.Fatalf("日限文案应提及上限, 实际 %s", rec.Body.String())
	}

	// 10. 超期 pending 的三条收敛出口（#1197 / ADR-0061 §2）。
	// 每一段都只断言「被测路径自己把状态推进了」：旧版在此写成
	// `if status != "expired" { 现场 new 一个 ContactService 调 ExpirePending 再断言 }`，
	// 机制不跑也恒绿——那正是守护漏装能静默存活三周的原因，不得复发。
	setPastPending := func(id int64) {
		t.Helper()
		if err := db.Model(&model.ContactRequest{}).Where("id = ?", id).
			Updates(map[string]any{"status": string(service.ContactGrantPending), "expires_at": time.Now().Add(-time.Hour)}).Error; err != nil {
			t.Fatalf("置为超期 pending: %v", err)
		}
	}
	// 每次都用**全新的 dest**：gorm 会把 dest 上已填的主键折进 WHERE，
	// 复用同一个变量查第二条记录会得到 `id = 旧 AND id = 新` ⇒ 恒 record not found。
	statusOf := func(id int64) string {
		t.Helper()
		var row model.ContactRequest
		if err := db.First(&row, id).Error; err != nil {
			t.Fatalf("find %d: %v", id, err)
		}
		return row.Status
	}

	// 出口一：学员点同意 → 按行当场落态并拒绝（窗口已闭的申请不可被裁决）。
	setPastPending(secondID)
	rec = doWithToken(t, r, studentToken, http.MethodPost, "/api/resume/contact-requests/"+strconv.Itoa(int(secondID))+"/approve", nil)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "已过期") {
		t.Fatalf("超期同意应 400 且提示已过期, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if got := statusOf(secondID); got != string(service.ContactGrantExpired) {
		t.Fatalf("approve 的按行出口应把超期 pending 落为 expired, 实际 %s", got)
	}

	// 出口二·甲：expired 不进冷却（冷却只认 rejected/revoked）⇒ 学员不响应不该永久堵死企业。
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": "闭窗后重发"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("超期落 expired 后应可立即重发, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var thirdReq struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &thirdReq); err != nil {
		t.Fatalf("parse thirdReq: %v", err)
	}
	thirdID := thirdReq.Data.ID

	// 出口二·乙：Create 判唯一前的**定向落态**——不依赖守护 tick，重试当场拿到真结论。
	setPastPending(thirdID)
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu.ID, "message": "闭窗未收敛时重发"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create 应先定向落态再判唯一, 实际 %d %s", rec.Code, rec.Body.String())
	}
	if got := statusOf(thirdID); got != string(service.ContactGrantExpired) {
		t.Fatalf("定向落态应把闭窗 pending 置 expired, 实际 %s", got)
	}

	// 出口三：守护的收敛动作本身，用**装配根里那个实例**（handler 真正持有的 deps.ContactSvc），
	// 而不是现场 new 一个——否则测的就不是被接线的那条路径。
	var fourthReq struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &fourthReq); err != nil {
		t.Fatalf("parse fourthReq: %v", err)
	}
	fourthID := fourthReq.Data.ID
	setPastPending(fourthID)
	affected, err := deps.ContactSvc.ExpirePending(time.Now())
	if err != nil {
		t.Fatalf("ExpirePending: %v", err)
	}
	if affected < 1 {
		t.Fatalf("守护收敛应至少落 1 条, 实际 %d", affected)
	}
	if got := statusOf(fourthID); got != string(service.ContactGrantExpired) {
		t.Fatalf("守护应把闭窗 pending 落为 expired, 实际 %s", got)
	}

	// 11. 学员注销后授权失效
	// 创建一个新的学员与申请，同意后注销学员，招聘方读取应失败
	stu2 := testutil.SeedStudent(t, db, "stuToDelete", pwd)
	card2 := model.JobCard{UserID: stu2.ID, RealName: "待删学员", ContactPhone: "13900001111", Wechat: "todelete_wx", Visibility: "open", ExpectedRegions: model.JSONB([]byte(`[]`))}
	_ = db.Create(&card2).Error
	// 用 recruiterA 对 stu2 发起申请（需要冷却已过，但 recruiterA 对 stu2 无冷却）
	rec = doWithToken(t, r, recruiterAToken, http.MethodPost, "/api/recruit/contact-requests", map[string]any{"student_user_id": stu2.ID, "message": "注销测试"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("注销前申请应 201, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var delReq struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &delReq); err != nil {
		t.Fatalf("parse delReq: %v", err)
	}
	studentSess2 := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	stu2Token, _ := studentSess2.Issue(stu2.ID, stu2.Account, "hrwai_user")
	rec = doWithToken(t, r, stu2Token, http.MethodPost, "/api/resume/contact-requests/"+strconv.Itoa(int(delReq.Data.ID))+"/approve", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("注销前同意应 200, 实际 %d", rec.Code)
	}
	// 招聘方此时可读
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu2.ID)+"/contact", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("注销前明文应可读 200, 实际 %d", rec.Code)
	}
	// 注销学员
	// 通过 AuthService 删除（另起一个装配实例，避免与 r 背后的 deps 混淆——路由仍指向旧实例）
	deleteDeps := newContractDeps(t, db, cfg)
	if err := deleteDeps.AuthSvc.DeleteAccount(stu2.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	// 再次读取应失败
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu2.ID)+"/contact", nil)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("注销后读取应失败, 实际 %d %s", rec.Code, rec.Body.String())
	}
}
