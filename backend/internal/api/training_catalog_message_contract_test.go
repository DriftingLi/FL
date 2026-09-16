// ADR-0053 §1 契约测试：目录域 handler 收窄为「adapters + 声明」后，
// **状态码 + 成功文案 + data 形状**必须逐字不变。
//
// 为什么单独一个文件：既有四个契约测试（TestCatalogContract_*）复用的是**零漂移**证据，
// 但它们把信封的 message 读出来就丢掉了（`_, _, data := unpackData(...)`）——
// 也就是说这次重构把成功文案搬进适配器时，改错文案不会有任何测试报红。
// 本文件把「成功文案」补齐，并给两个新登记端点（/admin/specialties、/admin/levels）
// 与四个岗位端点补第一个契约用例。
//
// 两个新端点在 2026-09-16 之前完全不在契约里（无接口注解 → 不在生成的 API 文档里），
// 所以它们此前连一个契约用例都没有。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// assertEnvelope 断言 HTTP 状态、信封 code 与**成功文案**（本文件的存在理由）。
// wantMsg 传空串表示「不断言文案」（列表端点的文案不在本次锁定面）。
func assertEnvelope(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, wantMsg string) string {
	t.Helper()
	_, msg, data := unpackData(t, rec, wantCode)
	if wantMsg != "" && msg != wantMsg {
		t.Fatalf("成功文案 = %q, 期望 %q", msg, wantMsg)
	}
	return data
}

