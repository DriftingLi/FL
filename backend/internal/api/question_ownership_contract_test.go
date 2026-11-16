// Package api 题库写隔离契约（#2598 / 规格块 C）：三条写路径的归属判据在 HTTP 面上成立吗。
//
// 服务层判据由 internal/questionbank/ownership_test.go 钉住；这里钉的是**接线**：
//   - 端点确实把令牌里的 uid 与角色交给了谓词（漏传 uid 会退化成「谁都不是本人」，
//     漏传角色会让管理员也被拦——两种漏法在服务层测试里都看不出来）；
//   - 403 的档位与文案（C3：不用 404 伪装）；
//   - 空创建者的存量题只放行管理员（C2）；
//   - 「我创建的」筛选只收行集、且学员面拿不到这个轴（C2 的另一半 + C4）。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/admincap"
	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/model"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

// ownershipEnv 写隔离契约的环境：两位讲师（A 是作者，B 是旁人）+ 一位管理员。
//
// 讲师 id 由 SeedTutor 自增给出，不写死——写死会让「B 恰好等于 A」这种夹具错误
// 变成静默通过（两个常量相等时，他人的题看起来就是本人的题）。
type ownershipEnv struct {
	t      *testing.T
	db     *gorm.DB
	r      *gin.Engine
	tutorA model.Tutor // 作者
	tutorB model.Tutor // 旁人
	tokA   string
	tokB   string
	tokAdm string
}

func newOwnershipEnv(t *testing.T) *ownershipEnv {
	t.Helper()
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "ownership-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	api := r.Group("/api")
	// 管理端能力解析源（#1618 段1）：题库存根里没有它，管理员令牌会被一律 403
	// （admin 的能力自数据层回答，静态表不再回答 admin）。本文件要验「管理员直通」，
	// 因此必须挂上——这与装配根 NewRouter 里的那条是同一份实现。
	api.Use(middleware.AdminCapabilityResolver(admincap.New(db, zap.NewNop())))
	deps := newContractDeps(t, db, cfg)
	questionbank.RegisterRoutes(api, deps.RouterDeps().Session, deps.RouterDeps().CredentialScope, deps.QuestionBankSvc, deps.FileSvc)

	tutorA := *testutil.SeedTutor(t, db, "own_tutor_a", "x")
	tutorB := *testutil.SeedTutor(t, db, "own_tutor_b", "x")
	admin := testutil.SeedAdmin(t, db, "own_admin", "x")

	sess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{Name: cfg.AuthCookie.Name})
	issue := func(id int, account, role string) string {
		tok, err := sess.Issue(id, account, role)
		if err != nil {
			t.Fatalf("签发 %s token 失败: %v", role, err)
		}
		return tok
	}
	return &ownershipEnv{
		t: t, db: db, r: r,
		tutorA: tutorA, tutorB: tutorB,
		tokA:   issue(tutorA.TutorID, tutorA.Username, "tutor"),
		tokB:   issue(tutorB.TutorID, tutorB.Username, "tutor"),
		tokAdm: issue(admin.AdminID, admin.Username, "admin"),
	}
}

