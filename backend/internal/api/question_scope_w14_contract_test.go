// Package api ADR-0062 票 3（题目半边）回归：学员把题目读/写进出的每一条路径都必须收
// 「入口装配的 scope」（可见性谓词 + 当前证件分区），不得靠 caller 自觉。
//
// 形状照 question_pool_readpath_test.go（#981 的 by-id 那条，同一判据的另一半）：
// 学员 = 题库池口径（published + 排源标记真题题 + 当前证件），draft / pending / 真题题 /
// 非当前证件一律按「不存在」；讲师编辑面不受影响（读 draft 是它的本职）。
//
// 钉住的五条漏口（本票修复前逐条可复现）：
//  1. GET /question-bank/questions     —— 完全不查池，status 不传即全量，出口带答案；
//  2. GET /notes                       —— LEFT JOIN question 回填题干时不带池谓词；
//  3. PUT /questions/:id/note          —— 写面只判题存在，可把笔记挂在不可见题上；
//  4. GET|POST /questions/:id/comments —— 连题目存在性都不查（枚举 + 挂载）；
//  5. POST /favorites (question 支)    —— 只判 published，不排真题、不分区证件。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// poolLeakFixture 一份「学员池内外各一题」的夹具 + 四种身份口径的 token。
type poolLeakFixture struct {
	t    *testing.T
	db   *gorm.DB
	r    *gin.Engine
	cred int

	studentID int

	studentToken string
	tutorToken   string

	poolQ       model.Question // 池内：published + 当前证件 + 非真题
	draftQ      model.Question // draft（学员不可见）
	pendingQ    model.Question // pending（学员不可见）
	sourceQ     model.Question // published + 当前证件 + 源标记真题（学员不可见）
	otherCredQ  model.Question // published + 别的证件（学员不可见）
	deletedSeen model.Question // 池内题的对照组（证明「拦的是池，不是全部」）
}

// newPoolLeakFixture 播种夹具。当前证件 = credA；学员选 credA。
func newPoolLeakFixture(t *testing.T) *poolLeakFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "scope-w14-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}

	credA, credB := 11, 12
	student := model.HrwaiUser{Account: "scope_w14_user", Phone: "13800000611", Username: "学员",
		Status: 1, CurrentCredentialID: &credA, CreatedAt: testutil.Now()}
	if err := db.Create(&student).Error; err != nil {
		t.Fatalf("建学员失败: %v", err)
	}
	tutor := model.Tutor{Username: "scope_w14_tutor", Password: "x", Name: "讲师", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&tutor).Error; err != nil {
		t.Fatalf("建讲师失败: %v", err)
	}

	f := &poolLeakFixture{t: t, db: db, cred: credA}
	f.studentID = int(student.ID)
	mkQ := func(content, status string, cred *int) model.Question {
		q := model.Question{
			Type: "single_choice", Content: content, Options: model.JSONB(`{"A":"甲","B":"乙"}`),
			Answer: "A", Explanation: "解析:" + content, Status: status, CredentialID: cred,
			CreatedByType: "tutor", CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
		}
		if err := db.Create(&q).Error; err != nil {
			t.Fatalf("建题失败: %v", err)
		}
		return q
	}
	f.poolQ = mkQ("池内题", "published", &credA)
	f.deletedSeen = mkQ("池内对照组题", "published", &credA)
	f.draftQ = mkQ("草稿题", "draft", &credA)
	f.pendingQ = mkQ("待审题", "pending", &credA)
	f.otherCredQ = mkQ("别的证件题", "published", &credB)
	f.sourceQ = mkQ("真题题", "published", &credA)
	tag := model.QuestionTag{Code: "SRCW14", Name: "真题", IsSourceTag: true, Status: 1,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now()}
	if err := db.Create(&tag).Error; err != nil {
		t.Fatalf("建标签失败: %v", err)
	}
	if err := db.Create(&model.QuestionTagRelation{QuestionID: f.sourceQ.ID, TagID: tag.ID, CreatedAt: testutil.Now()}).Error; err != nil {
		t.Fatalf("建标签关系失败: %v", err)
	}

	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	st, err := sess.Issue(student.ID, student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发学员 token 失败: %v", err)
	}
	tt, err := sess.Issue(tutor.TutorID, tutor.Username, "tutor")
	if err != nil {
		t.Fatalf("签发讲师 token 失败: %v", err)
	}
	f.studentToken, f.tutorToken = st, tt

	r := gin.New()
	api := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	RegisterQuestionBankRoutes(api, deps.RouterDeps(), deps.QuestionBankSvc, deps.FileSvc)
	RegisterNoteRoutes(api, deps.RouterDeps(), deps.NoteSvc)
	RegisterQuestionInteractionRoutes(api, deps.RouterDeps(), deps.QuestionCommentSvc, deps.NoteSvc, deps.QuestionKnowledgeSvc)
	RegisterFavoriteRoutes(api, deps.RouterDeps(), deps.FavoriteSvc)
	f.r = r
	return f
}

