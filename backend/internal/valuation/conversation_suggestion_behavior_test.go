package valuation

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"forklift-training/internal/aiassistant"
)

func TestValuationSuggestionOneSentenceFillsRealVehicleCombination(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[
			{"field":"brand","state":"value","value":"合力"},
			{"field":"vehicle_type","state":"value","value":"电动叉车"},
			{"field":"series","state":"value","value":"K系列"},
			{"field":"tonnage","state":"value","value":3},
			{"field":"config_type","state":"value","value":"标准"},
			{"field":"mast_type","state":"value","value":"标准门架"},
			{"field":"mast_height_mm","state":"value","value":3000},
			{"field":"usage_hours","state":"value","value":0},
			{"field":"has_registration_certificate","state":"value","value":false}
		]`, `{"engine_model":"不可核对型号"}`, `[]`), nil
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":one-sentence"
	w := requestSuggestion(t, r, draft["id"].(string), "one-sentence", 1, "合力电动叉车K系列3吨标准配置标准门架3000毫米，0工时，没有登记证", "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("一句话多项受理失败: %d\n%s", w.Code, w.Body.String())
	}
	operation := waitForSuggestion(t, r, opID)
	if operation["status"] != OperationStatusSucceeded {
		t.Fatalf("真实完整车型组合应通过: %v", operation)
	}
	_, _, got := decodeBody(t, performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+draft["id"].(string), nil))
	fields := got["fields"].(map[string]any)
	for name, want := range map[string]any{
		"brand": "合力", "vehicle_type": "电动叉车", "series": "K系列", "tonnage": float64(3),
		"config_type": "标准", "mast_type": "标准门架", "mast_height_mm": float64(3000),
		"usage_hours": float64(0), "has_registration_certificate": false,
	} {
		if fields[name].(map[string]any)["value"] != want {
			t.Fatalf("字段 %s = %v, want %v", name, fields[name], want)
		}
	}
	if got["supplemental"].(map[string]any)["engine_model"] != "不可核对型号" {
		t.Fatalf("无可靠关联型号应保留为补充资料: %v", got["supplemental"])
	}
}

func TestValuationSuggestionOneSentenceLeavesCascadeCandidates(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[
			{"field":"brand","state":"value","value":"合力"},
			{"field":"vehicle_type","state":"value","value":"电动叉车"},
			{"field":"series","state":"value","value":"K系列"},
			{"field":"tonnage","state":"candidate","candidate_keys":["tonnage:3","tonnage:3.5"]}
		]`, `{}`, `["请确认吨位"]`), nil
	}}
	r, dict, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	dict.originalPrices = append(dict.originalPrices, OriginalPrice{
		Brand: "合力", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3.5,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000,
	})
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":one-sentence-candidates"
	w := requestSuggestion(t, r, draft["id"].(string), "one-sentence-candidates", 1, "合力电动叉车K系列，可能3或3.5吨", "")
	if w.Code != http.StatusAccepted {
		t.Fatalf("一句话候选受理失败: %d\n%s", w.Code, w.Body.String())
	}
	operation := waitForSuggestion(t, r, opID)
	if operation["status"] != OperationStatusSucceeded {
		t.Fatalf("一句话候选应成功: %v", operation)
	}
	_, _, got := decodeBody(t, performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+draft["id"].(string), nil))
	tonnage := got["fields"].(map[string]any)["tonnage"].(map[string]any)
	if tonnage["state"] != string(DraftFieldCandidate) || len(tonnage["candidates"].([]any)) != 2 {
		t.Fatalf("一句话多项中的歧义必须留给用户: %v", tonnage)
	}
}

