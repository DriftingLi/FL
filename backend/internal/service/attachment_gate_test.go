// 简历域的附件归属写面门禁（第十二波票 4）：JobCard 写面端到端（Create 拒绝外链、编辑未改不动）。
// 精选域的同类门禁已随域包搬去 internal/featured/attachment_gate_test.go（ADR-0070）。
package service

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestJobCardAttachmentOwnershipGate(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewJobCardService(db, nil, zap.NewNop())

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
	if err := db.Create(&model.JobCard{UserID: 102, Photos: model.JSONB([]byte(`["` + external + `"]`))}).Error; err != nil {
		t.Fatal(err)
	}
	// 编辑未改不动：重提交含存量外链 → 放行
	if _, err := svc.Upsert(102, JobCardInput{Photos: photosPtr([]string{external})}); err != nil {
		t.Fatalf("编辑未改不动应放行存量外链: %v", err)
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