// hiddenByID 池外题集合：id → 名字（四条判据共用同一份，逐题跑一遍）。
func (f *poolLeakFixture) hiddenByID() map[int]string {
	return map[int]string{
		f.draftQ.ID: "draft", f.pendingQ.ID: "pending",
		f.sourceQ.ID: "source-tag", f.otherCredQ.ID: "other-credential",
	}
}

// seedNote / seedComment 直接落库（绕开写面），用于单测读面的泄漏。
func (f *poolLeakFixture) seedNote(questionID, userID int, content string) {
	f.t.Helper()
	qid := questionID
	if err := f.db.Create(&model.Note{QuestionID: &qid, UserID: userID, Content: content, UpdatedAt: testutil.Now()}).Error; err != nil {
		f.t.Fatalf("播种笔记失败: %v", err)
	}
}

func (f *poolLeakFixture) seedComment(questionID, userID int, content string) {
	f.t.Helper()
	if err := f.db.Create(&model.QuestionComment{QuestionID: questionID, UserID: userID, Content: content, CreatedAt: testutil.Now()}).Error; err != nil {
		f.t.Fatalf("播种评论失败: %v", err)
	}
}

// body 取响应体字符串（判「有没有漏出题干文案」用，比解析 JSON 更不容易被字段名骗过）。
func doAndBody(t *testing.T, f *poolLeakFixture, token, method, path string, body any) (int, string) {
	t.Helper()
	rec := doWithToken(t, f.r, token, method, path, body)
	return rec.Code, rec.Body.String()
}

// TestStudentQuestionListIsPoolScoped 锁 1：GET /question-bank/questions 的学员支必须过池。
// 修复前该端点只挂 JWTAuth、status 不传即全量 ⇒ 学员 token 翻走 draft/pending/真题题。
// 讲师支（CapQuestionAuthor）必须仍能读全量含 draft——那是它的本职，不得回归。
func TestStudentQuestionListIsPoolScoped(t *testing.T) {
	f := newPoolLeakFixture(t)

	_, body := doAndBody(t, f, f.studentToken, http.MethodGet, "/api/question-bank/questions?page_size=50", nil)
	if strings.Contains(body, "草稿题") || strings.Contains(body, "待审题") ||
		strings.Contains(body, "真题题") || strings.Contains(body, "别的证件题") {
		t.Fatalf("学员列表泄漏池外题: %s", body)
	}
	if !strings.Contains(body, "池内题") {
		t.Fatalf("学员列表应含池内题: %s", body)
	}
	// status 参数不得被学员用作绕池通道（池的「已发布」在 scope 里，不在入参里）。
	if _, b := doAndBody(t, f, f.studentToken, http.MethodGet, "/api/question-bank/questions?page_size=50&status=draft", nil); strings.Contains(b, "草稿题") {
		t.Fatalf("status=draft 不得让学员读到草稿题: %s", b)
	}
	// 编辑面：讲师读得到 draft。
	if _, b := doAndBody(t, f, f.tutorToken, http.MethodGet, "/api/question-bank/questions?page_size=50", nil); !strings.Contains(b, "草稿题") {
		t.Fatalf("讲师列表读不到 draft（编辑面回归）: %s", b)
	}
}

