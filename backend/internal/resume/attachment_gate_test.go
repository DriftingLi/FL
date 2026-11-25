// 简历域的附件归属写面门禁（第十二波票 4）：JobCard 写面端到端（Create 拒绝外链、编辑未改不动）。
// 精选域的同类门禁已随域包搬去 internal/featured/attachment_gate_test.go（ADR-0070）；
// 本文件随简历域搬进 internal/resume（P2 波 4e：JobCard 写面的实现与举证住同一个包）。
package resume

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func resumeRawMessage(raw string) *json.RawMessage {
	rm := json.RawMessage(raw)
	return &rm
}

func resumePtr[T any](v T) *T {
	return &v
}

func TestJobCardAttachmentOwnershipGate(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())

	const own = "/static/uploads/resumes/images/p_1.webp"
	const external = "https://evil.example.com/p.jpg"
	photosPtr := func(v any) *json.RawMessage {
		b, _ := json.Marshal(v)
		rm := json.RawMessage(b)
		return &rm
	}

	// 创建面：外链工作照 → 拒绝
	if _, err := svc.Upsert(101, JobCardInput{Photos: photosPtr([]string{external})}); err == nil {
		t.Fatal("创建面外链工作照应报错")
	}
	// 创建面：本站工作照 → 放行
	if _, err := svc.Upsert(101, JobCardInput{Photos: photosPtr([]string{own})}); err != nil {
		t.Fatalf("创建面本站工作照应放行: %v", err)
	}
	// 存量外链直接种库（门禁开启前的历史数据）
	if err := db.Create(&model.JobCard{
		UserID:               102,
		Photos:               model.JSONB([]byte(`["` + external + `"]`)),
		ResumeCertifications: model.JSONB([]byte(`[{"cert_no":"旧证书","image_urls":["` + external + `"]}]`)),
	}).Error; err != nil {
		t.Fatal(err)
	}
	// 编辑未改不动：重提交含存量外链 → 放行
	if _, err := svc.Upsert(102, JobCardInput{
		Photos:               photosPtr([]string{external}),
		ResumeCertifications: photosPtr([]any{map[string]any{"cert_no": "旧证书", "image_urls": []string{external}}}),
	}); err != nil {
		t.Fatalf("编辑未改不动应放行存量照片与证书外链: %v", err)
	}
	// 编辑：在存量外链之外新增一条外链 → 拒绝
	if _, err := svc.Upsert(102, JobCardInput{Photos: photosPtr([]string{external, "https://evil2/x.jpg"})}); err == nil {
		t.Fatal("新增外链工作照应报错")
	}
	// 证书原图同判据：新增外链 image_urls → 拒绝
	certs := photosPtr([]any{map[string]any{"cert_no": "C1", "image_urls": []string{external}}})
	if _, err := svc.Upsert(101, JobCardInput{ResumeCertifications: certs}); err == nil {
		t.Fatal("证书新增外链原图应报错")
	}
	// 证书本站原图 → 放行
	certsOwn := photosPtr([]any{map[string]any{"cert_no": "C1", "image_urls": []string{own}}})
	if _, err := svc.Upsert(101, JobCardInput{ResumeCertifications: certsOwn}); err != nil {
		t.Fatalf("证书本站原图应放行: %v", err)
	}
	// 畸形载荷 → 拒（不可解析即无法过门禁，静默跳过等于旁路）
	malformed := func(s string) *json.RawMessage { rm := json.RawMessage(s); return &rm }
	if _, err := svc.Upsert(103, JobCardInput{Photos: malformed(`{"url":"` + external + `"}`)}); err == nil {
		t.Fatal("工作照非字符串数组应报错")
	}
	if _, err := svc.Upsert(103, JobCardInput{ResumeCertifications: malformed(`"not-rows"`)}); err == nil {
		t.Fatal("证书信息形态不符应报错")
	}
}

