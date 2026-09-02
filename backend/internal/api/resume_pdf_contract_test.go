// 契约测试 #485：在线简历 PDF 实时渲染与详情页内嵌预览。
// 锁定外部行为：recruiter 鉴权（非招聘角色 403/401）、隐藏卡 404、
// 打码口径（文本流含打码姓名、不含明文电话/微信/证书原图/工作照/现居地精确地址）、
// 学员本人端点鉴权（非本人 404/403）。
package api

import (
	"bytes"
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

func TestResumePDFContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	spec1 := model.Position{Code: "pdf_forklift", Name: "叉车维修", Status: 1}
	if err := db.Create(&spec1).Error; err != nil {
		t.Fatalf("create spec: %v", err)
	}
	cred1 := model.Credential{Code: "pdf_cred_n1", Name: "叉车N1", Category: "special_operation", Status: 1}
	if err := db.Create(&cred1).Error; err != nil {
		t.Fatalf("create cred: %v", err)
	}

	pwd, _ := service.HashPassword("pass1234")
	stu1 := testutil.SeedStudent(t, db, "stuPdf1", pwd)
	stu2 := testutil.SeedStudent(t, db, "stuPdf2", pwd)
	stu3 := testutil.SeedStudent(t, db, "stuPdfHidden", pwd)
	ip := func(v int) *int { return &v }
	now := time.Now()
	card1 := model.JobCard{
		UserID: stu1.ID, RealName: "张三丰", ContactPhone: "13800000001", Wechat: "zhang_wx", Region: "江苏苏州精确地址123号",
		ExpectedPositionID: &spec1.PositionID, ExpectedPositionExtra: "叉车维修技师", ExpectedRegions: model.JSONB([]byte(`["江苏苏州"]`)),
		SalaryMin: ip(8000), SalaryMax: ip(12000), SalaryNegotiable: false,
		AvailableIn: "immediate", JobNature: "fulltime", ExperienceYears: 5, SelfIntro: "5年叉车维修经验，熟悉电动叉车故障诊断",
		ResumeExperiences:    model.JSONB([]byte(`[{"company":"A公司","role":"维修工","start_month":"2020-01","end_month":"2023-01","desc":"负责叉车日常维修"}]`)),
		ResumeCertifications: model.JSONB([]byte(`[{"credential_id":` + strconv.Itoa(cred1.ID) + `,"cert_no":"CERT001","expire_date":"2028-01-01","image_urls":["http://example.com/cert1.jpg"]}]`)),
		ResumeFileURL:        "/static/uploads/resumes/stu1.pdf", Photos: model.JSONB([]byte(`["http://example.com/photo1.jpg"]`)), Visibility: "open",
		CreatedAt: now, UpdatedAt: now,
	}
	card2 := model.JobCard{
		UserID: stu2.ID, RealName: "李四", ContactPhone: "13900000002", Wechat: "li_wx", Region: "浙江杭州",
		ExpectedPositionID: nil, ExpectedPositionExtra: "电工", ExpectedRegions: model.JSONB([]byte(`["浙江杭州"]`)),
		SalaryMin: ip(10000), SalaryMax: ip(15000), SalaryNegotiable: false,
		AvailableIn: "1w", JobNature: "parttime", ExperienceYears: 2, SelfIntro: "2年电工经验",
		ResumeExperiences: model.JSONB([]byte("[]")), ResumeCertifications: model.JSONB([]byte("[]")),
		ResumeFileURL: "", Photos: model.JSONB([]byte("[]")), Visibility: "open",
		CreatedAt: now, UpdatedAt: now,
	}
	cardHidden := model.JobCard{
		UserID: stu3.ID, RealName: "王五", ContactPhone: "13700000003", Wechat: "wang_wx", Region: "上海",
		ExpectedRegions: model.JSONB([]byte(`["上海"]`)), Visibility: "hidden",
		CreatedAt: now, UpdatedAt: now,
	}
	for _, c := range []model.JobCard{card1, card2, cardHidden} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("create card %d: %v", c.UserID, err)
		}
	}

	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminPdf", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")

	// 建招聘者并登录
	body := map[string]any{
		"username": "recruitPdf1", "password": "recruit123", "company_name": "测试企业-pdf", "credit_code": "91110000MApdf", "business_scope": "叉车维修", "contact_name": "联系人", "contact_phone": "13800001111", "contact_email": "pdf@example.com",
	}
	rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("建招聘者失败 %d %s", rec.Code, rec.Body.String())
	}
	recruiterToken := func() string {
		rec2 := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login", map[string]any{"username": "recruitPdf1", "password": "recruit123"})
		if rec2.Code != http.StatusOK {
			t.Fatalf("recruiter login fail %d %s", rec2.Code, rec2.Body.String())
		}
		var resp loginResp
		if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
			t.Fatalf("parse login: %v", err)
		}
		return resp.Data.Token
	}()
	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	studentToken, _ := studentSess.Issue(stu1.ID, stu1.Account, "hrwai_user")
	student2Token, _ := studentSess.Issue(stu2.ID, stu2.Account, "hrwai_user")

	// ===== 鉴权 =====
	// 未登录 401
	rec = doWithoutToken(t, r, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu1.ID)+"/pdf")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未登录 PDF 应 401, 实际 %d", rec.Code)
	}
	// 学员访问招聘端 PDF 403
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu1.ID)+"/pdf", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("学员访问招聘端 PDF 应 403, 实际 %d %s", rec.Code, rec.Body.String())
	}
	// 招聘者访问学员侧本人 PDF 403
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/resume/pdf", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("招聘者访问学员侧 PDF 应 403, 实际 %d %s", rec.Code, rec.Body.String())
	}

	// ===== 招聘者正常取流：打码口径 =====
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu1.ID)+"/pdf", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("招聘者 PDF 应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/pdf") {
		t.Fatalf("Content-Type 应为 application/pdf, 实际 %q", ct)
	}
	pdfBody := rec.Body.Bytes()
	if len(pdfBody) < 1000 {
		t.Fatalf("PDF 字节过短 %d", len(pdfBody))
	}
	// 打码口径：解 UTF-16 文本流含打码姓名与字段，不含明文电话/微信/证书原图/工作照/精确地址
	decoded := decodePDFUTF16(pdfBody)
	if decoded == "" {
		t.Fatalf("PDF 文本流解码为空，无法断言打码口径")
	}
	for _, needle := range []string{"13800000001", "zhang_wx", "cert1.jpg", "photo1.jpg", "江苏苏州精确地址123号", "张三丰"} {
		if strings.Contains(decoded, needle) {
			t.Fatalf("打码 PDF 不应包含 %q", needle)
		}
	}
	if !strings.Contains(decoded, "张*丰") {
		t.Fatalf("打码 PDF 应含打码姓名 张*丰，解码文本=%s", decoded)
	}
	for _, needle := range []string{"叉车维修技师", "5年叉车维修经验", "CERT001", "A公司", "全职", "现居地", "江苏省/苏州市"} {
		if !strings.Contains(decoded, needle) {
			t.Fatalf("打码 PDF 应含 %q（解码文本=%s）", needle, decoded)
		}
	}

	// ===== 隐藏卡 404 =====
	rec = doWithToken(t, r, recruiterToken, http.MethodGet, "/api/recruit/resumes/"+strconv.Itoa(stu3.ID)+"/pdf", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hidden 卡 PDF 应 404, 实际 %d %s", rec.Code, rec.Body.String())
	}

	// ===== 学员本人端点 =====
	rec = doWithToken(t, r, studentToken, http.MethodGet, "/api/resume/pdf", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员本人 PDF 应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	rawMine := decodePDFUTF16(rec.Body.Bytes())
	for _, needle := range []string{"13800000001", "zhang_wx", "cert1.jpg", "张三丰"} {
		if strings.Contains(rawMine, needle) {
			t.Fatalf("学员本人打码 PDF 不应包含 %q", needle)
		}
	}
	if !strings.Contains(rawMine, "张*丰") {
		t.Fatalf("学员本人 PDF 应含打码姓名")
	}
	// 学员 2 看学员 1 的：本人端点按自己鉴权，不暴露他人（404——无本人卡时）
	rec = doWithToken(t, r, student2Token, http.MethodGet, "/api/resume/pdf", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("学员2 本人卡应 200, 实际 %d", rec.Code)
	}
}

