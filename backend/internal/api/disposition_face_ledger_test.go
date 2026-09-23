// 第①批的**声明式档位台账**（ADR-0064 决策 8）。
//
// 形状：每个端点登记「它可能产生的错误档位」，台账对**登记过的每一档**逐一验。
// 锁住的方向（如实说明，不夸大）：
//   - **反向锁**：登记了某一档却打不出来 ⇒ 红。这是本文件真正提供的方向。
//   - 正向的「未登记即红」需要枚举路由再比对登记表，与第④批的表态锁同属机器锁建设，
//     本批不预先造半个（决策 8 明确表态锁最后建）。在那之前，登记面靠批次边界 + 评审。
//
// 为什么不是「固定两例」：本批三档并存 —— 真不存在(404) / 输入不合法(400) / 查不动(500)，
// 且同一个码可有多种成因（400 既可能是 id 非数字，也可能是 id 为负数）。所以档位用切片而非
// map[int]：按码建表会把第二种成因挤掉。
//
// 故障注入 = 删表（与 readpath_failure_contract_test.go 同族手法）：先播种再删，
// 于是「对象取不到」与「表根本查不动」是两条分明的路径，不会互相冒充。
// 表名是本文件内的字面量常量，不接任何外部输入。
//
// 修复前的形状（本台账逐条钉住）：这 7 个端点全都只有**一格**错误面
// （errStatusAll(404) / WithSuccess(…, 404|400)）⇒ 真不存在、查不动、输入不合法三种事实
// 挤进同一个码；其中 GetGenerationTask 还把 fmt.Errorf("任务不存在: %w") 的驱动原文
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

// faceCase 一档要打的请求：路径模板（含 %d 时由 by 填入实际 id）+ 方法 + 请求体 + 数据布置。
type faceCase struct {
	method string
	tmpl   string
	by     func(s subjectIDs) int
	body   any
	setup  func(t *testing.T, db *gorm.DB)
}

// declaredFace 一档声明：want 是它应当落的状态码，req 是打这一档要发的请求。
type declaredFace struct {
	want int
	req  faceCase
}

// endpointFaces 一个端点的档位集合。
type endpointFaces struct {
	name  string
	cases []declaredFace
}

func byStudent(s subjectIDs) int   { return s.student }
func byTutor(s subjectIDs) int     { return s.tutor }
func byRecruiter(s subjectIDs) int { return s.recruiter }
func byMissing(s subjectIDs) int   { return s.missing }

// 同一条「输入不合法」的两种成因：非数字、负数。后者是本次分档时差点踩出的回归——
// 默认面从 400/404 收窄到 500 之后，service 里那句裸「用户 ID 非法」会被答成 500。
const badID = "/not-a-number"
const negID = "/-5"

