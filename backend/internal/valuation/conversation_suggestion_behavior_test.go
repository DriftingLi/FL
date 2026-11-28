package valuation

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
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
			{"field":"mast_height_mm","state":"value","value":9999}
		]`, `{}`, `[]`), nil
	}}
	r, _, _, _ := newSuggestionTestEngine(t, model, defaultSuggestionConfig(), zap.NewNop())
	draft := createConversationDraft(t, r, map[string]any{})
	opID := "op:" + draft["id"].(string) + ":invalid-combination"
	requestSuggestion(t, r, draft["id"].(string), "invalid-combination", 1, "捏造目录组合", "")
	operation := waitForSuggestion(t, r, opID)
	if operation["status"] != OperationStatusFailed || operation["error_code"] != ErrorCodeSuggestionValidation {
		t.Fatalf("不存在的车型组合必须校验失败: %v", operation)
	}
	_, _, got := decodeBody(t, performRequest(r, http.MethodGet, "/api/valuation/conversation/drafts/"+draft["id"].(string), nil))
	if got["version"] != float64(1) {
		t.Fatalf("失败建议不得留下部分车型字段: %v", got)
	}
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