// decodePDFUTF16 从 PDF 内容流中抽取文本（gofpdf 把 UTF-16BE 文本以转义字面量写入未压缩流）。
// 解析 (…)Tj 文本字面量并按 UTF-16BE 解码，返回拼接文本供打码口径断言。
func decodePDFUTF16(data []byte) string {
	// 定位每个对象字典与 stream；跳过 FlateDecode 压缩流（内容流 test_compress=false 未压缩）。
	var out strings.Builder
	i := 0
	for i < len(data) {
		// 找对象字典开始 <<
		dictStart := indexBytes(data, []byte("<<"), i)
		if dictStart < 0 {
			break
		}
		dictEnd := indexBytes(data, []byte(">>"), dictStart)
		if dictEnd < 0 {
			break
		}
		dict := string(data[dictStart : dictEnd+2])
		streamStart := indexBytes(data, []byte("stream"), dictEnd)
		if streamStart < 0 {
			break
		}
		// stream 关键字后可能有 CRLF 或 LF
		bodyStart := streamStart + len("stream")
		if bodyStart < len(data) && (data[bodyStart] == '\r' || data[bodyStart] == '\n') {
			bodyStart++
			// CRLF 双字节：跳过第二字节
			if bodyStart < len(data) && data[bodyStart-1] == '\r' && data[bodyStart] == '\n' {
				bodyStart++
			}
		}
		bodyEnd := indexBytes(data, []byte("endstream"), bodyStart)
		if bodyEnd < 0 {
			break
		}
		compressed := strings.Contains(dict, "FlateDecode")
		if !compressed {
			parsePDFTextLiterals(data[bodyStart:bodyEnd], &out)
		}
		i = bodyEnd + len("endstream")
	}
	return out.String()
}

