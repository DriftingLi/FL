// 学员可读面行为台账（ADR-0062 票4）。
//
// 为什么是行为台账而不是文本锁：ADR-0047 §1 选「扫蓝图函数体里有没有出现过 CapabilityRequired」
// 的理由是 gin 不在路由上暴露中间件链——那个理由仍然成立，但它导致「同一蓝图里别的路由挂了守卫，
// 就带着没挂的那条一起过关」（#1246 波次的实测现场：/api/question-bank/questions 整库漏答案）。
// 本台账不问源码长什么样，只问一件事：**拿学员 token 打这条 GET 路由，会不会拿到 2xx**。
// 会 ⇒ 必须在下面逐条登记理由；没登记当场红。新增一条学员能读的管理面从「没人注意到」变成「当场判红」。
package api

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/testutil"
)

// studentReadableSurface 学员可读面白名单：path 模板 → 理由。
// 判据取自 CONTEXT.md「学员可读面」：默认不可读，进入需显式登记（与票8a 的公开页清单同一形态）。
// 首登记的 55 条来自空台账跑出来的一次事实核对，不是凭印象列的——其中
// /api/question-bank/questions 是本轮抓到的真实漏口，按 #981 式能力分流收编（见该条理由）。
var studentReadableSurface = map[string]string{
	// ===== 无主体的公开面（访客即可读，学员 token 自然也能读）=====
	"/api":                     "服务索引（router.go 的根路由）",
	"/api/health/live":         "存活探针，部署侧在用",
	"/api/courses":             "课程列表：发现面，公开（内容受门禁）",
	"/api/featured-contents":   "内容精选公开列表（门户同源）",
	"/api/faq":                 "帮助中心学员面，一次返回分类与已发布条目",
	"/api/catalog/tree":        "课程目录树（学员端导航）",
	"/api/levels":              "课程等级字典（学员端筛选器）",
	"/api/tags":                "题库标签字典（学员端标签练习入口；计数走池口径）",
	"/api/positions":           "岗位字典（学员简历「期望岗位」与招聘域共用同一字典）",
	"/api/credentials":         "目标证件字典（学员端选择器）",
	"/api/credentials/grouped": "目标证件分组字典（同上）",
	"/api/materials":           "学习资料（课程附件聚合视图，公开浏览面）",

	// ===== 学员本人数据面（按 user_id 收口，越权按不存在）=====
	"/api/auth/me":                          "本人账号信息",
	"/api/me/credential":                    "本人当前证件",
	"/api/contributions/mine":               "我的投稿（全部状态）",
	"/api/student/materials":                "我可见的学习资料列表",
	"/api/student/profile":                  "我的资料",
	"/api/student/records":                  "我的学习记录",
	"/api/student/study-stats":              "我的学习统计",
	"/api/student/courses":                  "我的课程",
	"/api/notes":                            "我的笔记（私有）",
	"/api/favorites":                        "我的收藏",
	"/api/wrong-questions":                  "我的错题本",
	"/api/wrong-questions/stats":            "我的错题统计",
	"/api/wrong-questions/export":           "我的错题导出",
	"/api/notifications":                    "我的站内信",
	"/api/notifications/unread-count":       "我的未读数",
	"/api/points/balance":                   "我的积分余额",
	"/api/points/ledger":                    "我的积分流水",
	"/api/points/tasks":                     "我的任务中心",
	"/api/check-in/rank":                    "打卡排行榜（含我的名次）",
	"/api/mock-exam/history":                "我的模考历史",
	"/api/real-exam/papers":                 "我的套卷列表（带 entitled）",
	"/api/resume/applications":              "我的投递",
	"/api/resume/contact-requests":          "我收到的交换申请",
	"/api/resume/view-stats":                "我的简历查看聚合数",
	"/api/questions/:question_id/knowledge": "题目考点（题目详情侧栏，池外题按不存在）",

	// ===== 学员工作区功能面 =====
	"/api/ai-assistant/models":               "AI 助手可选模型列表",
	"/api/ai-assistant/modes":                "AI 助手模式列表",
	"/api/ai-assistant/sessions":             "我的 AI 会话列表",
	"/api/ai-assistant/user-models":          "我的自定义模型",
	"/api/forum/topics":                      "论坛主题列表（公开内容面）",
	"/api/forum/my-topics":                   "我发的主题",
	"/api/forum/my-replies":                  "我的回复",
	"/api/forum/my-liked-topics":             "我赞过的主题",
	"/api/forum/my-observed":                 "我围观的主题",
	"/api/forum/my-view-history":             "我的浏览记录",
	"/api/jobs":                              "职位广场（学员视角，带 apply_state）",
	"/api/practice-mode/stats":               "我的练习统计",
	"/api/practice-mode/practice-stats":      "我的刷题统计",
	"/api/practice-mode/history":             "我的练习历史",
	"/api/practice-mode/progress":            "我的练习进度",
	"/api/practice-mode/sequential-progress": "我的顺序练习进度（NULL 桶口径）",
	"/api/question-bank/stats":               "题库池计数（学员题目页与练习入口共用）",
	// 该端点是本轮台账抓到的真实漏口：收紧前完全不查池且带答案，学员 token 可整库翻走
	// draft/pending/源标记真题题。收法取 #981 同形——按能力分流（编辑面全量、学员面过池），
	// 而不是硬挂能力守卫：Web 侧确实只有 admin/tutor 消费它，但移动端 practice.uts 是学员消费者，
	// 403 会直接打断它（若日后裁定「移动端迁到 practice-mode 族」，此处改为挂守卫）。
	"/api/question-bank/questions": "题库列表：按能力分流（讲师/管理端 = 编辑面全量，学员 = 池口径）",
}

var ledgerParam = regexp.MustCompile(`:[A-Za-z0-9_]+|\*[A-Za-z0-9_]+`)

// TestStudentReadableSurfaceLedger 用学员 token 打全量 GET 路由，凡 2xx 者必须已登记。
func TestStudentReadableSurfaceLedger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "student-readable-ledger-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token := failureTestStudentToken(t, db, cfg, "readable_ledger_stu")

	var hits []string
	for _, rt := range r.Routes() {
		if rt.Method != http.MethodGet || !strings.HasPrefix(rt.Path, "/api") {
			continue
		}
		probe := ledgerParam.ReplaceAllString(rt.Path, "1")
		if strings.Contains(probe, "*") {
			continue // 通配段无法用单段样本打准
		}
		rec := doWithToken(t, r, token, http.MethodGet, probe, nil)
		if rec.Code >= http.StatusOK && rec.Code < http.StatusMultipleChoices {
			hits = append(hits, rt.Path)
		}
	}

	for _, p := range hits {
		if _, ok := studentReadableSurface[p]; !ok {
			t.Errorf("学员可读面台账未登记：%s 用学员 token 返回了 2xx —— 要么它本就是学员面（登记进 studentReadableSurface 并写理由），要么它漏挂了守卫", p)
		}
	}
	for p := range studentReadableSurface {
		if !containsPath(hits, p) {
			t.Errorf("台账里登记的 %s 本轮没有返回 2xx —— 判据变了还是路由改名了？请核对后删除或修复", p)
		}
	}
	t.Logf("学员可读面：全量 GET 路由中 2xx 命中 %d 条，均已逐条登记", len(hits))
}

func containsPath(hits []string, p string) bool {
	for _, h := range hits {
		if h == p {
			return true
		}
	}
	return false
}
