package service

import (
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// TestAuditService_DescribeAction 锁定 DescribeAction 的命名契约：
// 后缀动作短语优先 → 资源关键词匹配（按关键词长度降序）→ 兜底 verb → 无匹配兜底「verb+数据」。
// 期望值来自既定文案（/approve→通过审核、/reject→驳回申请、/password→重置密码、/status→调整状态），
// 不重算实现逻辑。
func TestAuditService_DescribeAction(t *testing.T) {
	svc := NewAuditService(nil)

	tests := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		// —— 后缀动作短语优先于资源关键词 ——
		{"approve suffix", "POST", "/api/admin/profile-reviews/12/approve", "通过审核"},
		{"reject suffix", "POST", "/api/admin/profile-reviews/12/reject", "驳回申请"},
		{"password suffix", "POST", "/api/admin/hrwai-users/9/password", "重置密码"},
		{"status suffix", "PUT", "/api/admin/courses/3/status", "调整状态"},

		// —— 资源关键词匹配（POST → 新增）——
		{"post profile-reviews", "POST", "/api/admin/profile-reviews", "新增资料审核"},
		{"post hrwai-users", "POST", "/api/admin/hrwai-users", "新增学员"},
		{"post exam-sessions", "POST", "/api/admin/exam-sessions", "新增考试场次"},
		{"post featured-content", "POST", "/api/admin/featured-content", "新增精选内容"},
		{"post ai-configs", "POST", "/api/admin/ai-configs", "新增AI配置"},
		{"post question", "POST", "/api/admin/questions", "新增题目"},
		{"post chapter", "POST", "/api/admin/chapters", "新增章节"},
		{"post course", "POST", "/api/admin/courses", "新增课程"},
		{"post grading", "POST", "/api/admin/grading", "新增阅卷"},
		{"post tutor", "POST", "/api/admin/tutors", "新增导师"},

		// —— 长度降序：forum/replies（论坛回复）优先于 forum/topics（论坛帖子）——
		{"forum replies", "POST", "/api/forum/replies", "新增论坛回复"},
		{"forum topics", "POST", "/api/forum/topics", "新增论坛帖子"},

		// —— 其他 verb ——
		{"put course", "PUT", "/api/admin/courses/3", "修改课程"},
		{"patch course", "PATCH", "/api/admin/courses/3", "修改课程"},
		{"delete question", "DELETE", "/api/admin/questions/7", "删除题目"},
		{"delete tutor", "DELETE", "/api/admin/tutors/2", "删除导师"},

		// —— 无匹配兜底：verb + 数据 ——
		{"no-match post", "POST", "/api/admin/some-other-thing", "新增数据"},
		{"no-match put", "PUT", "/api/admin/some-other-thing", "修改数据"},
		{"no-match delete", "DELETE", "/api/admin/some-other-thing", "删除数据"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := svc.DescribeAction(tt.method, tt.path); got != tt.want {
				t.Errorf("DescribeAction(%s, %s) = %q, want %q", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

// TestAuditService_DescribeAction_UnknownMethod 未知方法（GET 等非写操作）一律原样返回 method + path。
func TestAuditService_DescribeAction_UnknownMethod(t *testing.T) {
	svc := NewAuditService(nil)
	if got := svc.DescribeAction("GET", "/api/forum/topics"); got != "GET /api/forum/topics" {
		t.Errorf("GET 期望原样返回，得到 %q", got)
	}
}

// TestAuditService_List_PageSizeCap 页大小上限 100 收进 service 后：超上限回退默认值（ClampMax 语义）。
// 同时验证分页返回的 page/pageSize 钳制结果与 total 正确。
func TestAuditService_List_PageSizeCap(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	svc := NewAuditService(db)

	if err := svc.Write(model.AuditLog{ActorID: 1, ActorRole: "admin", ActorName: "管理员A", Action: "新增课程", Path: "/courses", Method: "POST", RequestID: "r1", IP: "1.1.1.1", Status: 200}); err != nil {
		t.Fatalf("写入审计记录失败: %v", err)
	}
	if err := svc.Write(model.AuditLog{ActorID: 1, ActorRole: "admin", ActorName: "管理员A", Action: "删除题目", Path: "/questions/7", Method: "DELETE", RequestID: "r2", IP: "1.1.1.1", Status: 200}); err != nil {
		t.Fatalf("写入审计记录失败: %v", err)
	}

	// pageSize 超上限（1000）→ ClampMax 回退默认（20），cap 语义生效
	items, total, page, pageSize := svc.List(1, 1000, 0, "", "")
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}
	if page != 1 || pageSize != 20 {
		t.Fatalf("page/pageSize = %d/%d, want 1/20 (ClampMax 回退默认)", page, pageSize)
	}
	if len(items) != 2 {
		t.Fatalf("本页应返回 2 条, got %d", len(items))
	}

	// page/pageSize 非正 → 默认钳制
	_, _, page2, pageSize2 := svc.List(0, 0, 0, "", "")
	if page2 != 1 || pageSize2 != 20 {
		t.Fatalf("默认分页 = %d/%d, want 1/20", page2, pageSize2)
	}

	// 合法 pageSize（2）不被钳制，且 actor 过滤生效
	_, total3, page3, pageSize3 := svc.List(1, 2, 999, "", "")
	if total3 != 0 {
		t.Fatalf("按 actor_id=999 过滤后 total = %d, want 0", total3)
	}
	if page3 != 1 || pageSize3 != 2 {
		t.Fatalf("合法 pageSize 2 被误钳: page/pageSize = %d/%d", page3, pageSize3)
	}
}