func TestJobCardValidationErrorPriority(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	sevenPhotos := make([]string, 7)
	for i := range sevenPhotos {
		sevenPhotos[i] = "/static/uploads/resumes/images/p.jpg"
	}
	photoJSON, _ := json.Marshal(sevenPhotos)
	allInvalid := func() JobCardInput {
		return JobCardInput{
			SelfIntro:            resumePtr(strings.Repeat("字", 1001)),
			ExpectedPositionID:   resumePtr(99999),
			SalaryMin:            resumePtr(-1),
			SalaryMax:            resumePtr(-1),
			ExperienceYears:      resumePtr(-1),
			ResumeCertifications: resumeRawMessage(`[{"credential_id":99999}]`),
			Photos:               resumeRawMessage(string(photoJSON)),
			ExpectedRegions:      resumeRawMessage(`{"region":"苏州"}`),
			ResumeExperiences:    resumeRawMessage(`{"company":"A"}`),
			ResumeFileURL:        resumePtr("resume.jpg"),
			Visibility:           resumePtr("public"),
		}
	}
	tests := []struct {
		name string
		fix  func(*JobCardInput)
		want string
	}{
		{"自我介绍", func(in *JobCardInput) {}, "自我介绍不能超过 1000 字"},
		{"岗位", func(in *JobCardInput) { in.SelfIntro = nil }, "期望岗位不存在"},
		{"薪资", func(in *JobCardInput) { in.SelfIntro, in.ExpectedPositionID = nil, nil }, "期望薪资不能为负"},
		{"最高薪资", func(in *JobCardInput) { in.SelfIntro, in.ExpectedPositionID, in.SalaryMin = nil, nil, nil }, "期望薪资不能为负"},
		{"薪资上下限", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax = nil, nil, resumePtr(20), resumePtr(10)
		}, "最低薪资不能高于最高薪资"},
		{"经历", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax = nil, nil, nil, nil
		}, "工作年限不能为负"},
		{"证书", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax, in.ExperienceYears = nil, nil, nil, nil, nil
		}, "第 1 条持证的证件不存在"},
		{"照片", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax, in.ExperienceYears = nil, nil, nil, nil, nil
			in.ResumeCertifications = resumeRawMessage(`[{"credential_id":0}]`)
		}, "工作照最多 6 张"},
		{"地区", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax, in.ExperienceYears = nil, nil, nil, nil, nil
			in.ResumeCertifications, in.Photos = nil, nil
		}, "意向地区格式错误"},
		{"工作经历", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax, in.ExperienceYears = nil, nil, nil, nil, nil
			in.ResumeCertifications, in.Photos, in.ExpectedRegions = nil, nil, nil
		}, "工作经历格式错误"},
		{"PDF", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax, in.ExperienceYears = nil, nil, nil, nil, nil
			in.ResumeCertifications, in.Photos, in.ExpectedRegions, in.ResumeExperiences = nil, nil, nil, nil
		}, "简历附件仅支持 PDF"},
		{"visibility", func(in *JobCardInput) {
			in.SelfIntro, in.ExpectedPositionID, in.SalaryMin, in.SalaryMax, in.ExperienceYears = nil, nil, nil, nil, nil
			in.ResumeCertifications, in.Photos, in.ExpectedRegions, in.ResumeExperiences, in.ResumeFileURL = nil, nil, nil, nil, nil
		}, "visibility 仅支持 hidden / open"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := allInvalid()
			tt.fix(&in)
			_, err := svc.validateInput(in)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("错误 = %v, want %q", err, tt.want)
			}
		})
	}

	malformedPhoto := resumeRawMessage(`{"url":"x"}`)
	in := JobCardInput{Photos: malformedPhoto, ResumeFileURL: resumePtr("resume.jpg")}
	if _, err := svc.validateInput(in); err == nil || err.Error() != "简历附件仅支持 PDF" {
		t.Fatalf("图片格式错误不得抢在 PDF 前: %v", err)
	}
	in.ResumeFileURL = nil
	in.Visibility = resumePtr("public")
	if _, err := svc.validateInput(in); err == nil || err.Error() != "visibility 仅支持 hidden / open" {
		t.Fatalf("图片格式错误不得抢在 visibility 前: %v", err)
	}
	in.Visibility = nil
	if _, err := svc.Upsert(2001, in); err == nil || err.Error() != "工作照格式无效（需字符串数组）" {
		t.Fatalf("图片格式错误应由 ownership 返回: %v", err)
	}
}