// parsePDFTextLiterals 解析未压缩内容流中的 (…)Tj 文本字面量并按 UTF-16BE 解码。
func parsePDFTextLiterals(stream []byte, out *strings.Builder) {
	i := 0
	for i < len(stream) {
		if stream[i] != '(' {
			i++
			continue
		}
		j := i + 1
		var lit []byte
		for j < len(stream) {
			if stream[j] == 0x5C {
				if j+1 < len(stream) {
					lit = append(lit, stream[j+1])
					j += 2
					continue
				}
				j++
				continue
			}
			if stream[j] == ')' {
				j++
				break
			}
			lit = append(lit, stream[j])
			j++
		}
		// UTF-16BE 解码
		for k := 0; k+1 < len(lit); k += 2 {
			code := uint16(lit[k])<<8 | uint16(lit[k+1])
			if code == 0xFEFF {
				continue
			}
			out.WriteRune(rune(code))
		}
		i = j
	}
}

// indexBytes 返回 data 中 needle 首次出现的位置，未找到返回 -1。
func indexBytes(data, needle []byte, from int) int {
	if from < 0 {
		from = 0
	}
	idx := bytes.Index(data[from:], needle)
	if idx < 0 {
		return -1
	}
	return from + idx
}

// #491：PDF 附件删除端点（学员本人；删除后 resume_file_url 置空）。
func TestResumePDFDeleteContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuPdfDel", pwd)
	now := time.Now()
	card := model.JobCard{
		UserID: stu.ID, RealName: "张三", ContactPhone: "13800000001", Region: "江苏省/苏州市",
		ExpectedRegions: model.JSONB([]byte(`["江苏省/苏州市"]`)), ResumeFileURL: "/static/uploads/resumes/a.pdf",
		Visibility: "hidden", CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("create card: %v", err)
	}
	cfg := &config.Config{
		JWTSecretKey:          "contract-test-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	studentSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	studentToken, _ := studentSess.Issue(stu.ID, stu.Account, "hrwai_user")
	// 删除附件
	rec := doWithToken(t, r, studentToken, http.MethodDelete, "/api/resume/pdf", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除附件应 200, 实际 %d %s", rec.Code, rec.Body.String())
	}
	var got model.JobCard
	if err := db.First(&got, "user_id = ?", stu.ID).Error; err != nil {
		t.Fatalf("read card: %v", err)
	}
	if got.ResumeFileURL != "" {
		t.Fatalf("删除后 resume_file_url 应置空, 实际 %q", got.ResumeFileURL)
	}
	// 未登录 401
	rec = doWithoutToken(t, r, http.MethodDelete, "/api/resume/pdf")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未登录删除应 401, 实际 %d", rec.Code)
	}
}
