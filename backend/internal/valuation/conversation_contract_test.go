package valuation

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/clock"
	"forklift-training/internal/storage"
)

func TestConversationDraftCreateAndGetPreservesZeroAndFalse(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)

	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts", map[string]any{
		"fields": map[string]any{
			"usage_hours": map[string]any{
				"state": "value", "value": 0, "source": "user", "evidence": []string{"仪表盘"},
			},
			"has_registration_certificate": map[string]any{
				"state": "value", "value": false, "source": "user", "evidence": []string{"用户明确说明"},
			},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("创建草稿状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	code, _, data := decodeBody(t, w)
	if code != http.StatusOK {
		t.Fatalf("创建草稿业务码 = %d, 期望 200", code)
	}
	id, _ := data["id"].(string)
	if len(id) < 32 {
		t.Fatalf("草稿能力 ID 必须是不可猜的随机标识: %q", id)
	}
	if data["version"] != float64(1) || data["status"] != "editing" {
		t.Fatalf("新草稿应为 version=1/status=editing: %v", data)
	}
	fields, _ := data["fields"].(map[string]any)
	hours := fields["usage_hours"].(map[string]any)
	certificate := fields["has_registration_certificate"].(map[string]any)
	if hours["state"] != "value" || hours["value"] != float64(0) || hours["source"] != "user" {
		t.Errorf("真实零工时未保留: %v", hours)
	}
	if certificate["state"] != "value" || certificate["value"] != false {
		t.Errorf("明确无登记证未保留: %v", certificate)
	}
	if len(hours["evidence"].([]any)) != 1 || len(certificate["evidence"].([]any)) != 1 {
		t.Errorf("字段依据未保留: hours=%v certificate=%v", hours, certificate)
	}

	w = performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("读取草稿状态码 = %d, 期望 200\nbody=%s", w.Code, w.Body.String())
	}
	_, _, got := decodeBody(t, w)
	if got["id"] != id || got["version"] != float64(1) {
		t.Errorf("读取草稿与创建结果不一致: %v", got)
	}
	gotFields := got["fields"].(map[string]any)
	if gotFields["usage_hours"].(map[string]any)["value"] != float64(0) ||
		gotFields["has_registration_certificate"].(map[string]any)["value"] != false {
		t.Errorf("读取后 0/false 被默认值覆盖: %v", gotFields)
	}
}

func TestConversationDraftPatchCASDeleteAndFieldStates(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)
	draft := createConversationDraft(t, r, map[string]any{})
	id := draft["id"].(string)

	w := performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+id, map[string]any{
		"base_version": 1,
		"fields": map[string]any{
			"usage_hours": map[string]any{"state": "range", "range": map[string]any{"min": 4000, "max": 6000}, "source": "user"},
			"tonnage": map[string]any{"state": "candidate", "candidates": []any{
				map[string]any{"id": "3t", "label": "3 吨", "value": 3, "source": "dictionary"},
				map[string]any{"id": "3.5t", "label": "3.5 吨", "value": 3.5, "source": "dictionary"},
			}},
			"condition_rating": map[string]any{"state": "unknown", "source": "user", "evidence": "用户说不清楚"},
		},
		"supplemental": map[string]any{"engine_model": "YC6B", "repair": "去年发动机大修"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, patched := decodeBody(t, w)
	if patched["version"] != float64(2) || patched["can_undo"] != true || patched["can_submit"] != false {
		t.Fatalf("PATCH 版本/可撤回/可提交状态错误: %v", patched)
	}
	fields := patched["fields"].(map[string]any)
	if fields["usage_hours"].(map[string]any)["state"] != "range" ||
		fields["tonnage"].(map[string]any)["state"] != "candidate" ||
		fields["condition_rating"].(map[string]any)["state"] != "unknown" {
		t.Errorf("字段状态未保留: %v", fields)
	}
	if fields["brand"].(map[string]any)["state"] != "missing" {
		t.Errorf("未提供字段应保持 missing: %v", fields["brand"])
	}
	if patched["supplemental"].(map[string]any)["repair"] != "去年发动机大修" {
		t.Errorf("补充资料未保留: %v", patched["supplemental"])
	}

	w = performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+id, map[string]any{
		"base_version": 1,
		"fields":       map[string]any{"brand": map[string]any{"state": "value", "value": "合力"}},
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("过期 base_version 应返回 409, got %d\nbody=%s", w.Code, w.Body.String())
	}

	w = performRequest(r, http.MethodDelete, "/api/valuation/conversation/drafts/"+id, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DELETE 状态码 = %d, 期望 204\nbody=%s", w.Code, w.Body.String())
	}
	w = performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+id, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("删除后 GET 应返回 404, got %d", w.Code)
	}
}

func TestConversationDraftIdleTTLAndReadDoesNotExtend(t *testing.T) {
	fakeClock := clock.At(time.Date(2026, 11, 24, 10, 0, 0, 0, time.UTC))
	r, _, _, _ := newTestValuationEngineWithDrafts(t, storage.NewLocalStorage(t.TempDir()), newMemDraftRepository(), fakeClock.Now)
	draft := createConversationDraft(t, r, map[string]any{})
	id := draft["id"].(string)

	fakeClock.T = fakeClock.T.Add(29 * time.Minute)
	w := performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("闲置 29 分钟应可恢复: %d", w.Code)
	}
	fakeClock.T = fakeClock.T.Add(time.Minute)
	w = performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+id, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("读取不得延长 TTL，闲置 30 分钟应过期: %d", w.Code)
	}

	draft = createConversationDraft(t, r, map[string]any{})
	id = draft["id"].(string)
	fakeClock.T = fakeClock.T.Add(29 * time.Minute)
	w = performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+id, map[string]any{
		"base_version": 1,
		"fields":       map[string]any{"brand": map[string]any{"state": "value", "value": "合力"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("闲置 29 分钟仍可修改: %d", w.Code)
	}
	fakeClock.T = fakeClock.T.Add(time.Minute)
	w = performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+id, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("成功修改应刷新 30 分钟闲置 TTL: %d", w.Code)
	}
}