func TestJobCardCertificationBatchValidation(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	first := model.Credential{Code: "batch_first", Name: "证书一", Category: "special_operation", Status: 1}
	second := model.Credential{Code: "batch_second", Name: "证书二", Category: "special_operation", Status: 1}
	if err := db.Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatal(err)
	}

	type credentialQuery struct {
		sql  string
		vars int
	}
	var credentialQueries []credentialQuery
	if err := db.Callback().Query().After("gorm:query").Register("resume:certificate-query-count", func(tx *gorm.DB) {
		if tx.Statement.Table == "credential" {
			credentialQueries = append(credentialQueries, credentialQuery{sql: tx.Statement.SQL.String(), vars: len(tx.Statement.Vars)})
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove("resume:certificate-query-count") })

	rows := fmt.Sprintf(`[
		{"credential_id":%d}, {"credential_id":%d}, {"credential_id":%d},
		{"credential_id":null}, {"credential_id":0}, {"credential_id":-1}
	]`, first.ID, first.ID, second.ID)
	parsed, err := svc.validateInput(JobCardInput{ResumeCertifications: resumeRawMessage(rows)})
	if err != nil {
		t.Fatalf("重复有效证书 ID 应放行: %v", err)
	}
	if len(parsed.certifications) != 6 {
		t.Fatalf("typed 证书行数 = %d, want 6", len(parsed.certifications))
	}
	if len(credentialQueries) != 1 {
		t.Fatalf("证书查询数 = %d, want 1; SQL=%v", len(credentialQueries), credentialQueries)
	}
	query := credentialQueries[0]
	if !strings.Contains(query.sql, "IN (?,?)") || query.vars != 2 {
		t.Fatalf("证书查询不是两个去重 ID 的单次 IN: %+v", query)
	}

	credentialQueries = nil
	_, err = svc.validateInput(JobCardInput{ResumeCertifications: resumeRawMessage(`[
		{"credential_id":null}, {"credential_id":98765}, {"credential_id":98765}, {"credential_id":98766}
	]`)})
	if err == nil || err.Error() != "第 2 条持证的证件不存在" {
		t.Fatalf("首个重复缺失 ID 应报原顺序第 2 条: %v", err)
	}
	if len(credentialQueries) != 1 || !strings.Contains(credentialQueries[0].sql, "IN (?,?)") || credentialQueries[0].vars != 2 {
		t.Fatalf("缺失证书也应单次去重 IN 查询: %v", credentialQueries)
	}

	credentialQueries = nil
	if _, err := svc.validateInput(JobCardInput{ResumeCertifications: resumeRawMessage(`[
		{"credential_id":null}, {"credential_id":0}, {"credential_id":-1}
	]`)}); err != nil {
		t.Fatalf("nil/非正证书 ID 应跳过: %v", err)
	}
	if len(credentialQueries) != 0 {
		t.Fatalf("无可查询证书 ID 时不应查询数据库: %v", credentialQueries)
	}
}

