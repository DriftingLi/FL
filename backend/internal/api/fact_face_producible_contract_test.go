// 契约测（ADR-0065 决策 7 + 决策 8「登记的档必须打得出」）：
// 本批给两张错误表加了条目，这里给**每一条**一条真实出口的行为例，并反向锁死「登记了却没人打得出」。
//
// 为什么两半都要：只断言「打得出」，漏登记的条目不会红（那是批④ 靠既有契约测抓到的那一半）；
// 只数「表里有几条」，登记一条永远到不了的哨兵照样绿。评审实测本批第一版就有四条这种死条目
// （ErrFavoriteNotFound / ErrCommentNotFound / ErrCommentNotOwned / ErrNoteNotFound 在当时的
// 可达路径上到不了）⇒ 反向那半是本锁存在的理由，不是装饰。
package api

import (
	"net/http"
	"strings"
	"testing"

	"forklift-training/internal/model"
	"forklift-training/internal/service"
	"forklift-training/internal/testutil"
)

// factFace 一条业务事实的可达出口：方法 + 路径 + body + 期望那句对外文案 + 该事实的哨兵。
type factFace struct {
	sent   error
	code   int // 该事实在本端点应落的那一档（400 = 输入不合法/业务事实；404 = 呈现层按不存在收口）
	method string
	path   string
	body   any
	want   string
}

func TestInteractionErrStatusFacesAreProducible(t *testing.T) {
	f := newPoolLeakFixture(t)
	// 他人的一条已存在的评论 ⇒ 「无权删除」这条必须有真实载体，否则它又会成为死条目。
	foreignUID := testutil.SeedStudent(t, f.db, "stuOtherCmt", "x").ID
	f.seedComment(f.poolQ.ID, foreignUID, "别人的评论")
	var foreign model.QuestionComment
	if err := f.db.Where("user_id = ? AND question_id = ?", foreignUID, f.poolQ.ID).
		First(&foreign).Error; err != nil {
		t.Fatalf("取回他人评论失败: %v", err)
	}
	hidden := 0
	for id := range f.hiddenByID() {
		hidden = id // 任一池外题：用来打「题目不存在」那一档
		break
	}

	assertFacesAgainstTable(t, f, "interactionErrStatus", interactionErrStatus, []factFace{
		{service.ErrQuestionNotFound, http.StatusNotFound, http.MethodGet, "/api/questions/" + qpath(hidden) + "/comments?page_size=10", nil, "题目不存在"},
		{service.ErrCommentContentEmpty, http.StatusBadRequest, http.MethodPost, "/api/questions/" + qpath(f.poolQ.ID) + "/comments", map[string]any{"content": "   "}, "评论内容不能为空"},
		{service.ErrCommentTooLong, http.StatusBadRequest, http.MethodPost, "/api/questions/" + qpath(f.poolQ.ID) + "/comments", map[string]any{"content": strings.Repeat("叉", 501)}, "评论不能超过500字"},
		{service.ErrCommentNotFound, http.StatusBadRequest, http.MethodDelete, "/api/questions/comments/999999", nil, "评论不存在"},
		{service.ErrCommentNotOwned, http.StatusBadRequest, http.MethodDelete, "/api/questions/comments/" + qpath(int(foreign.ID)), nil, "无权删除"},
		{service.ErrNoteContentEmpty, http.StatusBadRequest, http.MethodPut, "/api/questions/" + qpath(f.poolQ.ID) + "/note", map[string]any{"content": "  "}, "笔记内容不能为空"},
		{service.ErrNoteContentTooLong, http.StatusBadRequest, http.MethodPut, "/api/questions/" + qpath(f.poolQ.ID) + "/note", map[string]any{"content": strings.Repeat("记", 2001)}, "笔记不能超过2000字"},
	})
}

