// 保形锁（ADR-0065 决策 4 步 1）：`POST /api/recruit/contact-requests` 的 handler 从裸闭包迁到
// Endpoint 缝，**这一步不许有任何行为变更**——所以断言的是响应**字节**，不是状态码。
//
// 为什么必须先有这条锁：步 2 要把「学员不存在」从 400 改成 404。没有字节锁，步 1 与步 2 混在
// 一次改动里，404 出现时就无法归因它是「缝迁移带出来的」还是「改判带出来的」（第十五波第④批
// 「只认一种漏法的锁等于没有锁」的同一条判据）。
//
// 期望字节取自**迁移前**的实测响应（旧 handler 对任何 err 都走 response.BadRequest）；
// 信封形状见 pkg/response.R：{code,message,data}，键序即结构体声明序。
package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// contactCreateSecret 夹具会话的签名密钥：步 2 的档位测要用同一枚签发指向不存在学员的会话，
// 所以它必须是常量而不是 newContactCreateEnv 里的局部字面量。
const contactCreateSecret = "contact-create-shape-secret"

// contactCreateEnv 是本表全部用例共用的真实链路：路由 + 两家企业（其一已被禁用）+ 学员面。
type contactCreateEnv struct {
	router       *gin.Engine
	db           *gorm.DB
	studentID    int
	missingID    int
	recruiterID  int
	recruiterTok string
	disabledTok  string
}

func newContactCreateEnv(t *testing.T) *contactCreateEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:          "contact-create-shape-secret",
		JWTExpiresHours:       2,
		JWTRefreshExpiresDays: 7,
		AuthCookie:            config.AuthCookieConfig{Name: "hrwai_token", Domain: "example.com", Secure: false},
		RecruiterCookie:       config.RecruiterCookieConfig{Name: "recruiter_token", Domain: "", Secure: false},
	}
	r := NewRouter(newContractDeps(t, db, cfg))

	pwd, _ := service.HashPassword("pass1234")
	stu := testutil.SeedStudent(t, db, "stuCreateShape", pwd)

	adminPwd, _ := service.HashPassword("admin123")
	admin := testutil.SeedAdmin(t, db, "adminCreateShape", adminPwd)
	adminSess := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{
		Name: cfg.AuthCookie.Name, Domain: cfg.AuthCookie.Domain, Secure: cfg.AuthCookie.Secure})
	adminToken, _ := adminSess.Issue(admin.AdminID, admin.Username, "admin")

	// 登录必须在禁用之前：管理员禁用一家企业会同时吊销其会话（处置动作后果齐全，ADR-0064 第①批）。
	mkRecruiter := func(username, company string) (int, string) {
		rec := doWithToken(t, r, adminToken, http.MethodPost, "/api/admin/recruiters", map[string]any{
			"username": username, "password": "recruit123", "company_name": company,
			"credit_code": "91110000MA" + username, "business_scope": "叉车维修",
			"contact_name": "联系人", "contact_phone": "13800004444",
			"contact_email": username + "@example.com",
		})
		if rec.Code != http.StatusCreated {
			t.Fatalf("建招聘者 %s 失败 %d %s", username, rec.Code, rec.Body.String())
		}
		var created recruiterCreateResp
		if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
			t.Fatalf("解析创建响应失败: %v", err)
		}
		login := doJSON(t, r, http.MethodPost, "/api/auth/recruiter-login",
			map[string]any{"username": username, "password": "recruit123"})
		if login.Code != http.StatusOK {
			t.Fatalf("招聘者 %s 登录失败 %d %s", username, login.Code, login.Body.String())
		}
		var lb loginResp
		if err := json.Unmarshal(login.Body.Bytes(), &lb); err != nil {
			t.Fatalf("解析登录响应失败: %v", err)
		}
		return created.Data.ID, lb.Data.Token
	}
	recID, recTok := mkRecruiter("recruitCreateShape", "保形测试企业")
	disID, disTok := mkRecruiter("recruitCreateGone", "被禁用企业")
	if err := db.Model(&model.RecruiterUser{}).Where("id = ?", disID).Update("status", 0).Error; err != nil {
		t.Fatalf("禁用招聘者失败: %v", err)
	}

	return &contactCreateEnv{
		router: r, db: db, studentID: stu.ID, missingID: 999999,
		recruiterID: recID, recruiterTok: recTok, disabledTok: disTok,
	}
}

// postCreate 打一次发起接口并把 body 原样序列化（不做 map 归一，以便构造「非对象」那格）。
func (e *contactCreateEnv) postCreate(t *testing.T, tok string, body any) string {
	t.Helper()
	return doWithToken(t, e.router, tok, http.MethodPost, "/api/recruit/contact-requests", body).Body.String()
}

