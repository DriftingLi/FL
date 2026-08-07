// Package service 实现业务服务层。
package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// ChapterGenResult 单个章节生成结果。
type ChapterGenResult struct {
	ChapterID int    `json:"chapter_id"`
	Title     string `json:"title"`
	Status    string `json:"status"` // "success" | "failed"
	Content   string `json:"content,omitempty"`
	Error     string `json:"error,omitempty"`
}

// GenTaskStatus 生成任务状态快照（前端轮询返回结构）。
type GenTaskStatus struct {
	TaskID    string             `json:"task_id"`
	Status    string             `json:"status"` // "pending"|"processing"|"completed"|"failed"
	Total     int                `json:"total"`
	Completed int                `json:"completed"`
	Results   []ChapterGenResult `json:"results"`
}

// genTaskPayload async_task.payload 的结构。
type genTaskPayload struct {
	CourseID   int   `json:"course_id"`
	ChapterIDs []int `json:"chapter_ids"`
	UserID     int   `json:"user_id"`
}

// genTaskResult async_task.result 的结构（随生成进度更新）。
type genTaskResult struct {
	Total     int                `json:"total"`
	Completed int                `json:"completed"`
	Results   []ChapterGenResult `json:"results"`
}

// ContentGenerateService 课程内容异步生成服务。
type ContentGenerateService struct {
	db *gorm.DB
	ai *AIService
}

// NewContentGenerateService 构造 ContentGenerateService。
func NewContentGenerateService(db *gorm.DB, ai *AIService) *ContentGenerateService {
	return &ContentGenerateService{db: db, ai: ai}
}

// StartGeneration 启动异步生成任务，返回 task_id（字符串形式）。
// 校验通过后创建 async_task 记录并启动 goroutine 后台执行 runGeneration。
func (s *ContentGenerateService) StartGeneration(courseID int, chapterIDs []int, userID int) (string, error) {
	// 1. 校验课程存在
	var course model.Course
	if err := s.db.Select("course_id").First(&course, courseID).Error; err != nil {
		return "", fmt.Errorf("课程不存在: %w", err)
	}

	// 2. 校验章节存在且属于该课程
	var count int64
	if err := s.db.Model(&model.Chapter{}).
		Where("chapter_id IN ? AND course_id = ?", chapterIDs, courseID).
		Count(&count).Error; err != nil {
		return "", fmt.Errorf("校验章节失败: %w", err)
	}
	if int(count) != len(chapterIDs) {
		return "", fmt.Errorf("部分章节不存在或不属于该课程（期望 %d 个，实际 %d 个）", len(chapterIDs), count)
	}

	// 3. 创建 async_task 记录（status=pending）
	payload, _ := json.Marshal(genTaskPayload{
		CourseID:   courseID,
		ChapterIDs: chapterIDs,
		UserID:     userID,
	})
	task := model.AsyncTask{
		TaskType: "chapter_content_generate",
		Status:   "pending",
		Payload:  model.JSONB(payload),
	}
	if err := s.db.Create(&task).Error; err != nil {
		return "", fmt.Errorf("创建任务失败: %w", err)
	}
	taskID := fmt.Sprintf("%d", task.ID)

	// 4. 启动后台 goroutine
	go s.runGeneration(task.ID, courseID, chapterIDs, userID)

	return taskID, nil
}

