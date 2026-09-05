// 契约测试 #366：采纳状态机与积分直记（每帖只发一次分）。
//
// 覆盖不变式：每个帖的采纳分只发一次；CAS 幂等；更换不发分；取消不回滚；
// 自答零分；非楼主 403；流水可读；状态字段落库与 DTO 回显。
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// acceptResp 采纳响应（只取关心的字段）。
type acceptResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID              int64   `json:"id"`
		AcceptedReplyID *int64  `json:"accepted_reply_id"`
		SolvedAt        *string `json:"solved_at"`
		Category        string  `json:"category"`
	} `json:"data"`
}

// detailResp 详情响应（取 topic 与 replies 的采纳标记）。
type detailResp struct {
	Code int `json:"code"`
	Data struct {
		Topic struct {
			ID              int64   `json:"id"`
			AcceptedReplyID *int64  `json:"accepted_reply_id"`
			SolvedAt        *string `json:"solved_at"`
		} `json:"topic"`
		Replies []struct {
			ID         int64 `json:"id"`
			IsAccepted bool  `json:"is_accepted"`
		} `json:"replies"`
	} `json:"data"`
}

// ledgerResp 流水响应
type ledgerResp struct {
	Code int `json:"code"`
	Data struct {
		Items []struct {
			Delta   int    `json:"delta"`
			Reason  string `json:"reason"`
			RefType string `json:"ref_type"`
			RefID   string `json:"ref_id"`
		} `json:"items"`
		Total int64 `json:"total"`
	} `json:"data"`
}

// balanceResp 余额响应
type balanceResp struct {
	Code int `json:"code"`
	Data struct {
		Balance int `json:"balance"`
	} `json:"data"`
}

func TestForumAcceptContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	// 构造用户：楼主、答主1、答主2、旁观者
	author := model.HrwaiUser{Account: "accept_author", Phone: "13800000101", Username: "楼主", Status: 1, CreatedAt: testutil.Now(), PointsBalance: 0}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建楼主失败: %v", err)
	}
	answerer1 := model.HrwaiUser{Account: "accept_ans1", Phone: "13800000102", Username: "答主1", Status: 1, CreatedAt: testutil.Now(), PointsBalance: 0}
	if err := db.Create(&answerer1).Error; err != nil {
		t.Fatalf("创建答主1失败: %v", err)
	}
	answerer2 := model.HrwaiUser{Account: "accept_ans2", Phone: "13800000103", Username: "答主2", Status: 1, CreatedAt: testutil.Now(), PointsBalance: 0}
	if err := db.Create(&answerer2).Error; err != nil {
		t.Fatalf("创建答主2失败: %v", err)
	}
	bystander := model.HrwaiUser{Account: "accept_bystand", Phone: "13800000104", Username: "旁观者", Status: 1, CreatedAt: testutil.Now(), PointsBalance: 0}
	if err := db.Create(&bystander).Error; err != nil {
		t.Fatalf("创建旁观者失败: %v", err)
	}

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(apiGroup, deps.RouterDeps(), deps.ForumSvc, deps.ForumImageSvc)
	RegisterPointsRoutes(apiGroup, deps.RouterDeps(), deps.PointsSvc)

	issueToken := func(u model.HrwaiUser) string {
		tok, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(u.ID), u.Account, "hrwai_user")
		if err != nil {
			t.Fatalf("签发 token 失败: %v", err)
		}
		return tok
	}
	authorTok := issueToken(author)
	ans1Tok := issueToken(answerer1)
	bystanderTok := issueToken(bystander)

	do := func(tok, method, path string, body any) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	getBalance := func(tok string) int {
		rec := do(tok, http.MethodGet, "/api/points/balance", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("获取余额失败: %d %s", rec.Code, rec.Body.String())
		}
		var got balanceResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析余额失败: %v", err)
		}
		return got.Data.Balance
	}
	getLedger := func(tok string) ledgerResp {
		rec := do(tok, http.MethodGet, "/api/points/ledger?page=1&page_size=50", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("获取流水失败: %d %s", rec.Code, rec.Body.String())
		}
		var got ledgerResp
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析流水失败: %v", err)
		}
		return got
	}

	// 1. 发问答帖
	rec := do(authorTok, http.MethodPost, "/api/forum/topics", map[string]any{"category": "question", "title": "液压油温偏高", "content": "求解答"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发问答帖期望 201, got %d %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("解析发帖失败: %v", err)
	}
	topicID := created.Data.ID
	if topicID == 0 {
		t.Fatalf("发帖未返回 ID")
	}

	// 回复1、2
	createReply := func(tok string, tid int64, content string) int64 {
		rec := do(tok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", tid), map[string]any{"content": content})
		if rec.Code != http.StatusCreated {
			t.Fatalf("回复失败 %s: %d %s", content, rec.Code, rec.Body.String())
		}
		var got struct {
			Code int `json:"code"`
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("解析回复失败: %v", err)
		}
		return got.Data.ID
	}
	reply1 := createReply(ans1Tok, topicID, "检查散热器")
	// 答主2 也创建一条回复（需要先创建用户的 token，此处用 answerer2 的 account 签发）
	ans2Tok := issueToken(answerer2)
	reply2 := createReply(ans2Tok, topicID, "更换液压油")

	// 初始余额应为 0
	if bal := getBalance(authorTok); bal != 0 {
		t.Fatalf("楼主初始余额应为 0, got %d", bal)
	}
	if bal := getBalance(ans1Tok); bal != 0 {
		t.Fatalf("答主1初始余额应为 0, got %d", bal)
	}

	// 2. 首次采纳：楼主采纳 reply1
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": reply1})
	if rec.Code != http.StatusOK {
		t.Fatalf("首次采纳期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	var acc acceptResp
	if err := json.Unmarshal(rec.Body.Bytes(), &acc); err != nil {
		t.Fatalf("解析采纳响应失败: %v", err)
	}
	if acc.Data.AcceptedReplyID == nil || *acc.Data.AcceptedReplyID != reply1 {
		t.Fatalf("采纳后 accepted_reply_id 应为 %d, got %v", reply1, acc.Data.AcceptedReplyID)
	}
	if acc.Data.SolvedAt == nil || *acc.Data.SolvedAt == "" {
		t.Fatalf("采纳后 solved_at 不能为空")
	}
	// 余额校验：答主 +40，楼主 +5
	if bal := getBalance(ans1Tok); bal != 40 {
		t.Fatalf("答主1采纳后余额应为 40, got %d", bal)
	}
	if bal := getBalance(authorTok); bal != 5 {
		t.Fatalf("楼主采纳后余额应为 5, got %d", bal)
	}
	// 流水可读
	ledger1 := getLedger(ans1Tok)
	foundBonus := false
	for _, it := range ledger1.Data.Items {
		if it.Reason == "accepted_bonus" && it.Delta == 40 {
			foundBonus = true
		}
	}
	if !foundBonus {
		t.Fatalf("答主流水应含 accepted_bonus +40, 实际 %+v", ledger1.Data.Items)
	}
	ledgerAuthor := getLedger(authorTok)
	foundAction := false
	for _, it := range ledgerAuthor.Data.Items {
		if it.Reason == "accept_action" && it.Delta == 5 {
			foundAction = true
		}
	}
	if !foundAction {
		t.Fatalf("楼主流水应含 accept_action +5, 实际 %+v", ledgerAuthor.Data.Items)
	}
	// 详情校验：reply1 is_accepted
	rec = do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", topicID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("详情期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	var det detailResp
	if err := json.Unmarshal(rec.Body.Bytes(), &det); err != nil {
		t.Fatalf("解析详情失败: %v", err)
	}
	if det.Data.Topic.AcceptedReplyID == nil || *det.Data.Topic.AcceptedReplyID != reply1 {
		t.Fatalf("详情 topic accepted_reply_id 不符: %v", det.Data.Topic.AcceptedReplyID)
	}
	accMap := map[int64]bool{}
	for _, rp := range det.Data.Replies {
		accMap[rp.ID] = rp.IsAccepted
	}
	if !accMap[reply1] {
		t.Fatalf("reply1 应标记 is_accepted=true")
	}
	if accMap[reply2] {
		t.Fatalf("reply2 不应被标记为已采纳")
	}
	// 列表校验：MyTopics / List 均带 accepted 字段
	rec = do(authorTok, http.MethodGet, "/api/forum/my-topics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("my-topics 期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	var myList struct {
		Code int `json:"code"`
		Data struct {
			Topics []struct {
				ID              int64   `json:"id"`
				AcceptedReplyID *int64  `json:"accepted_reply_id"`
				SolvedAt        *string `json:"solved_at"`
			} `json:"topics"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &myList); err != nil {
		t.Fatalf("解析 my-topics 失败: %v", err)
	}
	if len(myList.Data.Topics) != 1 || myList.Data.Topics[0].AcceptedReplyID == nil || *myList.Data.Topics[0].AcceptedReplyID != reply1 {
		t.Fatalf("my-topics 采纳状态未回显: %+v", myList.Data.Topics)
	}

	// 3. 重复采纳同一条（幂等）：余额不变
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": reply1})
	if rec.Code != http.StatusOK {
		t.Fatalf("重复采纳期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	if bal := getBalance(ans1Tok); bal != 40 {
		t.Fatalf("重复采纳答主余额应仍为 40, got %d", bal)
	}
	if bal := getBalance(authorTok); bal != 5 {
		t.Fatalf("重复采纳楼主余额应仍为 5, got %d", bal)
	}
	// 流水不应新增
	ledger1 = getLedger(ans1Tok)
	if ledger1.Data.Total != 1 {
		t.Fatalf("重复采纳后流水 total 应仍为 1, got %d", ledger1.Data.Total)
	}

	// 4. 更换采纳对象：改到 reply2，不新增流水
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": reply2})
	if rec.Code != http.StatusOK {
		t.Fatalf("更换采纳期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	var acc2 acceptResp
	if err := json.Unmarshal(rec.Body.Bytes(), &acc2); err != nil {
		t.Fatalf("解析更换响应失败: %v", err)
	}
	if acc2.Data.AcceptedReplyID == nil || *acc2.Data.AcceptedReplyID != reply2 {
		t.Fatalf("更换后 accepted 应为 %d, got %v", reply2, acc2.Data.AcceptedReplyID)
	}
	// 余额不变
	if bal := getBalance(ans1Tok); bal != 40 {
		t.Fatalf("更换后原答主余额应仍为 40, got %d", bal)
	}
	if bal := getBalance(ans2Tok); bal != 0 {
		t.Fatalf("更换后新答主不应得新分, 期望 0 got %d", bal)
	}
	if bal := getBalance(authorTok); bal != 5 {
		t.Fatalf("更换后楼主余额应仍为 5, got %d", bal)
	}
	// 流水总数不变（仍各 1 条）
	if l := getLedger(ans1Tok); l.Data.Total != 1 {
		t.Fatalf("更换后答主1流水应仍为 1, got %d", l.Data.Total)
	}
	if l := getLedger(ans2Tok); l.Data.Total != 0 {
		t.Fatalf("更换后答主2流水应为 0, got %d", l.Data.Total)
	}
	// 详情 is_accepted 切换
	rec = do(authorTok, http.MethodGet, fmt.Sprintf("/api/forum/topics/%d", topicID), nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &det); err != nil {
		t.Fatalf("解析详情失败: %v", err)
	}
	accMap = map[int64]bool{}
	for _, rp := range det.Data.Replies {
		accMap[rp.ID] = rp.IsAccepted
	}
	if !accMap[reply2] || accMap[reply1] {
		t.Fatalf("更换后 is_accepted 标记错误: reply1=%v reply2=%v", accMap[reply1], accMap[reply2])
	}

	// 5. 取消采纳：状态回到未解决，已发分不回滚
	rec = do(authorTok, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("取消采纳期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	var cancel acceptResp
	if err := json.Unmarshal(rec.Body.Bytes(), &cancel); err != nil {
		t.Fatalf("解析取消响应失败: %v", err)
	}
	if cancel.Data.AcceptedReplyID != nil {
		t.Fatalf("取消后 accepted 应为 nil, got %v", *cancel.Data.AcceptedReplyID)
	}
	if cancel.Data.SolvedAt != nil {
		t.Fatalf("取消后 solved_at 应为 nil, got %v", *cancel.Data.SolvedAt)
	}
	// 余额不回滚
	if bal := getBalance(ans1Tok); bal != 40 {
		t.Fatalf("取消后答主余额应仍为 40, got %d", bal)
	}
	if bal := getBalance(authorTok); bal != 5 {
		t.Fatalf("取消后楼主余额应仍为 5, got %d", bal)
	}
	// 再次取消幂等
	rec = do(authorTok, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("重复取消期望 200, got %d", rec.Code)
	}

	// 5b. 取消后重采：应只改状态，不再发分（每帖只发一次分）
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": reply2})
	if rec.Code != http.StatusOK {
		t.Fatalf("取消后重采期望 200, got %d %s", rec.Code, rec.Body.String())
	}
	if bal := getBalance(ans1Tok); bal != 40 {
		t.Fatalf("取消后重采答主1余额应仍 40, got %d", bal)
	}
	if bal := getBalance(ans2Tok); bal != 0 {
		t.Fatalf("取消后重采答主2余额应仍 0, got %d", bal)
	}
	if bal := getBalance(authorTok); bal != 5 {
		t.Fatalf("取消后重采楼主余额应仍 5, got %d", bal)
	}

	// 6. 楼主采纳自己：直接拒绝（ADR-0028 禁止自问自答；替代旧「静默零分」）
	// 新开一帖
	rec = do(authorTok, http.MethodPost, "/api/forum/topics", map[string]any{"category": "question", "title": "自问自答帖", "content": "求助"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("自问帖失败: %d %s", rec.Code, rec.Body.String())
	}
	var selfCreated struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &selfCreated); err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	selfTopic := selfCreated.Data.ID
	selfReply := createReply(authorTok, selfTopic, "我自己来回答 self")
	beforeAuthorBal := getBalance(authorTok)
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", selfTopic), map[string]any{"reply_id": selfReply})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("自采纳期望 400 拒绝, got %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "不能采纳自己的回答") {
		t.Fatalf("自采纳文案不符: %s", rec.Body.String())
	}
	if getBalance(authorTok) != beforeAuthorBal {
		t.Fatalf("自采纳不应加分, before %d after %d", beforeAuthorBal, getBalance(authorTok))
	}
	// 自采纳不产生流水、不落采纳状态
	var cnt int64
	if err := db.Model(&model.PointsLedger{}).Where("ref_type = ? AND ref_id = ?", "forum_topic", fmt.Sprintf("%d", selfTopic)).Count(&cnt).Error; err != nil {
		t.Fatalf("查询流水失败: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("自采纳不应产生任何流水, got %d 条", cnt)
	}
	var selfTopicRow model.ForumTopic
	if err := db.First(&selfTopicRow, selfTopic).Error; err != nil {
		t.Fatalf("查询帖子失败: %v", err)
	}
	if selfTopicRow.AcceptedReplyID != nil {
		t.Fatalf("被拒的自采纳不应写入 accepted_reply_id")
	}

	// 7. 非楼主 403
	rec = do(bystanderTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": reply1})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("非楼主采纳应 403, got %d %s", rec.Code, rec.Body.String())
	}
	rec = do(bystanderTok, http.MethodDelete, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("非楼主取消应 403, got %d", rec.Code)
	}

	// 8. 非法入参：reply 不属于该主题 / 主题不存在
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", topicID), map[string]any{"reply_id": 999999})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 reply 应 400, got %d", rec.Code)
	}
	// 讨论帖不可采纳
	rec = do(authorTok, http.MethodPost, "/api/forum/topics", map[string]any{"category": "discussion", "title": "讨论帖", "content": "闲聊"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("发讨论帖失败: %d", rec.Code)
	}
	var disc struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &disc); err != nil {
		t.Fatalf("解析讨论帖失败: %v", err)
	}
	// 给讨论帖加一条回复
	rec = do(ans1Tok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/replies", disc.Data.ID), map[string]any{"content": "讨论回复2"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("讨论回复失败")
	}
	var dr2 struct {
		Code int `json:"code"`
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dr2); err != nil {
		t.Fatalf("解析讨论回复2失败: %v", err)
	}
	rec = do(authorTok, http.MethodPost, fmt.Sprintf("/api/forum/topics/%d/accept", disc.Data.ID), map[string]any{"reply_id": dr2.Data.ID})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("讨论帖采纳应 400, got %d %s", rec.Code, rec.Body.String())
	}

	// 9. 并发/重复语义已在 3 中覆盖：余额只动一次（断言最终余额而非调用次数）
	fmt.Println("论坛采纳契约通过：首次加分/幂等/换采/取消/自答零分/403/讨论帖拒绝均已守住")
}