// TestNoteListHidesOutOfPoolStem 锁 2：「我的笔记」回填题干必须带池谓词。
// 学员历史上把笔记挂在池外题上（本用例直接落库模拟修复前的存量），列表也不得把题干吐出来；
// 笔记行本身是学员自己的私有数据，仍列出（只是摘要为空），与「题目已删除」同一形态。
func TestNoteListHidesOutOfPoolStem(t *testing.T) {
	f := newPoolLeakFixture(t)
	for id := range f.hiddenByID() {
		f.seedNote(id, f.studentID, "挂在池外题上的笔记")
	}
	f.seedNote(f.poolQ.ID, f.studentID, "挂在池内题上的笔记")

	_, body := doAndBody(t, f, f.studentToken, http.MethodGet, "/api/notes?scope=question&page_size=50", nil)
	var resp struct {
		Data struct {
			Items []struct {
				QuestionID      *int   `json:"question_id"`
				QuestionContent string `json:"question_content"`
				Content         string `json:"content"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("解析笔记列表失败: %v %s", err, body)
	}
	hidden := f.hiddenByID()
	seen := map[int]string{f.poolQ.ID: "池内题"}
	if len(resp.Data.Items) != len(hidden)+1 {
		t.Fatalf("笔记行数应不变（私有数据照列），got %d: %s", len(resp.Data.Items), body)
	}
	for _, it := range resp.Data.Items {
		if it.QuestionID == nil {
			t.Fatalf("题目笔记的 question_id 应在: %+v", it)
		}
		if name, ok := hidden[*it.QuestionID]; ok {
			if it.QuestionContent != "" {
				t.Fatalf("池外题（%s）的题干不得回填，got %q: %s", name, it.QuestionContent, body)
			}
			continue
		}
		if want, ok := seen[*it.QuestionID]; ok && it.QuestionContent != want {
			t.Fatalf("池内题的题干应照常回填 %q, got %q", want, it.QuestionContent)
		}
	}
}

// TestNoteWriteRejectsOutOfPoolQuestion 锁 3：笔记写面（每人每题一条）必须收 scope。
func TestNoteWriteRejectsOutOfPoolQuestion(t *testing.T) {
	f := newPoolLeakFixture(t)
	for id, name := range f.hiddenByID() {
		code, body := doAndBody(t, f, f.studentToken, http.MethodPut,
			fmt.Sprintf("/api/questions/%d/note", id), map[string]any{"content": "挂上去"})
		if code == http.StatusOK {
			t.Fatalf("学员不得把笔记挂在 %s 上: %d %s", name, code, body)
		}
		if code != http.StatusNotFound {
			t.Fatalf("池外题按不存在处理，%s got %d %s", name, code, body)
		}
		var cnt int64
		f.db.Model(&model.Note{}).Where("question_id = ?", id).Count(&cnt)
		if cnt != 0 {
			t.Fatalf("%s 上不得留下笔记行", name)
		}
	}
	// 单条读面同口径：池外题的笔记读取按不存在（修复前只按 (question_id,user_id) 查，不问题）。
	if code, _ := doAndBody(t, f, f.studentToken, http.MethodGet,
		fmt.Sprintf("/api/questions/%d/note", f.draftQ.ID), nil); code != http.StatusNotFound {
		t.Fatalf("读池外题的笔记应 404, got %d", code)
	}
	if code, body := doAndBody(t, f, f.studentToken, http.MethodPut,
		fmt.Sprintf("/api/questions/%d/note", f.poolQ.ID), map[string]any{"content": "池内笔记"}); code != http.StatusOK {
		t.Fatalf("池内题应可保存笔记, got %d %s", code, body)
	}
}

// TestCommentReadRejectsOutOfPoolQuestion 锁 4：评论列表不得成为「池外题存在性/内容」的枚举器。
// 修复前 List 连题目存在性都不查 ⇒ 直调 id 即可读到任意题的评论。
func TestCommentReadRejectsOutOfPoolQuestion(t *testing.T) {
	f := newPoolLeakFixture(t)
	for id := range f.hiddenByID() {
		f.seedComment(id, f.studentID, "别人留在这题上的评论")
	}
	f.seedComment(f.poolQ.ID, f.studentID, "池内题的评论")

	for id, name := range f.hiddenByID() {
		code, body := doAndBody(t, f, f.studentToken, http.MethodGet,
			fmt.Sprintf("/api/questions/%d/comments?page_size=10", id), nil)
		if code == http.StatusOK {
			t.Fatalf("池外题 %s 的评论不得可枚举: %s", name, body)
		}
		if code != http.StatusNotFound {
			t.Fatalf("池外题 %s 应 404, got %d %s", name, code, body)
		}
	}
	code, body := doAndBody(t, f, f.studentToken, http.MethodGet,
		fmt.Sprintf("/api/questions/%d/comments?page_size=10", f.poolQ.ID), nil)
	if code != http.StatusOK || !strings.Contains(body, "池内题的评论") {
		t.Fatalf("池内题的评论应照常列出, got %d %s", code, body)
	}
}

// TestCommentWriteRejectsOutOfPoolQuestion 锁 4（写面半边）：池外题不可被挂评论。
func TestCommentWriteRejectsOutOfPoolQuestion(t *testing.T) {
	f := newPoolLeakFixture(t)
	for id, name := range f.hiddenByID() {
		code, body := doAndBody(t, f, f.studentToken, http.MethodPost,
			fmt.Sprintf("/api/questions/%d/comments", id), map[string]any{"content": "挂上去"})
		if code == http.StatusCreated {
			t.Fatalf("学员不得给 %s 挂评论: %s", name, body)
		}
		if code != http.StatusNotFound {
			t.Fatalf("给池外题挂评论应 404, %s got %d %s", name, code, body)
		}
	}
	if code, body := doAndBody(t, f, f.studentToken, http.MethodPost,
		fmt.Sprintf("/api/questions/%d/comments", f.poolQ.ID), map[string]any{"content": "这题易错"}); code != http.StatusCreated {
		t.Fatalf("池内题应可评论, got %d %s", code, body)
	}
}

// TestFavoriteQuestionRequiresPool 锁 5：收藏写面的题目支必须走池（含排真题与证件分区）。
// 修复前只判 status='published'，收藏后经 GET /api/favorites 回题干快照。
func TestFavoriteQuestionRequiresPool(t *testing.T) {
	f := newPoolLeakFixture(t)
	for id, name := range f.hiddenByID() {
		code, body := doAndBody(t, f, f.studentToken, http.MethodPost, "/api/favorites",
			map[string]any{"target_type": "question", "target_id": id})
		if code == http.StatusCreated {
			t.Fatalf("不得收藏 %s: %s", name, body)
		}
	}
	if code, body := doAndBody(t, f, f.studentToken, http.MethodPost, "/api/favorites",
		map[string]any{"target_type": "question", "target_id": f.poolQ.ID}); code != http.StatusCreated {
		t.Fatalf("池内题应可收藏, got %d %s", code, body)
	}
	_, list := doAndBody(t, f, f.studentToken, http.MethodGet, "/api/favorites?target_type=question", nil)
	var resp struct {
		Data struct {
			Favorites []struct {
				TargetID int    `json:"target_id"`
				Title    string `json:"title"`
			} `json:"favorites"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(list), &resp); err != nil {
		t.Fatalf("解析收藏列表失败: %v %s", err, list)
	}
	if len(resp.Data.Favorites) != 1 || resp.Data.Favorites[0].TargetID != f.poolQ.ID {
		t.Fatalf("收藏列表应只剩池内题: %s", list)
	}
}
