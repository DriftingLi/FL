// 契约测试 #889（ADR-0045）：IP 属地快照。
//
// 守五件事：
//  1. 发帖与回复**两条写入路径同口径**：发布那一刻取一次属地并落库。
//  2. **编辑不改属地**：编辑路径不写该字段（属地是发布那一刻的事实，不是用户资料）。
//  3. **内网 / 保留 / 非法 IP 落空**：空串而不是报错，也不阻断发帖。
//  4. 存量行（本次改造之前发的帖）属地为空串。
//  5. DTO 读写往返：创建响应 / 详情 / 列表三处都回传同一份值。
//
// **刻意不绑定「某个 IP → 某个省」的映射**（spec #887 测试口径）：xdb 是数据快照，
// 更新后映射会变。这里只断言「与 geolocation.Resolve 的结果一致」与「非空」，
// 具体是哪个省留给库本身。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/geolocation"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// 公共地址（114DNS），用于「有属地」的路径；内网地址用于「落空」的路径。
const (
	regionPublicIP  = "114.114.114.114"
	regionPrivateIP = "192.168.1.9"
)

type ipRegion struct {
	Province string `json:"ip_province"`
	City     string `json:"ip_city"`
}

type ipRegionResp struct {
	Code int `json:"code"`
	Data struct {
		ID int64 `json:"id"`
		ipRegion
	} `json:"data"`
}

type ipRegionDetailResp struct {
	Code int `json:"code"`
	Data struct {
		Topic struct {
			ID int64 `json:"id"`
			ipRegion
		} `json:"topic"`
		Replies []struct {
			ID int64 `json:"id"`
			ipRegion
		} `json:"replies"`
	} `json:"data"`
}

type ipRegionListResp struct {
	Code int `json:"code"`
	Data struct {
		Topics []struct {
			ID int64 `json:"id"`
			ipRegion
		} `json:"topics"`
	} `json:"data"`
}

func TestForumIPRegionContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(api, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)

	now := testutil.Now()
	mkUser := func(account, phone, name string) model.HrwaiUser {
		u := model.HrwaiUser{Account: account, Phone: phone, Username: name, Status: 1, CreatedAt: now}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("建用户失败: %v", err)
		}
		return u
	}
	author := mkUser("ir_author", "13800000801", "楼主")
	replier := mkUser("ir_replier", "13800000802", "答主")
	tok := func(u model.HrwaiUser) string {
		s, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
			Issue(int(u.ID), u.Account, "hrwai_user")
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return s
	}
	authorTok, replierTok := tok(author), tok(replier)

	// do 以指定客户端 IP 发请求：测试装配里 TRUSTED_PROXIES 为空，
	// middleware.ClientIP 取 TCP 对端地址，故 RemoteAddr 就是属地解析的输入。
	do := func(tk, method, path, clientIP string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+tk)
		if clientIP != "" {
			req.RemoteAddr = clientIP + ":51820"
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	want := geolocation.Resolve(regionPublicIP)
	if want.Province == "" {
		t.Fatalf("前置条件失败：%s 在库中应能解出省级属地（xdb 数据异常？）", regionPublicIP)
	}

	var topicID int64

	t.Run("发帖：属地按发布时的客户端 IP 落库并回传", func(t *testing.T) {
		w := do(authorTok, http.MethodPost, "/api/forum/topics", regionPublicIP, map[string]any{
			"category": "discussion", "title": "属地帖", "content": "正文",
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("发帖应 201: %d %s", w.Code, w.Body.String())
		}
		var got ipRegionResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析发帖响应失败: %v", err)
		}
		topicID = got.Data.ID
		if got.Data.Province != want.Province || got.Data.City != want.City {
			t.Errorf("创建响应属地 = {%q, %q}, want {%q, %q}",
				got.Data.Province, got.Data.City, want.Province, want.City)
		}

		var row model.ForumTopic
		if err := db.First(&row, topicID).Error; err != nil {
			t.Fatalf("读回落库行失败: %v", err)
		}
		if row.IPProvince != want.Province || row.IPCity != want.City {
			t.Errorf("落库属地 = {%q, %q}, want {%q, %q}",
				row.IPProvince, row.IPCity, want.Province, want.City)
		}
	})

	t.Run("回复：与发帖同口径", func(t *testing.T) {
		w := do(replierTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", topicID),
			regionPublicIP, map[string]any{"content": "回复正文"})
		if w.Code != http.StatusCreated {
			t.Fatalf("回复应 201: %d %s", w.Code, w.Body.String())
		}
		var got ipRegionResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析回复响应失败: %v", err)
		}
		if got.Data.Province != want.Province || got.Data.City != want.City {
			t.Errorf("回复响应属地 = {%q, %q}, want {%q, %q}",
				got.Data.Province, got.Data.City, want.Province, want.City)
		}
		var row model.ForumReply
		if err := db.Order("id DESC").First(&row).Error; err != nil {
			t.Fatalf("读回回复行失败: %v", err)
		}
		if row.IPProvince != want.Province || row.IPCity != want.City {
			t.Errorf("回复落库属地 = {%q, %q}, want {%q, %q}",
				row.IPProvince, row.IPCity, want.Province, want.City)
		}
	})

	t.Run("详情与列表：回传同一份快照", func(t *testing.T) {
		w := do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d?page=1&page_size=20", topicID), "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("详情应 200: %d %s", w.Code, w.Body.String())
		}
		var detail ipRegionDetailResp
		if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
			t.Fatalf("解析详情失败: %v", err)
		}
		if detail.Data.Topic.Province != want.Province || detail.Data.Topic.City != want.City {
			t.Errorf("详情主题属地 = {%q, %q}, want {%q, %q}",
				detail.Data.Topic.Province, detail.Data.Topic.City, want.Province, want.City)
		}
		if len(detail.Data.Replies) != 1 {
			t.Fatalf("详情应含 1 条回复, got %d", len(detail.Data.Replies))
		}
		if rp := detail.Data.Replies[0]; rp.Province != want.Province || rp.City != want.City {
			t.Errorf("详情回复属地 = {%q, %q}, want {%q, %q}", rp.Province, rp.City, want.Province, want.City)
		}

		w = do(authorTok, http.MethodGet, "/api/forum/topics?scope=all&page=1&page_size=50", "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("列表应 200: %d %s", w.Code, w.Body.String())
		}
		var list ipRegionListResp
		if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
			t.Fatalf("解析列表失败: %v", err)
		}
		found := false
		for _, tp := range list.Data.Topics {
			if tp.ID != topicID {
				continue
			}
			found = true
			if tp.Province != want.Province || tp.City != want.City {
				t.Errorf("列表主题属地 = {%q, %q}, want {%q, %q}", tp.Province, tp.City, want.Province, want.City)
			}
		}
		if !found {
			t.Error("列表里应能找到刚发的帖")
		}
	})

	t.Run("编辑帖子不改属地", func(t *testing.T) {
		// 用一个完全不同的客户端 IP 编辑：属地必须仍是发布那一刻的快照。
		w := do(authorTok, http.MethodPut, fmt.Sprintf("/api/forum/topics/%d", topicID), "8.8.8.8", map[string]any{
			"category": "discussion", "title": "改过标题", "content": "改过正文",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("编辑应 200: %d %s", w.Code, w.Body.String())
		}
		var got ipRegionResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析编辑响应失败: %v", err)
		}
		if got.Data.Province != want.Province || got.Data.City != want.City {
			t.Errorf("编辑后响应属地 = {%q, %q}, want 保持 {%q, %q}",
				got.Data.Province, got.Data.City, want.Province, want.City)
		}
		var row model.ForumTopic
		if err := db.First(&row, topicID).Error; err != nil {
			t.Fatalf("读回落库行失败: %v", err)
		}
		if row.IPProvince != want.Province || row.IPCity != want.City {
			t.Errorf("编辑后落库属地 = {%q, %q}, want 保持 {%q, %q}",
				row.IPProvince, row.IPCity, want.Province, want.City)
		}
	})

	t.Run("内网地址发帖：属地落空且不阻断", func(t *testing.T) {
		w := do(authorTok, http.MethodPost, "/api/forum/topics", regionPrivateIP, map[string]any{
			"category": "discussion", "title": "内网来源帖", "content": "正文",
		})
		if w.Code != http.StatusCreated {
			t.Fatalf("内网来源不应阻断发帖: %d %s", w.Code, w.Body.String())
		}
		var got ipRegionResp
		if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析发帖响应失败: %v", err)
		}
		if got.Data.Province != "" || got.Data.City != "" {
			t.Errorf("内网地址应落空属地, got {%q, %q}", got.Data.Province, got.Data.City)
		}
	})

	t.Run("存量行：属地为空串", func(t *testing.T) {
		// 直接插一行模拟本次改造之前的历史帖（没有属地）。
		legacy := model.ForumTopic{
			Category: "discussion", UserID: int(author.ID), Title: "存量帖", Content: "正文",
			ContentFormat: "text", CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&legacy).Error; err != nil {
			t.Fatalf("插入存量行失败: %v", err)
		}
		w := do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", legacy.ID), "", nil)
		if w.Code != http.StatusOK {
			t.Fatalf("详情应 200: %d %s", w.Code, w.Body.String())
		}
		var detail ipRegionDetailResp
		if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
			t.Fatalf("解析详情失败: %v", err)
		}
		if detail.Data.Topic.Province != "" || detail.Data.Topic.City != "" {
			t.Errorf("存量行属地应为空串, got {%q, %q}", detail.Data.Topic.Province, detail.Data.Topic.City)
		}
		// 字段必须在 JSON 里出现（前端按空串判断整段不渲染，而不是 undefined）。
		if !bytes.Contains(w.Body.Bytes(), []byte(`"ip_province"`)) {
			t.Error("详情响应应包含 ip_province 字段（空串仍是字段，不是缺失）")
		}
	})
}
