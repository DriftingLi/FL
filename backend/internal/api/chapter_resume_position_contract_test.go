// 契约测（ADR-0062 决策 11）：断点续播位置由后端在章节详情下发。
// 旧形状是前端在章节详情里同步读「另一条并发请求填的 map」，谁先回来全凭运气 ⇒ 多半读到 0，
// 从课程列表点进看到一半的章节每次都从片头重播（词表「学习位置」承诺失效）。
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

func TestChapterDetailProjectsResumePosition(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{
		JWTSecretKey:    "resume-pos-secret",
		JWTExpiresHours: 2,
		AuthCookie:      config.AuthCookieConfig{Name: "hrwai_token"},
	}
	r := NewRouter(newContractDeps(t, db, cfg))
	token := failureTestStudentToken(t, db, cfg, "resume_pos_stu")
	var studentID int
	if err := db.Model(&model.HrwaiUser{}).Where("username = ?", "resume_pos_stu").
		Select("id").Row().Scan(&studentID); err != nil {
		t.Fatalf("查学员失败: %v", err)
	}

	spec := model.Specialty{Code: "pos", Name: "位置方向", SortOrder: 1, Status: 1}
	lv := model.CourseLevel{Code: "pos-lv", Name: "入门", SortOrder: 1, Status: 1}
	course := model.Course{Name: "液压系统", Status: 1, SpecialtyID: &spec.SpecialtyID,
		LevelID: &lv.LevelID, CreatedAt: testutil.Now()}
	ch := model.Chapter{Title: "油泵拆装", OrderNum: 1, CreatedAt: testutil.Now()}
	rec := model.StudyRecord{StudentID: studentID, VideoPosition: 823}
	for _, row := range []any{&spec, &lv, &course, &ch, &rec} {
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("播种失败: %v", err)
		}
	}
	if err := db.Model(&model.Chapter{}).Where("chapter_id = ?", ch.ChapterID).
		Update("course_id", course.CourseID).Error; err != nil {
		t.Fatalf("挂章节到课程失败: %v", err)
	}
	if err := db.Model(&model.StudyRecord{}).Where("record_id = ?", rec.RecordID).
		Updates(map[string]any{"course_id": course.CourseID, "chapter_id": ch.ChapterID}).Error; err != nil {
		t.Fatalf("关联学习记录失败: %v", err)
	}

	url := fmt.Sprintf("/api/course/%d/chapter/%d", course.CourseID, ch.ChapterID)
	w := doWithToken(t, r, token, http.MethodGet, url, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("章节详情应 200, got %d %s", w.Code, w.Body.String())
	}
	var env struct {
		Data struct {
			ResumePosition int `json:"resume_position"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("解析响应失败: %v body=%s", err, w.Body.String())
	}
	if env.Data.ResumePosition != 823 {
		t.Fatalf("resume_position 应回学习记录里的 823, got %d（%s）", env.Data.ResumePosition, w.Body.String())
	}

	// 查不动不得被伪装成「位置 0」（票6 同判据）：删表后请求必须失败，而不是 200 回一个从头播的位置。
	if err := db.Migrator().DropTable(&model.StudyRecord{}); err != nil {
		t.Fatalf("注入故障（删学习记录表）失败: %v", err)
	}
	if w := doWithToken(t, r, token, http.MethodGet, url, nil); w.Code == http.StatusOK {
		t.Fatalf("学习位置查不动时不得返回 200 伪零值: %s", w.Body.String())
	}
}
