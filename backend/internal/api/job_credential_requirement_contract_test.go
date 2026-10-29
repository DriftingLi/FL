// 契约测试 #2066：职位所需证件（单一「要求」档，可空多选；无「优先/加分」语义）。
// 守的边界：
//   - 写面：多选落库（重复去重）、非法 / 不存在的 id → 400、空数组与字段缺省等价；
//   - 读面：`required_credentials` 恒发（无要求时是 []，不是缺席）；按证件筛选「命中任一即返回」，
//     无要求的职位不命中；总数不因多证件被放大（EXISTS 而非 JOIN）；
//   - 编辑：整组替换，清空后回到「不限」；
//   - 筛选参数显式非法 → 400（静默忽略会让调用方以为结果已按证件收窄）。
//
// 双适配器：SQLite 恒绿 + Postgres（真实迁移建表，无 DATABASE_URL 时跳过）。
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/core"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// jobCredentialDTO 职位响应里本票关心的两格（其余字段由别的契约测试钉）。
type jobCredentialDTO struct {
	CredentialID int    `json:"credential_id"`
	Name         string `json:"name"`
}

// jobCredentialResp 单个职位的响应载荷。
type jobCredentialResp struct {
	ID                  int                `json:"id"`
	RequiredCredentials []jobCredentialDTO `json:"required_credentials"`
}

// jobCredentialList 职位列表响应载荷（只取本票要用的两格）。
type jobCredentialList struct {
	Items []jobCredentialResp `json:"items"`
	Total int64               `json:"total"`
}

// seedJobCredential 建一枚目标证件（编码唯一，sort_order 决定输出顺序）。
func seedJobCredential(t *testing.T, db *gorm.DB, code, name string, sortOrder int) int {
	t.Helper()
	c := model.Credential{
		Code: code, Name: name, Category: "special_operation",
		SortOrder: sortOrder, Status: 1, CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&c).Error; err != nil {
		t.Fatalf("建证件 %s 失败: %v", code, err)
	}
	return c.ID
}

// decodeJobData 取信封里的 data（单个职位）。
func decodeJobData(t *testing.T, body []byte) jobCredentialResp {
	t.Helper()
	var env struct {
		Data jobCredentialResp `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("解析职位响应失败: %v body=%s", err, string(body))
	}
	return env.Data
}

// decodeJobList 取信封里的 data（职位列表）。
func decodeJobList(t *testing.T, body []byte) jobCredentialList {
	t.Helper()
	var env struct {
		Data jobCredentialList `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("解析职位列表失败: %v body=%s", err, string(body))
	}
	return env.Data
}