func TestJobCardParsedJSONReuseAndNormalization(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	cert := model.Credential{Code: "reuse_cert", Name: "复用证书", Category: "special_operation", Status: 1}
	if err := db.Create(&cert).Error; err != nil {
		t.Fatal(err)
	}
	in := JobCardInput{
		ExpectedRegions:      resumeRawMessage(`["苏州"]`),
		ResumeExperiences:    resumeRawMessage(`[{"company":"A"}]`),
		ResumeCertifications: resumeRawMessage(fmt.Sprintf(`[{"credential_id":%d,"image_urls":["/static/uploads/resumes/images/c.webp"]}]`, cert.ID)),
		Photos:               resumeRawMessage(`["/static/uploads/resumes/images/p.webp"]`),
	}
	parsed, err := svc.validateInput(in)
	if err != nil {
		t.Fatal(err)
	}
	in.ExpectedRegions = resumeRawMessage(`broken`)
	in.ResumeExperiences = resumeRawMessage(`broken`)
	in.ResumeCertifications = resumeRawMessage(`broken`)
	in.Photos = resumeRawMessage(`broken`)
	card := model.JobCard{UserID: 2101}
	if err := validateAttachmentOwnership(&card, parsed); err != nil {
		t.Fatalf("ownership 不应重新解析输入: %v", err)
	}
	applyInput(&card, in, parsed)
	if string(card.ExpectedRegions) != `["苏州"]` || string(card.ResumeExperiences) != `[{"company":"A"}]` ||
		string(card.ResumeCertifications) != fmt.Sprintf(`[{"credential_id":%d,"image_urls":["/static/uploads/resumes/images/c.webp"]}]`, cert.ID) ||
		string(card.Photos) != `["/static/uploads/resumes/images/p.webp"]` {
		t.Fatalf("apply 未复用 typed 解析结果: %+v", card)
	}

	for _, raw := range []string{"", "null", "[]", " null ", " [] "} {
		want := raw
		if raw == "" || raw == "null" || raw == "[]" {
			want = "[]"
		}
		t.Run("normalize_"+raw, func(t *testing.T) {
			card := model.JobCard{
				ExpectedRegions:      model.JSONB(`["旧"]`),
				ResumeExperiences:    model.JSONB(`[{"company":"旧"}]`),
				ResumeCertifications: model.JSONB(`[{"cert_no":"旧"}]`),
				Photos:               model.JSONB(`["旧"]`),
			}
			parsed, err := svc.validateInput(JobCardInput{
				ExpectedRegions:      resumeRawMessage(raw),
				ResumeExperiences:    resumeRawMessage(raw),
				ResumeCertifications: resumeRawMessage(raw),
				Photos:               resumeRawMessage(raw),
			})
			if err != nil {
				t.Fatal(err)
			}
			applyInput(&card, JobCardInput{
				ExpectedRegions:      resumeRawMessage(raw),
				ResumeExperiences:    resumeRawMessage(raw),
				ResumeCertifications: resumeRawMessage(raw),
				Photos:               resumeRawMessage(raw),
			}, parsed)
			for name, got := range map[string]model.JSONB{
				"expected_regions": card.ExpectedRegions, "resume_experiences": card.ResumeExperiences,
				"resume_certifications": card.ResumeCertifications, "photos": card.Photos,
			} {
				if string(got) != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}

	card = model.JobCard{
		ExpectedRegions:      model.JSONB(`["旧"]`),
		ResumeExperiences:    model.JSONB(`[{"company":"旧"}]`),
		ResumeCertifications: model.JSONB(`[{"cert_no":"旧"}]`),
		Photos:               model.JSONB(`["旧"]`),
	}
	applyInput(&card, JobCardInput{}, &parsedResumeInput{})
	if string(card.ExpectedRegions) != `["旧"]` || string(card.ResumeExperiences) != `[{"company":"旧"}]` ||
		string(card.ResumeCertifications) != `[{"cert_no":"旧"}]` || string(card.Photos) != `["旧"]` {
		t.Fatal("partial update 覆盖了未传 JSONB 字段")
	}
}

func TestJobCardPartialUpdateAndVisibilityDifference(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	original := model.JobCard{
		UserID: 2201, RealName: "旧姓名", ContactPhone: "13800000000", Wechat: "old_wechat", Region: "旧地区",
		ExpectedPositionID: resumePtr(88), ExpectedPositionExtra: "旧岗位", ExpectedRegions: model.JSONB(`["旧地区"]`),
		SalaryMin: resumePtr(1), SalaryMax: resumePtr(2), SalaryNegotiable: true, AvailableIn: "旧到岗", JobNature: "旧性质",
		ExperienceYears: 3, SelfIntro: "旧自我介绍", ResumeExperiences: model.JSONB(`[{"company":"旧公司"}]`),
		ResumeCertifications: model.JSONB(`[{"cert_no":"旧证书"}]`), ResumeFileURL: "/static/uploads/resumes/old.pdf",
		Photos: model.JSONB(`["/static/uploads/resumes/images/old.webp"]`), Visibility: "open",
	}
	if err := db.Create(&original).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Upsert(original.UserID, JobCardInput{RealName: resumePtr(" 新姓名 ")}); err != nil {
		t.Fatal(err)
	}
	var got model.JobCard
	if err := db.First(&got, "user_id = ?", original.UserID).Error; err != nil {
		t.Fatal(err)
	}
	original.RealName = "新姓名"
	original.CreatedAt = got.CreatedAt
	original.UpdatedAt = got.UpdatedAt
	if !reflect.DeepEqual(got, original) {
		t.Fatalf("partial update 改动了未传字段:\ngot  %+v\nwant %+v", got, original)
	}

	emptyVisibility := resumePtr(" \t ")
	if _, err := svc.validateInput(JobCardInput{Visibility: emptyVisibility}); err != nil {
		t.Fatalf("validateInput 应允许空 visibility: %v", err)
	}
	if _, err := svc.Upsert(2202, JobCardInput{Visibility: emptyVisibility}); err == nil || err.Error() != "visibility 仅支持 hidden / open" {
		t.Fatalf("Upsert 应拒绝空 visibility: %v", err)
	}
	if _, err := svc.Upsert(original.UserID, JobCardInput{Visibility: emptyVisibility}); err == nil || err.Error() != "visibility 仅支持 hidden / open" {
		t.Fatalf("Upsert 编辑面也应拒绝空 visibility: %v", err)
	}
}

func TestJobCardMalformedJSONErrors(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewService(db, nil, zap.NewNop())
	tests := []struct {
		name string
		in   JobCardInput
		want string
	}{
		{"证书损坏", JobCardInput{ResumeCertifications: resumeRawMessage(`[{`)}, "持证信息格式错误"},
		{"地区损坏", JobCardInput{ExpectedRegions: resumeRawMessage(`[{`)}, "意向地区格式错误"},
		{"经历损坏", JobCardInput{ResumeExperiences: resumeRawMessage(`[{`)}, "工作经历格式错误"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.validateInput(tt.in); err == nil || err.Error() != tt.want {
				t.Fatalf("错误 = %v, want %q", err, tt.want)
			}
		})
	}
	if _, err := svc.validateInput(JobCardInput{Photos: resumeRawMessage(`[{`)}); err != nil {
		t.Fatalf("照片损坏只记录到 ownership: %v", err)
	}
}