func TestValuationSuggestionRejectsInvalidVehicleCombination(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[
			{"field":"brand","state":"value","value":"合力"},
			{"field":"vehicle_type","state":"value","value":"电动叉车"},
			{"field":"series","state":"value","value":"K系列"},
			{"field":"tonnage","state":"value","value":3},
			{"field":"config_type","state":"value","value":"标准"},
			{"field":"mast_type","state":"value","value":"标准门架"},
			{"field":"mast_height_mm","state":"value","value":9999},
			{"field":"usage_hours","state":"value","value":800}
		]`, `{}`, `[]`), nil
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":invalid-combination"
	requestSuggestion(t, r, draft["id"].(string), "invalid-combination", 1, "捏造目录组合", "")
	operation := waitForSuggestion(t, r, opID)
	if operation["status"] != OperationStatusSucceeded {
		t.Fatalf("不存在的车型组合应拒绝车型补丁但保留无关合法补丁: %v", operation)
	}
	_, _, got := decodeBody(t, performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+draft["id"].(string), nil))
	if got["version"] != float64(2) {
		t.Fatalf("合法补丁应生成新版本: %v", got)
	}
	fields := got["fields"].(map[string]any)
	if fields["mast_height_mm"].(map[string]any)["state"] != string(DraftFieldMissing) {
		t.Fatalf("目录外的门架高度不应落盘: %v", fields["mast_height_mm"])
	}
	if fields["brand"].(map[string]any)["value"] != "合力" {
		t.Fatalf("独立通过目录校验的品牌补丁应保留: %v", fields["brand"])
	}
	if fields["usage_hours"].(map[string]any)["value"] != float64(800) {
		t.Fatalf("无关合法补丁应保留: %v", fields["usage_hours"])
	}
}

func TestValuationSuggestionSingleMastHeightValidatesMergedCombination(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[
			{"field":"mast_height_mm","state":"value","value":9999},
			{"field":"usage_hours","state":"value","value":123}
		]`, `{}`, `[]`), nil
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createCompleteConversationDraft(t, r)
	operationID := "op:" + draft["id"].(string) + ":single-mast-height"
	requestSuggestion(t, r, draft["id"].(string), "single-mast-height", 1, "门架改成 9999，工时 123", "")
	operation := waitForSuggestion(t, r, operationID)
	if operation["status"] != OperationStatusSucceeded {
		t.Fatalf("单项失败不应丢弃合法补丁: %v", operation)
	}
	_, _, got := decodeBody(t, performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+draft["id"].(string), nil))
	fields := got["fields"].(map[string]any)
	if fields["mast_height_mm"].(map[string]any)["value"] != float64(baseEvalRequest().MastHeightMM) {
		t.Fatalf("与现有车型组合冲突的门架高度必须被拒绝: %v", fields["mast_height_mm"])
	}
	if fields["usage_hours"].(map[string]any)["value"] != float64(123) {
		t.Fatalf("合法工时补丁应独立保留: %v", fields["usage_hours"])
	}
	questions := operation["result"].(map[string]any)["questions"].([]any)
	if !suggestionQuestionsContain(questions, "mast_height_mm") {
		t.Fatalf("失败字段应通过现有 questions 返回问题: %v", questions)
	}
}