// contactBody 拼出失败信封的确切字节（data 恒 null）。
func contactBody(code int, msg string) string {
	return `{"code":` + strconv.Itoa(code) + `,"message":` + string(mustJSON(msg)) + `,"data":null}`
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// TestContactCreateFace_BytesPreservedAcrossSeamMigration 是步 1 的保形锁。
//
// 两格**本锁未覆盖**与原因（写成空白比假装全绿好）：
//   - 「今日申请已达上限」：要打它得先建 21 个学员（dailyLimit 是 ContactService 的私有字段，
//     api 包改不了）。它由 TestContactContract_FullFlow 覆盖——事实也正是那里红的第一格：
//     我第一次装配表时漏了这条具名哨兵，它掉进 500 默认面，那条既有契约测当场判红。
//   - 「查不动」（DB 故障）：旧 handler 把它咽成 400 + **驱动原文**，字节随驱动版本而变，写不成
//     字面量。这一格正是步 2 要改判的对象，由 contact_create_fact_tier_test.go 接管。
func TestContactCreateFace_BytesPreservedAcrossSeamMigration(t *testing.T) {
	e := newContactCreateEnv(t)
	long := strings.Repeat("叉", 201)

	t.Run("输入不合法与业务事实：一律 400 + 该事实自己那句", func(t *testing.T) {
		cooldownStu := testutil.SeedStudent(t, e.db, "stuCreateCooldown", "x")
		now := time.Now()
		if err := e.db.Create(&model.ContactRequest{
			RecruiterID: e.recruiterID, StudentUserID: cooldownStu.ID, Message: "旧申请",
			Status:    string(service.ContactGrantRejected),
			CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now, DecidedAt: &now,
		}).Error; err != nil {
			t.Fatalf("播种被拒授权失败: %v", err)
		}
		// 冷却判定读的是 decided_at，播种必须存在于发起之前，故放在表之前而不是用例内。

		for _, tc := range []struct {
			name string
			tok  string
			body map[string]any
			want string
		}{
			{"空附言", e.recruiterTok, map[string]any{"student_user_id": e.studentID, "message": ""}, contactBody(400, "附言不能为空")},
			{"纯空白附言", e.recruiterTok, map[string]any{"student_user_id": e.studentID, "message": "   "}, contactBody(400, "附言不能为空")},
			{"超长附言", e.recruiterTok, map[string]any{"student_user_id": e.studentID, "message": long}, contactBody(400, "附言不能超过 200 字")},
			{"学员 id 为 0", e.recruiterTok, map[string]any{"student_user_id": 0, "message": "想聊聊岗位"}, contactBody(400, "参数错误")},
			{"学员 id 为负", e.recruiterTok, map[string]any{"student_user_id": -1, "message": "想聊聊岗位"}, contactBody(400, "参数错误")},
			{"学员不存在", e.recruiterTok, map[string]any{"student_user_id": e.missingID, "message": "想聊聊岗位"}, contactBody(400, "学员不存在")},
			{"招聘者已禁用", e.disabledTok, map[string]any{"student_user_id": e.studentID, "message": "想聊聊岗位"}, contactBody(400, "招聘者账号已禁用")},
			{"冷却期内", e.recruiterTok, map[string]any{"student_user_id": cooldownStu.ID, "message": "再试一次"}, contactBody(400, "该学员 30 天内拒绝或撤回过申请，冷却期内不能重复申请")},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if got := e.postCreate(t, tc.tok, tc.body); got != tc.want {
					t.Fatalf("响应字节变了\n 期望 %s\n 实际 %s", tc.want, got)
				}
			})
		}
	})

	t.Run("绑定失败：400 + 固定那句", func(t *testing.T) {
		rec := doWithToken(t, e.router, e.recruiterTok, http.MethodPost,
			"/api/recruit/contact-requests", json.RawMessage(`[1,2,3]`))
		want := contactBody(400, "请求参数错误")
		if got := rec.Body.String(); got != want {
			t.Fatalf("响应字节变了\n 期望 %s\n 实际 %s", want, got)
		}
	})

	t.Run("pending 唯一：400 + 已存在那句", func(t *testing.T) {
		stu := testutil.SeedStudent(t, e.db, "stuCreatePending", "x")
		body := map[string]any{"student_user_id": stu.ID, "message": "第一次申请"}
		first := e.postCreate(t, e.recruiterTok, body)
		if !strings.HasPrefix(first, `{"code":201,"message":"申请已提交"`) {
			t.Fatalf("夹具没建出 pending，后面的断言无意义：%s", first)
		}
		want := contactBody(400, "已存在待处理的申请")
		if got := e.postCreate(t, e.recruiterTok, body); got != want {
			t.Fatalf("响应字节变了\n 期望 %s\n 实际 %s", want, got)
		}
	})

	t.Run("成功面：201 + 申请 DTO", func(t *testing.T) {
		stu := testutil.SeedStudent(t, e.db, "stuCreateOk", "x")
		got := e.postCreate(t, e.recruiterTok, map[string]any{"student_user_id": stu.ID, "message": "聊聊岗位"})
		if !strings.HasPrefix(got, `{"code":201,"message":"申请已提交","data":{`) {
			t.Fatalf("成功面字节变了：%s", got)
		}
		if !strings.Contains(got, `"student_user_id":`+strconv.Itoa(stu.ID)) ||
			!strings.Contains(got, `"status":"pending"`) {
			t.Fatalf("成功面 DTO 形状变了：%s", got)
		}
	})
}
