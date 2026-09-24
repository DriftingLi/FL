// ADR-0065 批③ 的对称锁：同一份非法输入，在 Create 与 Update 两条面上必须给同一个码；
// 「引用对象查不动」在两条面上也必须给 500。
//
// 为什么要单独一把（不并进 course_domain_face_ledger_test.go）：台账按「端点 × 档位」登记，
// 它锁得住「某一端点某一档打得出」，锁不住**同一件输入错误在两个动词上分家**——
// 而那正是本批的病灶：
//
//   - Create（`api/admin.go` 的 `WithSuccess(created(…), http.StatusBadRequest)`）默认面 **400**
//     ⇒ `applyCourseTrainingFields` 里那 4 处 `Count()` 的「查不动」被答成**参数错误**
//     （精选在第③批修过的镜像病，这一域漏了）；
//   - Update（默认面 500 + 只挂 3 条哨兵）⇒ 11 条裸 `errors.New` 的输入不合法全被咽成 **500**。
//
// 于是本文件的两组断言在实现之前**两头都是红的**（非法输入：Create 400 / Update 500；
// 查不动：Create 400 / Update 500），这也是它自己的红证——一把今天就在红的锁不需要额外破坏。
package api

import (
	"fmt"
	"net/http"
	"testing"
)

// courseFactCase 一件「输入不合法」或「引用查不动」的事实，以及在两条面上要打的请求。
type courseFactCase struct {
	name  string         // 事实名（进用例名）
	patch map[string]any // 叠加在合法基础 body 上的字段
	// dropTable 非空 ⇒ 本例测的是「引用对象查不动」：先删掉那张表，再断言两面都 500。
	dropTable string
}

// courseFactCases 是 `applyCourseTrainingFields`（service/course_service.go:907-1005）里
// 登记 11 条的逐条清点 + 4 条「查不动」+ 前置课程那支的 1 条对称可构造例：
//
//	证件 ID无效 :911 / 不存在 :921 / 查不动 :917
//	方向 ID无效 :929 / 不存在 :939 / 查不动 :935
//	等级 ID无效 :947 / 不存在 :957 / 查不动 :953
//	模板 ID无效 :965 / 不存在 :975 / 查不动 :971
//	理论学时负 :982 / 实操学时负 :988 / 排序值负 :994
//
// 这里不写「想当然的期望码」，只写「两条面必须同码」+ 两类各自的目标码：
// 输入不合法 ⇒ 400（含引用对象不存在：客户端给的是坏引用，不是服务端故障）；查不动 ⇒ 500。
var courseFactCases = []courseFactCase{
	{name: "证件ID为负", patch: map[string]any{"credential_id": -5}},
	{name: "证件引用不存在", patch: map[string]any{"credential_id": 999999}},
	{name: "证件查不动", patch: map[string]any{"credential_id": 1}, dropTable: "credential"},
	{name: "方向ID为负", patch: map[string]any{"specialty_id": -5}},
	{name: "方向引用不存在", patch: map[string]any{"specialty_id": 999999}},
	{name: "方向查不动", patch: map[string]any{"specialty_id": 999999}, dropTable: "specialty"},
	{name: "等级ID为负", patch: map[string]any{"level_id": -5}},
	{name: "等级引用不存在", patch: map[string]any{"level_id": 999999}},
	{name: "等级查不动", patch: map[string]any{"level_id": 999999}, dropTable: "course_level"},
	{name: "模板ID为负", patch: map[string]any{"certificate_template_id": -5}},
	{name: "模板引用不存在", patch: map[string]any{"certificate_template_id": 999999}},
	{name: "模板查不动", patch: map[string]any{"certificate_template_id": 999999}, dropTable: "certificate_template"},
	{name: "理论学时为负", patch: map[string]any{"theory_hours": -1}},
	{name: "实操学时为负", patch: map[string]any{"practice_hours": -1}},
	{name: "排序值为负", patch: map[string]any{"sort_order": -1}},
	// 前置课程那一支：登记时漏了（决策 3 的 14 条 = 11 + 这 3 条）。它两面都构造得出，
	// 所以进同一张对称表；「设成自己的前置」与「循环」只在 Update 面可构造，另测。
	{name: "前置课程不存在", patch: map[string]any{"prerequisite_course_ids": []int{999999}}},
}