func (e *ownershipEnv) seedQuestion(createdBy *int, status string) model.Question {
	e.t.Helper()
	q := model.Question{
		Type: "single_choice", Content: "归属题-" + status, Answer: "A",
		Status: status, CreatedBy: createdBy, CreatedByType: "tutor",
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := e.db.Create(&q).Error; err != nil {
		e.t.Fatalf("建题失败: %v", err)
	}
	return q
}

func (e *ownershipEnv) path(q model.Question, suffix string) string {
	return "/api/question-bank/questions/" + strconv.Itoa(q.ID) + suffix
}

// TestQuestionWriteOwnership_ThreePaths 三条写路径：本人 200 / 他人 403 / 管理员 200。
//
// 每格都断言**状态码 + 错误文案**：只说 403 不够——把 404 或 400 复用成「不是你的」
// 同样能让「别人改不了」这条成立，但那正是 C3 明确否掉的形态（编辑面全量可读，
// 存在性不是秘密，403 才让前端分得清「没权限」与「题没了」）。
func TestQuestionWriteOwnership_ThreePaths(t *testing.T) {
	t.Parallel()

	type writeOp struct {
		name   string
		method string
		suffix string
		body   any
	}
	ops := []writeOp{
		{"更新题目", http.MethodPut, "", map[string]any{"content": "改后的题干"}},
		{"提交审核", http.MethodPost, "/submit", nil},
		{"删除题目", http.MethodDelete, "", nil},
	}

	for _, op := range ops {
		t.Run(op.name, func(t *testing.T) {
			t.Run("本人放行", func(t *testing.T) {
				e := newOwnershipEnv(t)
				q := e.seedQuestion(&e.tutorA.TutorID, "draft")
				rec := doWithToken(t, e.r, e.tokA, op.method, e.path(q, op.suffix), op.body)
				if rec.Code != http.StatusOK {
					t.Fatalf("本人写自己的题应 200, got %d %s", rec.Code, rec.Body.String())
				}
			})
			t.Run("他人403", func(t *testing.T) {
				e := newOwnershipEnv(t)
				q := e.seedQuestion(&e.tutorA.TutorID, "draft")
				rec := doWithToken(t, e.r, e.tokB, op.method, e.path(q, op.suffix), op.body)
				if rec.Code != http.StatusForbidden {
					t.Fatalf("他人写应 403, got %d %s", rec.Code, rec.Body.String())
				}
				if msg := envelopeMessage(t, rec.Body.Bytes()); msg != "只能编辑自己创建的题目" {
					t.Fatalf("403 文案应为「只能编辑自己创建的题目」, got %q", msg)
				}
				// 拒绝必须真的没动数据：403 了但内容已经改掉，是最坏的形态
				var row model.Question
				if err := e.db.First(&row, q.ID).Error; err != nil {
					t.Fatalf("回读失败: %v", err)
				}
				if row.Content != q.Content || row.Status != q.Status {
					t.Fatalf("被拒的写操作改动了数据: %+v", row)
				}
			})
			t.Run("管理员直通", func(t *testing.T) {
				e := newOwnershipEnv(t)
				q := e.seedQuestion(&e.tutorA.TutorID, "draft")
				rec := doWithToken(t, e.r, e.tokAdm, op.method, e.path(q, op.suffix), op.body)
				if rec.Code != http.StatusOK {
					t.Fatalf("管理员应直通, got %d %s", rec.Code, rec.Body.String())
				}
			})
			t.Run("空创建者仅管理员", func(t *testing.T) {
				e := newOwnershipEnv(t)
				q := e.seedQuestion(nil, "draft")
				rec := doWithToken(t, e.r, e.tokA, op.method, e.path(q, op.suffix), op.body)
				if rec.Code != http.StatusForbidden {
					t.Fatalf("讲师写空创建者题应 403, got %d %s", rec.Code, rec.Body.String())
				}
				rec = doWithToken(t, e.r, e.tokAdm, op.method, e.path(q, op.suffix), op.body)
				if rec.Code != http.StatusOK {
					t.Fatalf("管理员写空创建者题应 200, got %d %s", rec.Code, rec.Body.String())
				}
			})
		})
	}
}

// TestQuestionListCreatedByFilterIsEditorOnly 「我创建的」筛选：编辑面收、学员面忽略（C2/C4）。
//
// 学员面那条断言不是形式主义：池的可见性口径只该由 scope 回答（ADR-0062 决策 4），
// 让 `?created_by=` 在学员面生效等于给学员开一条按作者收窄池的旁路。
func TestQuestionListCreatedByFilterIsEditorOnly(t *testing.T) {
	t.Parallel()
	e := newOwnershipEnv(t)
	qA := e.seedQuestion(&e.tutorA.TutorID, "draft")
	qB := e.seedQuestion(&e.tutorB.TutorID, "draft")
	// 学员可见的题（published 无证件 ⇒ 恒在池内），用来验证学员面不看 created_by
	poolQ := e.seedQuestion(&e.tutorA.TutorID, "published")
	_ = qA // 只在「A 的两道题都出现」这条断言里间接用到

	student := model.HrwaiUser{Account: "own_filter_user", Phone: "13800000888", Username: "学员", Status: 1, CreatedAt: testutil.Now()}
	if err := e.db.Create(&student).Error; err != nil {
		t.Fatalf("建学员失败: %v", err)
	}
	sess := security.NewSession("ownership-secret", time.Hour, security.CookieConfig{Name: "hrwai_token"})
	stuTok, err := sess.Issue(student.ID, student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发学员 token 失败: %v", err)
	}

	listIDs := func(token, query string) []int {
		t.Helper()
		rec := doWithToken(t, e.r, token, http.MethodGet, "/api/question-bank/questions"+query, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("列表 %q 应 200, got %d %s", query, rec.Code, rec.Body.String())
		}
		var env struct {
			Data struct {
				Questions []struct {
					ID int `json:"id"`
				} `json:"questions"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
			t.Fatalf("解列表失败: %v", err)
		}
		ids := make([]int, 0, len(env.Data.Questions))
		for _, q := range env.Data.Questions {
			ids = append(ids, q.ID)
		}
		return ids
	}

	// 编辑面：created_by=A 只出 A 的题
	onlyA := listIDs(e.tokA, "?created_by="+strconv.Itoa(e.tutorA.TutorID))
	if len(onlyA) != 2 { // qA（draft）+ poolQ（published），两道都是 A 的
		t.Fatalf("编辑面 created_by=A 应出 2 道, got %v", onlyA)
	}
	for _, id := range onlyA {
		if id == qB.ID {
			t.Fatalf("created_by=A 不该返回 B 的题 %d", qB.ID)
		}
	}
	// 编辑面不带筛选：全量（含 B 的题）——C4 读面不变
	allEdit := listIDs(e.tokA, "")
	if len(allEdit) != 3 {
		t.Fatalf("编辑面无筛选应看全量 3 道, got %v", allEdit)
	}

	// 学员面：带上 created_by 也只出池内题，旁路不成立
	stuIDs := listIDs(stuTok, "?created_by="+strconv.Itoa(e.tutorA.TutorID))
	if len(stuIDs) != 1 || stuIDs[0] != poolQ.ID {
		t.Fatalf("学员面应只出池内题 %d（created_by 不得收窄池）, got %v", poolQ.ID, stuIDs)
	}
}

// envelopeMessage 取 response.R 信封里的 message（只用于断言文案那一格）。
func envelopeMessage(t *testing.T, body []byte) string {
	t.Helper()
	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("解信封失败: %v（原文 %s）", err, string(body))
	}
	return env.Message
}