// runGeneration 后台执行生成任务。每完成 1 个章节就更新 async_task.result。
func (s *ContentGenerateService) runGeneration(taskID int, courseID int, chapterIDs []int, userID int) {
	// 更新状态为 processing
	if err := s.db.Model(&model.AsyncTask{}).Where("id = ?", taskID).
		Update("status", "processing").Error; err != nil {
		slog.Error("runGeneration 更新 processing 状态失败", "task_id", taskID, "error", err)
		return
	}

	// 查询课程信息
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		s.failTask(taskID, fmt.Sprintf("查询课程失败: %v", err))
		return
	}

	// 旧分类已退役：AI 生成的「课程分类」上下文改用专业方向名称
	classifyCtx := ""
	if course.SpecialtyID != nil {
		var spec model.Specialty
		if err := s.db.First(&spec, *course.SpecialtyID).Error; err == nil {
			classifyCtx = spec.Name
		}
	}

	// 查询所有章节标题
	var chapters []model.Chapter
	if err := s.db.Where("chapter_id IN ?", chapterIDs).Find(&chapters).Error; err != nil {
		s.failTask(taskID, fmt.Sprintf("查询章节失败: %v", err))
		return
	}
	chapterMap := make(map[int]string, len(chapters))
	for _, ch := range chapters {
		chapterMap[ch.ChapterID] = ch.Title
	}

	// 初始化 result
	result := genTaskResult{
		Total:     len(chapterIDs),
		Completed: 0,
		Results:   make([]ChapterGenResult, 0, len(chapterIDs)),
	}
	// 先写入初始 result（total 已确定，completed=0）
	s.updateTaskResult(taskID, &result)

	userIDPtr := &userID
	// 逐章节生成
	for _, chID := range chapterIDs {
		title := chapterMap[chID]
		genResult := ChapterGenResult{ChapterID: chID, Title: title}

		content, err := s.ai.GenerateChapterContent(course.Name, classifyCtx, course.Description, title, userIDPtr)
		if err != nil {
			genResult.Status = "failed"
			genResult.Error = err.Error()
			slog.Warn("章节内容生成失败", "task_id", taskID, "chapter_id", chID, "error", err)
		} else {
			// 写入 chapter.content
			if err := s.db.Model(&model.Chapter{}).Where("chapter_id = ?", chID).
				Update("content", content).Error; err != nil {
				genResult.Status = "failed"
				genResult.Error = fmt.Sprintf("写入数据库失败: %v", err)
				slog.Error("写入 chapter.content 失败", "chapter_id", chID, "error", err)
			} else {
				genResult.Status = "success"
				genResult.Content = content
			}
		}

		result.Results = append(result.Results, genResult)
		result.Completed++
		s.updateTaskResult(taskID, &result)
	}

	// 全部完成（即使部分失败也标记 completed，由 results 区分）
	finalStatus := "completed"
	// 若所有章节都失败，标记 failed
	allFailed := true
	for _, r := range result.Results {
		if r.Status == "success" {
			allFailed = false
			break
		}
	}
	if allFailed && result.Total > 0 {
		finalStatus = "failed"
	}

	if err := s.db.Model(&model.AsyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":     finalStatus,
			"updated_at": time.Now(),
		}).Error; err != nil {
		slog.Error("更新任务最终状态失败", "task_id", taskID, "error", err)
	}
}

// updateTaskResult 更新 async_task.result JSONB 字段。
func (s *ContentGenerateService) updateTaskResult(taskID int, result *genTaskResult) {
	resultJSON, _ := json.Marshal(result)
	if err := s.db.Model(&model.AsyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"result":     model.JSONB(resultJSON),
			"updated_at": time.Now(),
		}).Error; err != nil {
		slog.Error("更新任务 result 失败", "task_id", taskID, "error", err)
	}
}

// failTask 标记任务为 failed 并写入 error 字段。
func (s *ContentGenerateService) failTask(taskID int, errMsg string) {
	if err := s.db.Model(&model.AsyncTask{}).Where("id = ?", taskID).
		Updates(map[string]any{
			"status":     "failed",
			"error":      errMsg,
			"updated_at": time.Now(),
		}).Error; err != nil {
		slog.Error("failTask 更新失败", "task_id", taskID, "error", err)
	}
}

// CleanupInterruptedTasks 服务启动时把上次进程遗留的 pending/processing 任务标记为 failed。
// 进程重启时运行中的 goroutine 直接丢失，DB 里的任务若不清理会永远停在「生成中」，
// 前端轮询会一直显示生成中。此函数在服务启动阶段调用一次。
func (s *ContentGenerateService) CleanupInterruptedTasks() {
	res := s.db.Model(&model.AsyncTask{}).
		Where("status IN ?", []string{"pending", "processing"}).
		Updates(map[string]any{
			"status":     "failed",
			"error":      "服务重启，任务中断",
			"updated_at": time.Now(),
		})
	if res.Error != nil {
		slog.Error("清理遗留异步任务失败", "error", res.Error)
		return
	}
	if res.RowsAffected > 0 {
		slog.Warn("已清理服务重启遗留的异步任务", "count", res.RowsAffected)
	}
}

// GetTaskStatus 查询任务状态。返回前端轮询所需的 GenTaskStatus 结构。
func (s *ContentGenerateService) GetTaskStatus(taskID string) (*GenTaskStatus, error) {
	var task model.AsyncTask
	// taskID 为字符串形式，需解析为 int
	var id int
	if _, err := fmt.Sscanf(taskID, "%d", &id); err != nil {
		return nil, fmt.Errorf("无效的 task_id: %s", taskID)
	}
	if err := s.db.First(&task, id).Error; err != nil {
		return nil, fmt.Errorf("任务不存在: %w", err)
	}

	status := &GenTaskStatus{
		TaskID: taskID,
		Status: task.Status,
	}

	// 解析 result JSONB
	if len(task.Result) > 0 {
		var r genTaskResult
		if err := json.Unmarshal(task.Result, &r); err == nil {
			status.Total = r.Total
			status.Completed = r.Completed
			status.Results = r.Results
		}
	}

	// 若 result 为空但任务已完成，从 payload 解析 total
	if status.Total == 0 && len(task.Payload) > 0 {
		var p genTaskPayload
		if err := json.Unmarshal(task.Payload, &p); err == nil {
			status.Total = len(p.ChapterIDs)
		}
	}

	return status, nil
}
