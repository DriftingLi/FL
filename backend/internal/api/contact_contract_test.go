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
	r := NewRouter(newContractDeps(t, db, cfg))

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
	if err := db.Model(&model.ContactRequest{}).Where("recruiter_id = ?", recruiterBID).Update("created_at", yesterday).Error; err != nil {
		// ignore if no rows
	}
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

	// 10. 14 天过期：pending 14 天后自动过期
	// 将 secondID 的 expires_at 设为过去，调用 ExpirePending
	if err := db.Model(&model.ContactRequest{}).Where("id = ?", secondID).Updates(map[string]any{"expires_at": time.Now().Add(-time.Hour), "status": "pending"}).Error; err != nil {
		t.Fatalf("update expires_at: %v", err)
	}
	// 直接调用 service 的 ExpirePending（通过 deps 获取）
	// 这里通过 db 直接验证：调用 API 的过期检查会在 approve 时触发，但我们直接测试 service
	// 通过查询 pending 数量
	var pendingCnt int64
	db.Model(&model.ContactRequest{}).Where("status = ?", "pending").Count(&pendingCnt)
	// 手动触发过期（使用 ContactService 直接）
	// 获取 service 实例 via new deps? 我们无法直接拿到 service，但可以通过 db 更新后检查 approve 是否会转 expired
	rec = doWithToken(t, r, studentToken, http.MethodPost, "/api/resume/contact-requests/"+strconv.Itoa(int(secondID))+"/approve", nil)
	// 由于已过期，approve 应失败并提示过期
	if rec.Code == http.StatusOK {
		t.Fatalf("过期申请同意应失败, 实际 200")
	}
	// 直接通过 DB 检查 status 已被标记为 expired（在 Approve 中会转为 expired）
	var after model.ContactRequest
	if err := db.First(&after, secondID).Error; err != nil {
		t.Fatalf("find after: %v", err)
	}
	if after.Status != "expired" {
		// 如果未自动过期，手动调用 ExpirePending via service（我们需要拿到 service）
		// 通过 NewContactService 临时创建
		svc := service.NewContactService(db, nil, nil, nil)
		if _, err := svc.ExpirePending(time.Now()); err != nil {
			t.Fatalf("expire pending: %v", err)
		}
		_ = db.First(&after, secondID).Error
		if after.Status != "expired" {
			t.Fatalf("过期后 status 应为 expired, 实际 %s", after.Status)
		}
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
	// 通过 AuthService 删除
	deps := newContractDeps(t, db, cfg)
	if err := deps.AuthSvc.DeleteAccount(stu2.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	// 再次读取应失败
	rec = doWithToken(t, r, recruiterAToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu2.ID)+"/contact", nil)
	if rec.Code != http.StatusForbidden && rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
		t.Fatalf("注销后读取应失败, 实际 %d %s", rec.Code, rec.Body.String())
	}
}
