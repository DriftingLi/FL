// 第①批的**声明式档位台账**（ADR-0064 决策 8）。
//
// 形状：每个端点登记「它可能产生的错误档位集合」，台账对**登记过的每一档**逐一验；
// 断言本身即双向锁 —— 打不出该档就红（登记了却测不出），未登记的端点不进台账
// （deny-by-default 登记式，沿用 ADR-0062 票4 的形状）。
//
// 为什么不是「固定两例」：本批三档并存 —— 真不存在(404) / 输入不合法(400) / 查不动(500)。
// 例数写死会在收第二族时立刻过期（决策 3 把 40 处「输入不合法」纳进来正是那一刻）。
//
// 故障注入 = 删表（与 readpath_failure_contract_test.go 同族手法）：先播种再删，
// 于是「对象取不到」与「表根本查不动」是两条分明的路径，不会互相冒充。
// 表名是本文件内的字面量常量，不接任何外部输入。
//
// 修复前的形状（本台账逐条钉住）：这 7 个端点全都只有**一格**错误面
// （errStatusAll(404) / WithSuccess(…, 404|400)）⇒ 真不存在、查不动、输入不合法三种事实
// 挤进同一个码；其中 GetGenerationTask 还把 fmt.Errorf("任务不存在: %w", err) 的驱动原文
// （record not found）直接吐进响应体。
package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gorm.io/gorm"

	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// subjectIDs 台账用到的各类主体主键。missing 是一个必然不存在的 id。
type subjectIDs struct {
	student, tutor, recruiter int
	missing                   int
}

// faceCase 一档：路径模板（含 %d 时由 by 填入实际 id）+ 方法 + 请求体 + 数据/故障布置。
type faceCase struct {
	method string
	tmpl   string
	by     func(s subjectIDs) int
	body   any
	setup  func(t *testing.T, db *gorm.DB)
}

// 选 id 的小函数：让每一档保持一行，避免多行闭包把表撑散。
func byStudent(s subjectIDs) int   { return s.student }
func byTutor(s subjectIDs) int     { return s.tutor }
func byRecruiter(s subjectIDs) int { return s.recruiter }
func byMissing(s subjectIDs) int   { return s.missing }

// endpointFaces 一个端点的档位声明：键是它**声明会出现**的状态码。
type endpointFaces struct {
	name  string
	cases map[int]faceCase
}

func seedAll(t *testing.T, db *gorm.DB) subjectIDs {
	t.Helper()
	hashed, err := service.HashPassword("seedpass123")
	if err != nil {
		t.Fatalf("哈希种子口令失败: %v", err)
	}
	stu := testutil.SeedStudent(t, db, "ledger_stu", hashed)
	tut := testutil.SeedTutor(t, db, "ledger_tutor", hashed)
	rec := testutil.SeedRecruiter(t, db, "ledger_rec", hashed)
	return subjectIDs{
		student: stu.ID, tutor: tut.TutorID, recruiter: rec.ID,
		missing: 999999,
	}
}

// dropTable 制造「查不动」。
func dropTable(name string) func(t *testing.T, db *gorm.DB) {
	return func(t *testing.T, db *gorm.DB) {
		t.Helper()
		if err := db.Exec("DROP TABLE " + name).Error; err != nil {
			t.Fatalf("注入故障（删 %s 表）失败: %v", name, err)
		}
	}
}

func pw(s string) map[string]string { return map[string]string{"password": s} }