func TestConversationDraftUndoCreatesNewVersionAndRestoresEvidence(t *testing.T) {
	r, _, _ := newTestValuationEngine(t)
	draft := createConversationDraft(t, r, map[string]any{
		"fields": map[string]any{
			"usage_hours": map[string]any{"state": "value", "value": 0, "source": "user", "evidence": []string{"仪表盘"}},
		},
	})
	id := draft["id"].(string)
	w := performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+id, map[string]any{
		"base_version": 1,
		"fields": map[string]any{
			"usage_hours": map[string]any{"state": "value", "value": 4800, "source": "user", "evidence": []string{"用户纠正"}},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("纠正工时失败: %d", w.Code)
	}
	w = performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+id, map[string]any{
		"base_version": 2,
		"fields":       map[string]any{"tonnage": map[string]any{"state": "value", "value": 3.5, "source": "user"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("修改吨位失败: %d", w.Code)
	}

	w = performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts/"+id+"/undo", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("undo 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, undone := decodeBody(t, w)
	if undone["version"] != float64(4) || undone["status"] != "editing" || undone["can_undo"] != false {
		t.Fatalf("undo 应生成 version=4 的 editing 草稿: %v", undone)
	}
	fields := undone["fields"].(map[string]any)
	if fields["tonnage"].(map[string]any)["state"] != "missing" {
		t.Errorf("undo 未撤回最近一次吨位修改: %v", fields["tonnage"])
	}
	hours := fields["usage_hours"].(map[string]any)
	if hours["value"] != float64(4800) || hours["source"] != "user" || len(hours["evidence"].([]any)) != 1 {
		t.Errorf("undo 未保留撤回点字段依据: %v", hours)
	}

	w = performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts/"+id+"/undo", map[string]any{"base_version": 4})
	if w.Code != http.StatusConflict {
		t.Fatalf("单级 undo 后再次 undo 应返回 409, got %d", w.Code)
	}
}

func TestConversationDraftUpstreamCorrectionRevalidatesOnlyDownstream(t *testing.T) {
	r, dict, _ := newTestValuationEngine(t)
	dict.originalPrices = append(dict.originalPrices, OriginalPrice{
		Brand: "合力", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3.5,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
	})
	draft := createConversationDraft(t, r, map[string]any{"fields": map[string]any{
		"brand":          map[string]any{"state": "value", "value": "合力"},
		"vehicle_type":   map[string]any{"state": "value", "value": "电动叉车"},
		"series":         map[string]any{"state": "value", "value": "K系列"},
		"tonnage":        map[string]any{"state": "value", "value": 3},
		"config_type":    map[string]any{"state": "value", "value": "标准", "source": "user", "evidence": "原确认"},
		"mast_type":      map[string]any{"state": "value", "value": "旧门架", "source": "user", "evidence": "原确认"},
		"mast_height_mm": map[string]any{"state": "value", "value": 3000},
		"usage_hours":    map[string]any{"state": "value", "value": 0, "source": "user", "evidence": []string{"仪表盘"}},
	}})
	id := draft["id"].(string)
	w := performRequest(r, http.MethodPatch, "/api/valuation/conversation/drafts/"+id, map[string]any{
		"base_version": 1,
		"fields":       map[string]any{"tonnage": map[string]any{"state": "value", "value": 3.5, "source": "user"}},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("更正吨位失败: %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, patched := decodeBody(t, w)
	fields := patched["fields"].(map[string]any)
	config := fields["config_type"].(map[string]any)
	if config["state"] != "value" || config["value"] != "标准" {
		t.Fatalf("仍兼容的配置应保留确认值: %v", config)
	}
	mast := fields["mast_type"].(map[string]any)
	if mast["state"] != "candidate" || mast["source"] != "user" || len(mast["candidates"].([]any)) != 1 {
		t.Fatalf("不兼容门架应转 candidate 并保留原值与依据: %v", mast)
	}
	if mast["candidates"].([]any)[0].(map[string]any)["value"] != "旧门架" {
		t.Errorf("候选未保留原门架值: %v", mast)
	}
	hours := fields["usage_hours"].(map[string]any)
	if hours["state"] != "value" || hours["value"] != float64(0) || len(hours["evidence"].([]any)) != 1 {
		t.Errorf("无关工时事实与依据被级联覆盖: %v", hours)
	}

	w = performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts/"+id+"/undo", map[string]any{"base_version": 2})
	if w.Code != http.StatusOK {
		t.Fatalf("undo 级联修改失败: %d", w.Code)
	}
	_, _, undone := decodeBody(t, w)
	undoneFields := undone["fields"].(map[string]any)
	if undoneFields["tonnage"].(map[string]any)["value"] != float64(3) ||
		undoneFields["config_type"].(map[string]any)["state"] != "value" ||
		undoneFields["mast_type"].(map[string]any)["value"] != "旧门架" {
		t.Errorf("undo 未恢复上游及其下游状态: %v", undoneFields)
	}
}

func createConversationDraft(t *testing.T, r *gin.Engine, body map[string]any) map[string]any {
	t.Helper()
	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/drafts", body)
	if w.Code != http.StatusOK {
		t.Fatalf("创建草稿失败: %d\nbody=%s", w.Code, w.Body.String())
	}
	_, _, data := decodeBody(t, w)
	return data
}