func TestValuationSuggestionRejectsRangesForStringsAndBooleansIndependently(t *testing.T) {
	model := &suggestionModelFake{structured: func(context.Context, string, []*schema.Message, string, aiassistant.StructuredCompleteOptions) (string, error) {
		return suggestionJSON(`[
			{"field":"brand","state":"range","range":{"min":1,"max":2}},
			{"field":"original_paint","state":"range","range":{"min":0,"max":1}},
			{"field":"usage_hours","state":"range","range":{"min":100,"max":200}}
		]`, `{}`, `[]`), nil
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	operationID := "op:" + draft["id"].(string) + ":typed-ranges"
	requestSuggestion(t, r, draft["id"].(string), "typed-ranges", 1, "品牌和车况给范围，工时一百到二百", "")
	operation := waitForSuggestion(t, r, operationID)
	if operation["status"] != OperationStatusSucceeded {
		t.Fatalf("错误的 range 补丁应单独拒绝: %v", operation)
	}
	_, _, got := decodeBody(t, performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+draft["id"].(string), nil))
	fields := got["fields"].(map[string]any)
	if fields["brand"].(map[string]any)["state"] != string(DraftFieldMissing) || fields["original_paint"].(map[string]any)["state"] != string(DraftFieldMissing) {
		t.Fatalf("字符串和布尔字段不得接受 range: %v", fields)
	}
	if hours := fields["usage_hours"].(map[string]any); hours["state"] != string(DraftFieldRange) || hours["range"].(map[string]any)["min"] != float64(100) {
		t.Fatalf("数值字段的合法 range 应保留: %v", hours)
	}
	questions := operation["result"].(map[string]any)["questions"].([]any)
	if !suggestionQuestionsContain(questions, "brand") || !suggestionQuestionsContain(questions, "original_paint") {
		t.Fatalf("两个失败字段都应返回问题: %v", questions)
	}
}

func suggestionQuestionsContain(questions []any, field string) bool {
	for _, question := range questions {
		if text, ok := question.(string); ok && strings.Contains(text, field) {
			return true
		}
	}
	return false
}

func TestValuationSuggestionNamespaceDoesNotShadowEvaluationOperation(t *testing.T) {
	r, _, _, _ := newSuggestionTestEngine(t, &suggestionModelFake{}, defaultSuggestionConfig(), zap.NewNop())
	draft := createCompleteConversationDraft(t, r)
	body := map[string]any{
		"operation_id": "op:evaluation", "draft_id": draft["id"], "draft_version": 1, "request": baseEvalRequest(),
	}
	w := performRequest(r, http.MethodPost, "/api/valuation/conversation/operations", body)
	if w.Code != http.StatusOK {
		t.Fatalf("op: 前缀的正式评估操作不得被建议命名空间遮蔽: %d\n%s", w.Code, w.Body.String())
	}
	w = performRequest(r, http.MethodGet, "/api/valuation/conversation/operations/op:evaluation", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("op: 前缀的正式评估操作应可读取: %d\n%s", w.Code, w.Body.String())
	}
}

func TestValuationSuggestionSchemaRestrictsRangeToNumericFields(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal([]byte(valuationSuggestionSchemaJSON), &root); err != nil {
		t.Fatal(err)
	}
	patch := root["properties"].(map[string]any)["patches"].(map[string]any)["items"].(map[string]any)
	fieldNames := patch["properties"].(map[string]any)["field"].(map[string]any)["enum"].([]any)
	wantFields := make([]any, 0, len(draftFieldSpecs))
	for _, spec := range draftFieldSpecs {
		wantFields = append(wantFields, spec.name)
		if spec.requestPtr == nil || draftFieldKind(spec.name) != spec.kind {
			t.Fatalf("字段注册表缺少类型或 operation 投影: %+v", spec)
		}
	}
	if !reflect.DeepEqual(fieldNames, wantFields) || len(requiredDraftFields) != len(draftFieldSpecs) {
		t.Fatalf("schema/validation/operation 字段清单未收敛: schema=%v registry=%v", fieldNames, wantFields)
	}
	for i, spec := range draftFieldSpecs {
		if requiredDraftFields[i] != spec.name {
			t.Fatalf("validation 字段清单未收敛: index=%d got=%s want=%s", i, requiredDraftFields[i], spec.name)
		}
	}
	rules, ok := patch["allOf"].([]any)
	if !ok || len(rules) != 1 {
		t.Fatalf("schema 必须按字段类型限制 range: %v", patch)
	}
	condition := rules[0].(map[string]any)["if"].(map[string]any)["properties"].(map[string]any)
	fields := condition["state"].(map[string]any)["const"]
	if fields != string(DraftFieldRange) {
		t.Fatalf("range 条件错误: %v", condition)
	}
	allowed := condition["field"].(map[string]any)["enum"].([]any)
	want := []any{"tonnage", "mast_height_mm", "factory_year", "sale_year", "usage_hours"}
	if !reflect.DeepEqual(allowed, want) {
		t.Fatalf("range 字段清单 = %v, want %v", allowed, want)
	}
}

func TestValuationSuggestionSchemaIsStrictForStructuredGenerate(t *testing.T) {
	var root map[string]any
	if err := json.Unmarshal([]byte(valuationSuggestionSchemaJSON), &root); err != nil {
		t.Fatal(err)
	}
	if root["additionalProperties"] != false {
		t.Fatalf("根 schema 必须拒绝额外字段: %v", root)
	}
	properties := root["properties"].(map[string]any)
	patch := properties["patches"].(map[string]any)["items"].(map[string]any)
	if patch["additionalProperties"] != false {
		t.Fatalf("patch schema 必须拒绝 action 等额外字段: %v", patch)
	}
	wantRequired := []string{"field", "state", "value", "range", "candidate_keys", "evidence"}
	gotRequired := patch["required"].([]any)
	if len(gotRequired) != len(wantRequired) {
		t.Fatalf("严格结构化 schema 必须要求全部 patch 键: %v", gotRequired)
	}
	for i, want := range wantRequired {
		if gotRequired[i] != want {
			t.Fatalf("patch required = %v, want %v", gotRequired, wantRequired)
		}
	}
	patchProperties := patch["properties"].(map[string]any)
	if !reflect.DeepEqual(patchProperties["range"].(map[string]any)["type"], []any{"object", "null"}) {
		t.Fatalf("可选 range 必须用 null 表达: %v", patchProperties["range"])
	}
	if _, err := decodeSuggestionResult(`{"schema_version":1,"patches":[],"supplements":{},"questions":[],"action":"submit"}`); err == nil {
		t.Fatal("本地解码必须拒绝模型越权 action")
	}
}