func TestFavoriteErrStatusFacesAreProducible(t *testing.T) {
	f := newPoolLeakFixture(t)
	spec := model.Specialty{Code: "favf", Name: "收藏面", SortOrder: 1, Status: 1}
	lv := model.CourseLevel{Code: "favf-lv", Name: "等级", SortOrder: 1, Status: 1}
	if err := f.db.Create(&spec).Error; err != nil {
		t.Fatalf("播种方向失败: %v", err)
	}
	if err := f.db.Create(&lv).Error; err != nil {
		t.Fatalf("播种等级失败: %v", err)
	}
	off := model.Course{Name: "下架课程", Status: 0, SpecialtyID: &spec.SpecialtyID, LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	if err := f.db.Create(&off).Error; err != nil {
		t.Fatalf("播种课程失败: %v", err)
	}
	// Course.Status 带 gorm default ⇒ 零值会被覆盖，需显式回写（与 poolLeak 夹具同法）。
	if err := f.db.Model(&model.Course{}).Where("course_id = ?", off.CourseID).Update("status", 0).Error; err != nil {
		t.Fatalf("置未发布失败: %v", err)
	}
	offCh := model.Chapter{CourseID: off.CourseID, Title: "下架章节", OrderNum: 1, CreatedAt: testutil.Now()}
	if err := f.db.Create(&offCh).Error; err != nil {
		t.Fatalf("播种章节失败: %v", err)
	}
	draftFeat := model.FeaturedContent{Title: "草稿精选", Category: "industry", Status: 0, Content: "正文", CreatedAt: testutil.Now()}
	if err := f.db.Create(&draftFeat).Error; err != nil {
		t.Fatalf("播种精选失败: %v", err)
	}

	assertFacesAgainstTable(t, f, "favoriteErrStatus", favoriteErrStatus, []factFace{
		{service.ErrFavTargetCourseRejected, http.StatusBadRequest, http.MethodPost, "/api/favorites", map[string]any{"target_type": "course", "target_id": off.CourseID}, "课程不存在或不可收藏"},
		{service.ErrFavTargetChapterRejected, http.StatusBadRequest, http.MethodPost, "/api/favorites", map[string]any{"target_type": "chapter", "target_id": offCh.ChapterID}, "章节不存在或不可收藏"},
		{service.ErrFavTargetQuestionRejected, http.StatusBadRequest, http.MethodPost, "/api/favorites", map[string]any{"target_type": "question", "target_id": 999999}, "题目不存在或不可收藏"},
		{service.ErrFavTargetFeaturedRejected, http.StatusBadRequest, http.MethodPost, "/api/favorites", map[string]any{"target_type": "featured", "target_id": draftFeat.ContentID}, "内容不存在或不可收藏"},
		{service.ErrFavTargetTopicNotFound, http.StatusBadRequest, http.MethodPost, "/api/favorites", map[string]any{"target_type": "topic", "target_id": 999999}, "帖子不存在"},
		{service.ErrFavTargetTypeUnsupported, http.StatusBadRequest, http.MethodPost, "/api/favorites", map[string]any{"target_type": "nope", "target_id": 1}, "收藏类型仅支持"},
	})
}

// assertFacesAgainstTable 两半同时成立：
//  1. 每条业务事实都经真实请求打出**自己那句**、且落 400（掉进 500 就是表里漏登记）；
//  2. 表里每个具名条目都必须被某条例证覆盖——登记一条到不了的哨兵即红。
//
// fallback 那一档不在射程内（它说的是「没有名字的错误」），由
// visible_by_id_fault_contract_test.go 注故障单独锁。
func assertFacesAgainstTable(t *testing.T, f *poolLeakFixture, tableName string, tbl *errStatusTable, faces []factFace) {
	t.Helper()
	reached := map[error]bool{}
	for _, fc := range faces {
		t.Run(fc.want, func(t *testing.T) {
			rec := doWithToken(t, f.r, f.studentToken, fc.method, fc.path, fc.body)
			if rec.Code != fc.code {
				t.Fatalf("业务事实被答成 %d（应 %d；答 500 说明表里漏登记、它被默认面吞了）：%s",
					rec.Code, fc.code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), fc.want) {
				t.Fatalf("应说「%s」：%s", fc.want, rec.Body.String())
			}
			reached[fc.sent] = true
		})
	}
	for _, e := range tbl.entries {
		if e.sentinel == nil || reached[e.sentinel] {
			continue
		}
		t.Errorf("%s 登记了 %q，但没有任何一条行为例打得出它——「登记的档必须打得出」（决策 8）",
			tableName, e.sentinel.Error())
	}
}

func TestNoteErrStatusFacesAreProducible(t *testing.T) {
	f := newPoolLeakFixture(t)
	own := model.Note{UserID: f.studentID, Content: "我自己的独立笔记", UpdatedAt: testutil.Now()}
	if err := f.db.Create(&own).Error; err != nil {
		t.Fatalf("播种笔记失败: %v", err)
	}

	assertFacesAgainstTable(t, f, "noteErrStatus", noteErrStatus, []factFace{
		{service.ErrNoteContentEmpty, http.StatusBadRequest, http.MethodPost, "/api/notes", map[string]any{"content": "   "}, "笔记内容不能为空"},
		{service.ErrNoteContentTooLong, http.StatusBadRequest, http.MethodPost, "/api/notes", map[string]any{"content": strings.Repeat("记", 2001)}, "笔记不能超过2000字"},
		{service.ErrNoteNotFound, http.StatusNotFound, http.MethodPut, "/api/notes/999999", map[string]any{"content": "改一点"}, "笔记不存在"},
		{service.ErrNoteNotFound, http.StatusNotFound, http.MethodDelete, "/api/notes/999999", nil, "笔记不存在"},
	})

	// 反向控制：同一批路径换成真实 id 就不再报「不存在」——否则上面四条 404 可能是恒真的。
	if code, body := doAndBody(t, f, f.studentToken, http.MethodDelete, "/api/notes/"+qpath(own.ID), nil); code != http.StatusOK {
		t.Fatalf("删自己那条笔记应 200，实际 %d：%s", code, body)
	}
}