// TestCatalogContract_SuccessMessages 六类实体 × CRUD 的成功文案与状态码逐字锁定。
func TestCatalogContract_SuccessMessages(t *testing.T) {
	r, cfg, _ := newCatalogContractEnv(t)
	token := catalogAdminToken(t, cfg, 1)

	type entity struct {
		name string
		// 资源路径片段（/admin/<seg> 用于创建，复数用于列表）
		createPath string
		listPath   string
		itemPath   string // 含 %d
		sortPath   string // 含 %d；空串表示该实体无排序端点
		input      string
		// secondInput 交换排序用例需要第二个实体（编码不能与第一个重复）
		secondInput string
		createMsg   string
		updateMsg   string
		deleteMsg   string
		swapMsg     string
		idKey       string
		dataKey     string
		updateBody  string
	}

	entities := []entity{
		{
			name: "specialty", createPath: "/api/admin/specialty", listPath: "/api/admin/specialties",
			itemPath: "/api/admin/specialty/%d", sortPath: "/api/admin/specialty/%d/sort",
			input:       `{"code":"s1","name":"操作"}`,
			secondInput: `{"code":"s2","name":"维修"}`,
			createMsg:   "专业方向创建成功", updateMsg: "专业方向更新成功",
			deleteMsg: "专业方向删除成功", swapMsg: "排序已交换",
			idKey: "specialty_id", dataKey: "specialties", updateBody: `{"name":"操作改"}`,
		},
		{
			name: "level", createPath: "/api/admin/level", listPath: "/api/admin/levels",
			itemPath: "/api/admin/level/%d", sortPath: "/api/admin/level/%d/sort",
			input:       `{"code":"l1","name":"入门"}`,
			secondInput: `{"code":"l2","name":"进阶"}`,
			createMsg:   "课程等级创建成功", updateMsg: "课程等级更新成功",
			deleteMsg: "课程等级删除成功", swapMsg: "排序已交换",
			idKey: "level_id", dataKey: "levels", updateBody: `{"name":"入门改"}`,
		},
		{
			name: "certificate-template", createPath: "/api/admin/certificate-template",
			listPath: "/api/admin/certificate-templates", itemPath: "/api/admin/certificate-template/%d",
			input:     `{"code":"c1","name":"证书"}`,
			createMsg: "证书模板创建成功", updateMsg: "证书模板更新成功", deleteMsg: "证书模板删除成功",
			idKey: "id", dataKey: "certificate_templates", updateBody: `{"validity_days":730}`,
		},
		{
			name: "question-tag", createPath: "/api/admin/question-tag", listPath: "/api/admin/question-tags",
			itemPath:  "/api/admin/question-tag/%d",
			input:     `{"code":"t1","name":"液压"}`,
			createMsg: "题库标签创建成功", updateMsg: "题库标签更新成功", deleteMsg: "题库标签删除成功",
			idKey: "id", dataKey: "tags", updateBody: `{"name":"液压改"}`,
		},
		{
			name: "credential", createPath: "/api/admin/credential", listPath: "/api/admin/credentials",
			itemPath: "/api/admin/credential/%d", sortPath: "/api/admin/credential/%d/sort",
			input:       `{"code":"d1","name":"叉车证","category":"special_operation"}`,
			secondInput: `{"code":"d2","name":"电工证","category":"special_operation"}`,
			createMsg:   "证件创建成功", updateMsg: "证件更新成功",
			deleteMsg: "证件删除成功", swapMsg: "排序已交换",
			idKey: "id", dataKey: "credentials", updateBody: `{"name":"叉车证改"}`,
		},
		{
			name: "position", createPath: "/api/admin/position", listPath: "/api/admin/positions",
			itemPath: "/api/admin/position/%d", sortPath: "/api/admin/position/%d/sort",
			input:       `{"code":"p1","name":"叉车司机"}`,
			secondInput: `{"code":"p2","name":"维修工"}`,
			createMsg:   "岗位创建成功", updateMsg: "岗位已更新",
			deleteMsg: "岗位已删除", swapMsg: "排序已更新",
			idKey: "position_id", dataKey: "positions", updateBody: `{"name":"叉车司机改"}`,
		},
	}

	for _, e := range entities {
		t.Run(e.name, func(t *testing.T) {
			// 创建：201 + 文案 + 载荷
			rec := catalogRequest(t, r, token, http.MethodPost, e.createPath, e.input)
			data := assertEnvelope(t, rec, http.StatusCreated, e.createMsg)
			var created map[string]any
			if err := json.Unmarshal([]byte(data), &created); err != nil {
				t.Fatalf("解析创建响应失败: %v", err)
			}
			id, ok := created[e.idKey]
			if !ok {
				t.Fatalf("创建响应缺 %s: %+v", e.idKey, created)
			}
			itemID := int(id.(float64))

			// 列表：200 + data 以声明的键承载数组（两个新登记的端点在这里拿到第一个用例）
			rec = catalogRequest(t, r, token, http.MethodGet, e.listPath, "")
			data = assertEnvelope(t, rec, http.StatusOK, "")
			var list map[string]json.RawMessage
			if err := json.Unmarshal([]byte(data), &list); err != nil {
				t.Fatalf("解析列表响应失败: %v", err)
			}
			if _, ok := list[e.dataKey]; !ok {
				t.Fatalf("列表响应的 data 缺键 %q: %s", e.dataKey, data)
			}

			// 更新：200 + 文案 + 载荷
			rec = catalogRequest(t, r, token, http.MethodPut, fmt.Sprintf(e.itemPath, itemID), e.updateBody)
			assertEnvelope(t, rec, http.StatusOK, e.updateMsg)

			// 排序（有该端点的实体）：200 + 文案 + 无载荷
			if e.sortPath != "" {
				second := catalogRequest(t, r, token, http.MethodPost, e.createPath, e.secondInput)
				data = assertEnvelope(t, second, http.StatusCreated, e.createMsg)
				var secondCreated map[string]any
				if err := json.Unmarshal([]byte(data), &secondCreated); err != nil {
					t.Fatalf("解析第二个实体失败: %v", err)
				}
				secondID := int(secondCreated[e.idKey].(float64))
				rec = catalogRequest(t, r, token, http.MethodPut,
					fmt.Sprintf(e.sortPath, secondID), fmt.Sprintf(`{"swap_with":%d}`, itemID))
				data = assertEnvelope(t, rec, http.StatusOK, e.swapMsg)
				if data != "null" {
					t.Fatalf("排序端点无载荷，data 应为 null，得到 %s", data)
				}
			}

			// 删除：200 + 文案 + 无载荷
			rec = catalogRequest(t, r, token, http.MethodDelete, fmt.Sprintf(e.itemPath, itemID), "")
			data = assertEnvelope(t, rec, http.StatusOK, e.deleteMsg)
			if data != "null" {
				t.Fatalf("删除端点无载荷，data 应为 null，得到 %s", data)
			}
		})
	}
}

// TestCatalogContract_PublicPositions 公开岗位字典端点（/positions）的契约：
// 学员端与招聘端共用，仅启用项；ADR-0053 §7 把它补进契约后它才第一次有类型与用例。
func TestCatalogContract_PublicPositions(t *testing.T) {
	r, cfg, _ := newCatalogContractEnv(t)
	token := catalogAdminToken(t, cfg, 1)

	if rec := catalogRequest(t, r, token, http.MethodPost, "/api/admin/position",
		`{"code":"driver","name":"叉车司机"}`); rec.Code != http.StatusCreated {
		t.Fatalf("建岗位失败: %d", rec.Code)
	}

	rec := catalogRequest(t, r, "", http.MethodGet, "/api/positions", "")
	data := assertEnvelope(t, rec, http.StatusOK, "")
	var body struct {
		Positions []map[string]any `json:"positions"`
	}
	if err := json.Unmarshal([]byte(data), &body); err != nil {
		t.Fatalf("解析公开岗位响应失败: %v", err)
	}
	if len(body.Positions) != 1 {
		t.Fatalf("公开岗位列表应有 1 项: %+v", body.Positions)
	}
	assertDictKeys(t, body.Positions[0],
		[]string{"code", "created_at", "description", "name", "position_id", "sort_order", "status"})
}