func assertJobCredentialRequirementContract(t *testing.T, db *gorm.DB) {
	testutil.SetTestGinMode()
	adminPwd, _ := core.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminJobCred", adminPwd)
	stuPwd, _ := core.HashPassword("student123")
	stu := seedStudent(t, db, "stuJobCred", stuPwd)

	cfg := &config.Config{JWTSecretKey: "job-credential-secret", JWTExpiresHours: 2}
	r := NewRouter(newContractDeps(t, db, cfg))
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")
	stuSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	stuToken, _ := stuSess.Issue(stu.ID, stu.Username, "hrwai_user")

	// 招聘者经管理端邀约创建（与既有两个职位契约测试同一入口）
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", map[string]any{
		"username": "jobCredRec", "password": "recruit123", "company_name": "证件要求测试企业",
		"credit_code": "91110000JOBCRED", "business_scope": "叉车维修", "contact_name": "李四",
		"contact_phone": "13800008888", "contact_email": "jobcred@example.com",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败: %d %s", rec.Code, rec.Body.String())
	}
	login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login",
		map[string]any{"username": "jobCredRec", "password": "recruit123"})
	if login.Code != http.StatusOK {
		t.Fatalf("招聘者登录失败: %d %s", login.Code, login.Body.String())
	}
	var lr loginResp
	_ = json.Unmarshal(login.Body.Bytes(), &lr)
	recToken := lr.Data.Token

	db.Exec("INSERT INTO positions (code, name, status) VALUES (?, ?, ?)", "job_cred_tech", "叉车维修技师", 1)
	var posID int
	db.Raw("SELECT position_id FROM positions WHERE code = ?", "job_cred_tech").Scan(&posID)
	if posID == 0 {
		t.Fatal("岗位种子没建成")
	}

	// sort_order 故意与入参顺序相反：输出顺序按字典序（sort_order, id），不按提交顺序。
	credA := seedJobCredential(t, db, "JOB_CRED_A", "叉车司机证", 2)
	credB := seedJobCredential(t, db, "JOB_CRED_B", "叉车维修证", 1)

	createJob := func(title string, extra map[string]any) *httptest.ResponseRecorder {
		body := map[string]any{"title": title, "position_id": posID, "region": "江苏苏州"}
		for k, v := range extra {
			body[k] = v
		}
		return doWithToken(t, r, recToken, http.MethodPost, "/api/recruit/jobs", body)
	}

	// 1. 多选落库：重复 id 去重，输出按字典排序
	rec = createJob("双证职位", map[string]any{"required_credential_ids": []int{credA, credB, credB}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建双证职位失败: %d %s", rec.Code, rec.Body.String())
	}
	job1 := decodeJobData(t, rec.Body.Bytes())
	if len(job1.RequiredCredentials) != 2 {
		t.Fatalf("重复 id 应去重成 2 条, got %+v", job1.RequiredCredentials)
	}
	if job1.RequiredCredentials[0].CredentialID != credB || job1.RequiredCredentials[1].CredentialID != credA {
		t.Fatalf("所需证件应按 sort_order 排序 [%d %d], got %+v", credB, credA, job1.RequiredCredentials)
	}
	if job1.RequiredCredentials[0].Name != "叉车维修证" {
		t.Fatalf("所需证件应带名称, got %+v", job1.RequiredCredentials[0])
	}
	var rows int64
	db.Model(&model.JobCredentialRequirement{}).Where("job_posting_id = ?", job1.ID).Count(&rows)
	if rows != 2 {
		t.Fatalf("关联行应只有 2 条（去重后）, got %d", rows)
	}

	// 2/3. 字段缺省与空数组等价：都是「不限」，且响应里键在场、值为 []
	rec = createJob("不限证件职位", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建不限证件职位失败: %d %s", rec.Code, rec.Body.String())
	}
	job2 := decodeJobData(t, rec.Body.Bytes())
	if !strings.Contains(rec.Body.String(), `"required_credentials":[]`) {
		t.Fatalf("无要求时也必须发 []（不能缺席）, got %s", rec.Body.String())
	}
	rec = createJob("空数组职位", map[string]any{"required_credential_ids": []int{}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建空数组职位失败: %d %s", rec.Code, rec.Body.String())
	}
	job3 := decodeJobData(t, rec.Body.Bytes())
	if !strings.Contains(rec.Body.String(), `"required_credentials":[]`) {
		t.Fatalf("空数组应等价于不限, got %s", rec.Body.String())
	}
	db.Model(&model.JobCredentialRequirement{}).Where("job_posting_id IN ?", []int{job2.ID, job3.ID}).Count(&rows)
	if rows != 0 {
		t.Fatalf("不限证件的职位不应有要求行, got %d", rows)
	}

	// 4/5. 非法 id → 400（不存在 / 非正数）
	rec = createJob("不存在证件", map[string]any{"required_credential_ids": []int{999999}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("不存在的证件应 400, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "所需证件") {
		t.Fatalf("400 文案应说清是所需证件, got %s", rec.Body.String())
	}
	rec = createJob("零号证件", map[string]any{"required_credential_ids": []int{0}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非正数证件 id 应 400, got %d %s", rec.Code, rec.Body.String())
	}

	listJobs := func(query string) jobCredentialList {
		path := "/api/jobs"
		if query != "" {
			path += "?" + query
		}
		res := doWithToken(t, r, stuToken, http.MethodGet, path, nil)
		if res.Code != http.StatusOK {
			t.Fatalf("学员职位列表失败: %d %s", res.Code, res.Body.String())
		}
		return decodeJobList(t, res.Body.Bytes())
	}

	// 6. 不加筛选：全部 open 职位，且排序仍是 published_at DESC, id DESC
	all := listJobs("")
	if all.Total != 3 || len(all.Items) != 3 {
		t.Fatalf("无筛选应回 3 个职位, got total=%d items=%d", all.Total, len(all.Items))
	}
	if all.Items[0].ID != job3.ID || all.Items[2].ID != job1.ID {
		t.Fatalf("既有排序（published_at DESC, id DESC）被改动: %+v", all.Items)
	}

	// 7. 双证职位：命中任一即返回（A、B 各查一次都命中它）
	for _, q := range []string{"credential_id=" + itoa(credA), "credential_id=" + itoa(credB)} {
		got := listJobs(q)
		if got.Total != 1 || len(got.Items) != 1 || got.Items[0].ID != job1.ID {
			t.Fatalf("%s 应只命中双证职位, got total=%d %+v", q, got.Total, got.Items)
		}
	}

	// 8. 只要求 B 的职位：A 不该命中它（证明是「命中任一」而不是「有要求就返回」）
	rec = createJob("单证职位", map[string]any{"required_credential_ids": []int{credB}})
	if rec.Code != http.StatusCreated {
		t.Fatalf("建单证职位失败: %d %s", rec.Code, rec.Body.String())
	}
	job4 := decodeJobData(t, rec.Body.Bytes())
	if got := listJobs("credential_id=" + itoa(credA)); got.Total != 1 || got.Items[0].ID != job1.ID {
		t.Fatalf("A 筛选应只命中双证职位（总数不因多证件放大）, got total=%d %+v", got.Total, got.Items)
	}
	if got := listJobs("credential_id=" + itoa(credB)); got.Total != 2 {
		t.Fatalf("B 筛选应命中双证 + 单证两个职位, got total=%d %+v", got.Total, got.Items)
	}
	// 无要求的职位在任何证件筛选下都不出现
	for _, q := range []string{"credential_id=" + itoa(credA), "credential_id=" + itoa(credB)} {
		for _, item := range listJobs(q).Items {
			if item.ID == job2.ID || item.ID == job3.ID {
				t.Fatalf("不限证件的职位不该被证件筛选命中: job=%d query=%s", item.ID, q)
			}
		}
	}

	// 9. 合法但不存在的证件 id：空结果（筛选不校验字典存在性，与其它筛选轴同口径）
	if got := listJobs("credential_id=999999"); got.Total != 0 || len(got.Items) != 0 {
		t.Fatalf("不存在的证件筛选应回空, got total=%d %+v", got.Total, got.Items)
	}

	// 10. 显式非法筛选值 → 400（静默忽略会让人以为已按证件收窄）
	for _, q := range []string{"credential_id=abc", "credential_id=0", "credential_id=-3"} {
		res := doWithToken(t, r, stuToken, http.MethodGet, "/api/jobs?"+q, nil)
		if res.Code != http.StatusBadRequest {
			t.Fatalf("%s 应 400, got %d %s", q, res.Code, res.Body.String())
		}
		if !strings.Contains(res.Body.String(), "证件") {
			t.Fatalf("%s 的 400 文案应提到证件, got %s", q, res.Body.String())
		}
	}

	// 11. 编辑回显 + 整组替换
	rec = doWithToken(t, r, recToken, http.MethodPut, "/api/recruit/jobs/"+itoa(job1.ID), map[string]any{
		"title": "双证职位", "position_id": posID, "region": "江苏苏州",
		"required_credential_ids": []int{credA},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("编辑职位失败: %d %s", rec.Code, rec.Body.String())
	}
	if updated := decodeJobData(t, rec.Body.Bytes()); len(updated.RequiredCredentials) != 1 ||
		updated.RequiredCredentials[0].CredentialID != credA {
		t.Fatalf("编辑后应只剩 A 一条, got %+v", updated.RequiredCredentials)
	}
	rec = doWithToken(t, r, recToken, http.MethodGet, "/api/recruit/jobs/"+itoa(job1.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("企业侧职位详情失败: %d %s", rec.Code, rec.Body.String())
	}
	if got := decodeJobData(t, rec.Body.Bytes()); len(got.RequiredCredentials) != 1 {
		t.Fatalf("企业侧详情应带回证件要求（供编辑回显）, got %+v", got.RequiredCredentials)
	}
	// 清空 ⇒ 回到「不限」
	rec = doWithToken(t, r, recToken, http.MethodPut, "/api/recruit/jobs/"+itoa(job1.ID), map[string]any{
		"title": "双证职位", "position_id": posID, "region": "江苏苏州",
		"required_credential_ids": []int{},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("清空证件失败: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"required_credentials":[]`) {
		t.Fatalf("清空后应发 [], got %s", rec.Body.String())
	}
	if got := listJobs("credential_id=" + itoa(credA)); got.Total != 0 {
		t.Fatalf("清空后 A 筛选不该再命中它, got total=%d %+v", got.Total, got.Items)
	}

	// 12. 学员详情同样带要求（移动端职位卡渲染「需持 X 证」的读面）
	rec = doWithToken(t, r, stuToken, http.MethodGet, "/api/jobs/"+itoa(job4.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员职位详情失败: %d %s", rec.Code, rec.Body.String())
	}
	if got := decodeJobData(t, rec.Body.Bytes()); len(got.RequiredCredentials) != 1 ||
		got.RequiredCredentials[0].CredentialID != credB {
		t.Fatalf("学员详情应带所需证件, got %+v", got.RequiredCredentials)
	}
}

func TestJobCredentialRequirementContract_OnSqlite(t *testing.T) {
	assertJobCredentialRequirementContract(t, testutil.NewMemoryDB(t))
}

func TestJobCredentialRequirementContract_OnPostgres(t *testing.T) {
	assertJobCredentialRequirementContract(t, testutil.NewPostgresDB(t))
}