var ledgerPathways = []endpointFaces{
	{
		name: "PUT /admin/hrwai-users/:id/status",
		cases: map[int]faceCase{
			http.StatusBadRequest:          {method: http.MethodPut, tmpl: "/api/admin/hrwai-users/not-a-number/status"},
			http.StatusNotFound:            {method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/status", by: byMissing},
			http.StatusInternalServerError: {method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/status", by: byStudent, setup: dropTable("hrwai_users")},
		},
	},
	{
		name: "PUT /admin/hrwai-users/:id/password",
		cases: map[int]faceCase{
			http.StatusBadRequest:          {method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/password", by: byStudent, body: pw("123")},
			http.StatusNotFound:            {method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/password", by: byMissing, body: pw("validpass123")},
			http.StatusInternalServerError: {method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/password", by: byStudent, body: pw("validpass123"), setup: dropTable("hrwai_users")},
		},
	},
	{
		name: "DELETE /admin/tutor/:tutor_id",
		cases: map[int]faceCase{
			http.StatusBadRequest:          {method: http.MethodDelete, tmpl: "/api/admin/tutor/not-a-number"},
			http.StatusNotFound:            {method: http.MethodDelete, tmpl: "/api/admin/tutor/%d", by: byMissing},
			http.StatusInternalServerError: {method: http.MethodDelete, tmpl: "/api/admin/tutor/%d", by: byTutor, setup: dropTable("tutor")},
		},
	},
	{
		name: "PUT /admin/tutor/:tutor_id/password",
		cases: map[int]faceCase{
			http.StatusBadRequest:          {method: http.MethodPut, tmpl: "/api/admin/tutor/%d/password", by: byTutor, body: pw("123")},
			http.StatusNotFound:            {method: http.MethodPut, tmpl: "/api/admin/tutor/%d/password", by: byMissing, body: pw("validpass123")},
			http.StatusInternalServerError: {method: http.MethodPut, tmpl: "/api/admin/tutor/%d/password", by: byTutor, body: pw("validpass123"), setup: dropTable("tutor")},
		},
	},
	{
		name: "PUT /admin/tutor/:tutor_id/status",
		cases: map[int]faceCase{
			http.StatusBadRequest:          {method: http.MethodPut, tmpl: "/api/admin/tutor/not-a-number/status"},
			http.StatusNotFound:            {method: http.MethodPut, tmpl: "/api/admin/tutor/%d/status", by: byMissing},
			http.StatusInternalServerError: {method: http.MethodPut, tmpl: "/api/admin/tutor/%d/status", by: byTutor, setup: dropTable("tutor")},
		},
	},
	{
		name: "PUT /admin/recruiters/:id/status",
		cases: map[int]faceCase{
			http.StatusBadRequest:          {method: http.MethodPut, tmpl: "/api/admin/recruiters/not-a-number/status"},
			http.StatusNotFound:            {method: http.MethodPut, tmpl: "/api/admin/recruiters/%d/status", by: byMissing},
			http.StatusInternalServerError: {method: http.MethodPut, tmpl: "/api/admin/recruiters/%d/status", by: byRecruiter, setup: dropTable("recruiter_users")},
		},
	},
	{
		name: "GET /admin/course/generate-content/:task_id",
		cases: map[int]faceCase{
			// 修复前这条被端点默认面答成 404：task_id 非数字是「输入不合法」，不是「不存在」。
			http.StatusBadRequest:          {method: http.MethodGet, tmpl: "/api/admin/course/generate-content/not-a-number"},
			http.StatusNotFound:            {method: http.MethodGet, tmpl: "/api/admin/course/generate-content/999999"},
			http.StatusInternalServerError: {method: http.MethodGet, tmpl: "/api/admin/course/generate-content/12345", setup: dropTable("async_task")},
		},
	},
}

// TestDispositionFaceLedger 正向 + 反向：登记的每一档必须打得出，且不得泄漏驱动原文。
func TestDispositionFaceLedger(t *testing.T) {
	for _, ep := range ledgerPathways {
		for status, c := range ep.cases {
			t.Run(fmt.Sprintf("%s/%d", ep.name, status), func(t *testing.T) {
				r, db, token := newAdminContractEnv(t)
				ids := seedAll(t, db)
				if c.setup != nil {
					c.setup(t, db)
				}
				path := c.tmpl
				if c.by != nil {
					path = fmt.Sprintf(c.tmpl, c.by(ids))
				}
				rec := doWithToken(t, r, token, c.method, path, c.body)
				if rec.Code != status {
					t.Fatalf("%s：声明档位 %d 打不出来，实得 %d，body=%s", ep.name, status, rec.Code, rec.Body.String())
				}
				if body := rec.Body.String(); strings.Contains(body, "record not found") {
					t.Fatalf("%s 把驱动原文吐进响应体（ADR-0064 决策 2 的泄漏族）: %s", ep.name, body)
				}
			})
		}
	}
}
