package valuation

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"go.uber.org/zap"
)

type recordingTrialSnapshotReader struct {
	current       *TrialSnapshot
	stored        map[string]*TrialSnapshot
	requests      []TrialRequest
	currentLoads  int
	snapshotLoads int
}

func (r *recordingTrialSnapshotReader) LoadTrialSnapshot(_ context.Context, req TrialRequest) (*TrialSnapshot, error) {
	r.requests = append(r.requests, req)
	if req.SnapshotID != "" {
		r.snapshotLoads++
		snapshot := r.stored[req.SnapshotID]
		if snapshot == nil {
			return nil, fmt.Errorf("快照不存在")
		}
		return snapshot, nil
	}
	r.currentLoads++
	if r.current == nil {
		return nil, fmt.Errorf("当前快照不存在")
	}
	return r.current, nil
}

func TestTrialHTTPEchoesRequestAndSnapshotIDs(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})

	t.Run("首次试算返回冻结快照标识", func(t *testing.T) {
		reader := &recordingTrialSnapshotReader{current: snapshot}
		router, _, evaluations := newTestValuationEngine(t)
		RegisterTrialRoutes(router, reader, zap.NewNop(), "development")
		req := trialCompleteRequest()
		req.RequestID = "trial-first-request"
		response := performRequest(router, http.MethodPost, "/api/valuation/trials", req)
		if response.Code != http.StatusOK {
			t.Fatalf("首次试算状态码=%d: %s", response.Code, response.Body.String())
		}
		data := mustDecodeData(t, response)
		if data["request_id"] != req.RequestID || data["snapshot_id"] != snapshot.Hash {
			t.Fatalf("响应必须回显请求与冻结快照标识: %v", data)
		}
		if reader.currentLoads != 1 || reader.snapshotLoads != 0 || len(reader.requests) != 1 || reader.requests[0].RequestID != req.RequestID {
			t.Fatalf("首次请求应透传 request_id 并读取当前快照: %+v", reader)
		}
		if len(evaluations.records) != 0 {
			t.Fatal("试算不得写正式评估")
		}
	})

	t.Run("指定旧快照时不读当前数据库快照", func(t *testing.T) {
		reader := &recordingTrialSnapshotReader{stored: map[string]*TrialSnapshot{snapshot.Hash: snapshot}}
		router, _, _ := newTestValuationEngine(t)
		RegisterTrialRoutes(router, reader, zap.NewNop(), "development")
		req := trialCompleteRequest()
		req.RequestID = "trial-recalculation"
		req.SnapshotID = snapshot.Hash
		response := performRequest(router, http.MethodPost, "/api/valuation/trials", req)
		if response.Code != http.StatusOK {
			t.Fatalf("旧快照重算状态码=%d: %s", response.Code, response.Body.String())
		}
		data := mustDecodeData(t, response)
		if data["request_id"] != req.RequestID || data["snapshot_id"] != snapshot.Hash {
			t.Fatalf("重算响应必须回显新 request_id 与原 snapshot_id: %v", data)
		}
		if reader.currentLoads != 0 || reader.snapshotLoads != 1 || len(reader.requests) != 1 || reader.requests[0].SnapshotID != snapshot.Hash {
			t.Fatalf("旧快照重算不得回读当前快照: %+v", reader)
		}
	})
}

func TestTrialRejectsMismatchedSnapshotID(t *testing.T) {
	price := OriginalPrice{ID: 1, Brand: "林德", VehicleType: "电动叉车", Series: "K系列", Tonnage: 3,
		ConfigType: "标准", MastType: "标准门架", MastHeightMM: 3000, EarliestFactoryYear: 2020, OriginalPrice: 100000}
	snapshot := newTrialSnapshotForTest(t, []OriginalPrice{price})
	req := trialCompleteRequest()
	req.SnapshotID = "not-the-loaded-snapshot"
	response, err := CalculateTrial(context.Background(), req, snapshot, "development")
	if err != nil {
		t.Fatal(err)
	}
	if response.Status != TrialStatusNeedsInfo || response.ReasonCode != TrialReasonInvalidInput || response.Interval != nil ||
		len(response.MissingFields) != 1 || response.MissingFields[0] != "snapshot_id" {
		t.Fatalf("快照标识与载荷不一致必须 fail-closed: %+v", response)
	}
}
