// spec #940 片二契约测试：主题详情的响应形状锁定（ADR-0047 §3 读面 typed DTO / ADR-0009 §2）。
//
// 收口前的形态是 service 返回 map[string]any（encoding/json 对 map 按 key 排序输出）。
// 收口后是 typed struct —— 本测试钉住三件事，保证「换类型不换字节」：
//  1. data 顶层 key **逐字且按序**为 page / pages / replies / topic / total（字母序 = 旧 map 的序列化序）；
//  2. 学员端详情与管理端详情**字节全等**（两处已共用同一实现，不再各写一份）；
//  3. 越界页的 replies 是空数组而不是 null（沿用旧 map 的 make(...) 语义）。
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
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// detailEnvelope 只取 data 的原始字节：形状断言直接在字节层做，避免二次解码掩盖顺序。
type detailEnvelope struct {
	Code int             `json:"code"`
	Data json.RawMessage `json:"data"`
}

// dataKeys 按**出现顺序**返回 data 对象的顶层 key（json.Decoder 逐 token 走）。
func dataKeys(t *testing.T, data json.RawMessage) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("读取 data 起始 token 失败: %v（data=%s）", err, string(data))
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		t.Fatalf("data 不是 JSON 对象: %s", string(data))
	}
	var keys []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatalf("读取 key 失败: %v", err)
		}
		keys = append(keys, k.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatalf("跳过 %v 的值失败: %v", k, err)
		}
	}
	return keys
}

func TestForumTopicDetailShapeContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)

	cfg := &config.Config{
		JWTSecretKey: "contract-test-secret",
		AuthCookie:   config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterForumRoutes(api, deps.RouterDeps(), deps.ForumSvc, deps.ForumModSvc, deps.ForumImageSvc)

	now := testutil.Now()
	author := model.HrwaiUser{Account: "shape_author", Phone: "13800000901", Username: "楼主", Status: 1, CreatedAt: now}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	topic := model.ForumTopic{
		UserID: int(author.ID), Category: "discussion", Title: "形状", Content: "如题",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&topic).Error; err != nil {
		t.Fatalf("建主题失败: %v", err)
	}
	for i := 0; i < 3; i++ {
		rp := model.ForumReply{
			TopicID: topic.ID, UserID: int(author.ID),
			Content:   fmt.Sprintf("回复 %d", i+1),
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		}
		if err := db.Create(&rp).Error; err != nil {
			t.Fatalf("建回复失败: %v", err)
		}
	}

	// 两条路径都用「楼主自己」作为 viewer：自帖不累加浏览量（ADR-0041），
	// 两次请求之间没有副作用 ⇒ 响应可以逐字节比对。
	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{})
	authorTok, err := sess.Issue(int(author.ID), author.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发学员 token 失败: %v", err)
	}
	adminTok, err := sess.Issue(int(author.ID), author.Account, "admin")
	if err != nil {
		t.Fatalf("签发管理端 token 失败: %v", err)
	}

	fetch := func(tok, path string) detailEnvelope {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("GET %s 期望 200, got %d %s", path, w.Code, w.Body.String())
		}
		var env detailEnvelope
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("解析 %s 失败: %v", path, err)
		}
		return env
	}

	detailPath := "/api/forum/topics/" + fmt.Sprint(topic.ID)
	adminPath := "/api/admin/forum/topics/" + fmt.Sprint(topic.ID)

	student := fetch(authorTok, detailPath)
	admin := fetch(adminTok, adminPath)

	t.Run("data 顶层 key 逐字且按序（字母序 = 旧 map 序列化序）", func(t *testing.T) {
		want := []string{"page", "pages", "replies", "topic", "total"}
		got := dataKeys(t, student.Data)
		if len(got) != len(want) {
			t.Fatalf("data key = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("data key 顺序 = %v, want %v（顺序变了就是响应字节变了）", got, want)
			}
		}
	})

	t.Run("学员端与管理端详情字节全等（已共用同一实现）", func(t *testing.T) {
		if !bytes.Equal(student.Data, admin.Data) {
			t.Fatalf("两端详情不一致：\n学员端: %s\n管理端: %s", string(student.Data), string(admin.Data))
		}
	})

	t.Run("越界页的 replies 是空数组而非 null", func(t *testing.T) {
		env := fetch(authorTok, detailPath+"?page=99&page_size=10")
		raw := struct {
			Replies json.RawMessage `json:"replies"`
		}{}
		if err := json.Unmarshal(env.Data, &raw); err != nil {
			t.Fatalf("解析越界页失败: %v", err)
		}
		if string(raw.Replies) != "[]" {
			t.Fatalf("越界页 replies = %s, want []（空数组，前端据此走空态）", string(raw.Replies))
		}
	})
}