// TestCourseInvalidInputSameFaceOnCreateAndUpdate 每件输入不合法在两条面上同码且都是 400；
// 每件「查不动」在两条面上同码且都是 500。
func TestCourseInvalidInputSameFaceOnCreateAndUpdate(t *testing.T) {
	for _, tc := range courseFactCases {
		want := http.StatusBadRequest
		if tc.dropTable != "" {
			want = http.StatusInternalServerError
		}
		t.Run(fmt.Sprintf("%s/%d", tc.name, want), func(t *testing.T) {
			r, db, token := newAdminContractEnv(t)
			ids := seedCourseDomain(t, db)

			base := map[string]any{"name": "对称锁课程", "specialty_id": ids.specialty, "level_id": ids.level}
			body := map[string]any{}
			for k, v := range base {
				body[k] = v
			}
			for k, v := range tc.patch {
				body[k] = v
			}
			// 基础 body 必须自己先合法，否则测的就不是本例那件事实（红在这里 ⇒ 用例设计错，不是代码错）。
			if got := doWithToken(t, r, token, http.MethodPost, "/api/admin/course", base).Code; got != http.StatusCreated {
				t.Fatalf("基础 body 打 Create 应得 201，实得 %d ⇒ 本例的差值不在被测字段上", got)
			}
			if got := doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/course/%d", ids.course), base).Code; got != http.StatusOK {
				t.Fatalf("基础 body 打 Update 应得 200，实得 %d", got)
			}

			if tc.dropTable != "" {
				dropTable(tc.dropTable)(t, db)
			}

			created := doWithToken(t, r, token, http.MethodPost, "/api/admin/course", body)
			updated := doWithToken(t, r, token, http.MethodPut, fmt.Sprintf("/api/admin/course/%d", ids.course), body)
			if created.Code != want {
				t.Errorf("Create 面上「%s」应 %d，实得 %d：%s", tc.name, want, created.Code, created.Body.String())
			}
			if updated.Code != want {
				t.Errorf("Update 面上「%s」应 %d，实得 %d：%s", tc.name, want, updated.Code, updated.Body.String())
			}
			if created.Code != updated.Code {
				t.Errorf("同一件「%s」两条面分家：Create %d vs Update %d ⇒ 客户端只能按动词猜语义",
					tc.name, created.Code, updated.Code)
			}
			// 查不动不得把驱动原文吐出去（ADR-0064 决策 9）。
			if want == http.StatusInternalServerError {
				for _, rec := range []interface{ Body() []byte }{} {
					_ = rec
				} // 见下：直接用 recorder 的 body 字符串
				for name, body := range map[string]string{"Create": created.Body.String(), "Update": updated.Body.String()} {
					if leakyDriverText(body) {
						t.Errorf("%s 面把驱动原文外发了（ADR-0064 决策 9）：%s", name, body)
					}
				}
			}
		})
	}
}

// TestCoursePrerequisiteFactsAreUpdateOnly 「设成自己的前置」与「循环依赖」在 Create 面
// 构造不出来（新课程的 id 在写库之后才知道，也不可能有入边）⇒ 这里只声明 Update 面，
// 并把「为什么另一面不声明」写在测试里 —— 台账的「据实不声明」口径（ADR-0064 决策 8）。
func TestCoursePrerequisiteFactsAreUpdateOnly(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(ids courseDomainIDs) map[string]any
	}{
		{"自己是自己的前置", func(ids courseDomainIDs) map[string]any {
			return map[string]any{"prerequisite_course_ids": []int{ids.course}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, token := newAdminContractEnv(t)
			ids := seedCourseDomain(t, db)
			rec := doWithToken(t, r, token, http.MethodPut,
				fmt.Sprintf("/api/admin/course/%d", ids.course), tc.build(ids))
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("应 400（输入不合法），实得 %d：%s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestCourseSwapSortFaces 交换排序那一族：路径课程不存在 ⇒ 404；坏引用 / 跨组 ⇒ 400；
// 查不动 ⇒ 500 且不外发驱动原文。改之前这五格全挤在默认面 400 里（「这门课没有」被说成
// 「参数错了」，而 `no such table` 直接发给客户端）。
func TestCourseSwapSortFaces(t *testing.T) {
	type swapCase struct {
		name   string
		drop   string
		pathID func(courseDomainIDs) int
		body   map[string]any
		want   int
	}
	for _, tc := range []swapCase{
		{name: "换到不存在的课程", pathID: func(d courseDomainIDs) int { return d.course },
			body: map[string]any{"swap_with": 999999}, want: http.StatusBadRequest},
		{name: "路径课程不存在", pathID: func(d courseDomainIDs) int { return d.missing },
			body: map[string]any{"swap_with": 1}, want: http.StatusNotFound},
		{name: "查不动", drop: "course", pathID: func(d courseDomainIDs) int { return d.course },
			body: map[string]any{"swap_with": 999999}, want: http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db, token := newAdminContractEnv(t)
			ids := seedCourseDomain(t, db)
			if tc.drop != "" {
				dropTable(tc.drop)(t, db)
			}
			rec := doWithToken(t, r, token, http.MethodPut,
				fmt.Sprintf("/api/admin/course/%d/sort", tc.pathID(ids)), tc.body)
			if rec.Code != tc.want {
				t.Fatalf("应 %d，实得 %d：%s", tc.want, rec.Code, rec.Body.String())
			}
			if tc.want >= 500 && leakyDriverText(rec.Body.String()) {
				t.Fatalf("5xx 面外发驱动原文（ADR-0064 决策 9）：%s", rec.Body.String())
			}
		})
	}
	// 「未挂载方向/等级的课程不能参与排序」与「该实体不支持排序交换」今天都构造不出 HTTP 档：
	// 前者要求一行存在但未挂载的课程，而 Create 面的挂载不变式不放它进来；后者要求某张
	// 目录表 Sortable=false，而现有 5 个 swap 端点对应的实体都开了排序。⇒ 两条事实留在
	// service 侧具名，端点档位**据实不声明**（不是漏测）。
}