func seedAll(t *testing.T, db *gorm.DB) subjectIDs {
	t.Helper()
	hashed, err := service.HashPassword("seedpass123")
	if err != nil {
		t.Fatalf("哈希种子口令失败: %v", err)
	}
	stu := testutil.SeedStudent(t, db, "ledger_stu", hashed)
	tut := testutil.SeedTutor(t, db, "ledger_tutor", hashed)
	rec := testutil.SeedRecruiter(t, db, "ledger_rec", hashed)
	return subjectIDs{student: stu.ID, tutor: tut.TutorID, recruiter: rec.ID, missing: 999999}
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

func passwordBody(s string) map[string]string { return map[string]string{"password": s} }

var ledgerPathways = []endpointFaces{
	{
		name: "PUT /admin/hrwai-users/:id/status",
		cases: []declaredFace{
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users" + badID + "/status"}},
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users" + negID + "/status"}},
			{want: http.StatusNotFound, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/status", by: byMissing}},
			{want: http.StatusInternalServerError, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/status", by: byStudent, setup: dropTable("hrwai_users")}},
		},
	},
	{
		name: "PUT /admin/hrwai-users/:id/password",
		cases: []declaredFace{
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/password", by: byStudent, body: passwordBody("123")}},
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users" + negID + "/password", body: passwordBody("validpass123")}},
			{want: http.StatusNotFound, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/password", by: byMissing, body: passwordBody("validpass123")}},
			{want: http.StatusInternalServerError, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/hrwai-users/%d/password", by: byStudent, body: passwordBody("validpass123"), setup: dropTable("hrwai_users")}},
		},
	},
	{
		name: "DELETE /admin/tutor/:tutor_id",
		cases: []declaredFace{
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodDelete, tmpl: "/api/admin/tutor" + badID}},
			{want: http.StatusNotFound, req: faceCase{method: http.MethodDelete, tmpl: "/api/admin/tutor/%d", by: byMissing}},
			{want: http.StatusInternalServerError, req: faceCase{method: http.MethodDelete, tmpl: "/api/admin/tutor/%d", by: byTutor, setup: dropTable("tutor")}},
		},
	},
	{
		name: "PUT /admin/tutor/:tutor_id/password",
		cases: []declaredFace{
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/tutor/%d/password", by: byTutor, body: passwordBody("123")}},
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/tutor" + negID + "/password", body: passwordBody("validpass123")}},
			{want: http.StatusNotFound, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/tutor/%d/password", by: byMissing, body: passwordBody("validpass123")}},
			{want: http.StatusInternalServerError, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/tutor/%d/password", by: byTutor, body: passwordBody("validpass123"), setup: dropTable("tutor")}},
		},
	},
	{
		name: "PUT /admin/tutor/:tutor_id/status",
		cases: []declaredFace{
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/tutor" + badID + "/status"}},
			{want: http.StatusNotFound, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/tutor/%d/status", by: byMissing}},
			{want: http.StatusInternalServerError, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/tutor/%d/status", by: byTutor, setup: dropTable("tutor")}},
		},
	},
	{
		name: "PUT /admin/recruiters/:id/status",
		cases: []declaredFace{
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/recruiters" + badID + "/status"}},
			{want: http.StatusNotFound, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/recruiters/%d/status", by: byMissing}},
			{want: http.StatusInternalServerError, req: faceCase{method: http.MethodPut, tmpl: "/api/admin/recruiters/%d/status", by: byRecruiter, setup: dropTable("recruiter_users")}},
		},
	},
	{
		name: "GET /admin/course/generate-content/:task_id",
		cases: []declaredFace{
			// 修复前被端点默认面答成 404：非数字 task_id 是「输入不合法」，不是「不存在」。
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodGet, tmpl: "/api/admin/course/generate-content" + badID}},
			// 严格解析的那一半：fmt.Sscanf("%d") 会把 "12abc" 静默截成 12 当成合法查询。
			{want: http.StatusBadRequest, req: faceCase{method: http.MethodGet, tmpl: "/api/admin/course/generate-content/12abc"}},
			{want: http.StatusNotFound, req: faceCase{method: http.MethodGet, tmpl: "/api/admin/course/generate-content/999999"}},
			{want: http.StatusInternalServerError, req: faceCase{method: http.MethodGet, tmpl: "/api/admin/course/generate-content/12345", setup: dropTable("async_task")}},
		},
	},
}

// TestDispositionFaceLedger 逐档验：登记的档必须打得出，且任何一档都不得泄漏驱动原文。
func TestDispositionFaceLedger(t *testing.T) {
	for _, ep := range ledgerPathways {
		for i, declared := range ep.cases {
			c := declared.req
			t.Run(fmt.Sprintf("%s/%d#%d", ep.name, declared.want, i), func(t *testing.T) {
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
				if rec.Code != declared.want {
					t.Fatalf("%s：声明档位 %d 打不出来，实得 %d，body=%s", ep.name, declared.want, rec.Code, rec.Body.String())
				}
				if body := rec.Body.String(); strings.Contains(body, "record not found") {
					t.Fatalf("%s 把驱动原文吐进响应体（ADR-0064 决策 2 的泄漏族）: %s", ep.name, body)
				}
			})
		}
	}
}
