// Package service 学员侧课程与章节。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// CourseService 学员课程服务。
type CourseService struct {
	db          *gorm.DB
	fileService *FileService
}

// NewCourseService 创建课程服务实例。
func NewCourseService(db *gorm.DB, fileService *FileService) *CourseService {
	return &CourseService{db: db, fileService: fileService}
}

// GetCourses 课程列表（可额外按专业方向/课程等级过滤）。
// 未挂专业方向/等级的课程不展示（与目录树口径统一）。
func (s *CourseService) GetCourses(page, pageSize int, specialtyID, levelID *int) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 12
	}
	q := s.db.Model(&model.Course{}).
		Where("status = ? AND specialty_id IS NOT NULL AND level_id IS NOT NULL", 1)
	if specialtyID != nil {
		q = q.Where("specialty_id = ?", *specialtyID)
	}
	if levelID != nil {
		q = q.Where("level_id = ?", *levelID)
	}
	var total int64
	q.Count(&total)
	var courses []model.Course
	q.Order("sort_order ASC, created_at DESC, course_id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&courses)
	// 证书模板数量少，一次加载映射（避免逐课程 N+1）
	var certs []model.CertificateTemplate
	s.db.Find(&certs)
	certNameByID := make(map[int]string, len(certs))
	for i := range certs {
		certNameByID[certs[i].ID] = certs[i].Name
	}
	items := make([]map[string]any, 0, len(courses))
	for i := range courses {
		item := courseToDict(&courses[i])
		fillChapterCount(s.db, courses[i].CourseID, item)
		fillPrereqIDs(s.db, courses[i].CourseID, item)
		if id := courses[i].CertificateTemplateID; id != nil {
			item["certificate_name"] = certNameByID[*id]
		}
		items = append(items, item)
	}
	pages := int((total + int64(pageSize) - 1) / int64(pageSize))
	return map[string]any{
		"total":   total,
		"page":    page,
		"pages":   pages,
		"courses": items,
	}
}

// GetCourseDetail 课程详情。
func (s *CourseService) GetCourseDetail(courseID, studentID int) (map[string]any, error) {
	var course model.Course
	if err := s.db.First(&course, courseID).Error; err != nil {
		return nil, errors.New("课程不存在")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)
	chapterList := make([]map[string]any, 0, len(chapters))
	for i := range chapters {
		chapterList = append(chapterList, chapterToDict(&chapters[i]))
	}
	progress := 0.0
	if studentID > 0 {
		var record model.StudyRecord
		// 优先取课程级记录（chapter_id IS NULL）；历史数据没有 NULL 记录时回退到任意一条记录。
		if err := s.db.Where("student_id = ? AND course_id = ? AND chapter_id IS NULL", studentID, courseID).
			Order("record_id ASC").Limit(1).Find(&record).Error; err == nil && record.RecordID == 0 {
			s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).
				Order("record_id ASC").Limit(1).Find(&record)
		}
		if record.RecordID > 0 {
			progress = record.Progress
		}
	}
	detail := courseToDict(&course)
	fillChapterCount(s.db, course.CourseID, detail)
	fillCourseMeta(s.db, &course, detail)
	return map[string]any{
		"course_info": detail,
		"chapters":    chapterList,
		"progress":    progress,
	}, nil
}

// GetChapterDetail 章节详情。
func (s *CourseService) GetChapterDetail(courseID, chapterID, studentID int) (map[string]any, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	if chapter.CourseID != courseID {
		return nil, errors.New("章节不属于该课程")
	}
	var chapters []model.Chapter
	s.db.Where("course_id = ?", courseID).Order("order_num").Find(&chapters)

	prevID, nextID := 0, 0
	for i, ch := range chapters {
		if ch.ChapterID == chapterID {
			if i > 0 {
				prevID = chapters[i-1].ChapterID
			}
			if i < len(chapters)-1 {
				nextID = chapters[i+1].ChapterID
			}
			break
		}
	}

	studyStatus := "not_started"
	if studentID > 0 {
		var record model.StudyRecord
		// 同上：使用 Limit(1).Find() 避免首次访问章节时 GORM 误报 record not found。
		if err := s.db.Where("student_id = ? AND course_id = ? AND chapter_id = ?", studentID, courseID, chapterID).Limit(1).Find(&record).Error; err == nil && record.RecordID > 0 {
			if record.Progress >= 100 {
				studyStatus = "completed"
			} else {
				studyStatus = "studying"
			}
		}
	}

	result := chapterToDict(&chapter)
	if prevID != 0 {
		result["previous_chapter_id"] = prevID
	} else {
		result["previous_chapter_id"] = nil
	}
	if nextID != 0 {
		result["next_chapter_id"] = nextID
	} else {
		result["next_chapter_id"] = nil
	}
	result["study_status"] = studyStatus

	var files []model.ChapterFile
	s.db.Where("chapter_id = ?", chapterID).Order("created_at").Find(&files)
	fileList := make([]map[string]any, 0, len(files))
	if len(files) == 0 && chapter.FileURL != "" {
		fileList = append(fileList, legacyFileEntry(&chapter))
	} else {
		for i := range files {
			fileList = append(fileList, chapterFileToDict(&files[i]))
		}
	}
	result["files"] = fileList
	return result, nil
}

// GetChapterSlides 章节幻灯片。
// 优先读取 DB 中持久化的 slide_urls；为空则从 PPT 文件下载并触发转图，
// 转图成功后把 URL 列表回写 chapter.slide_urls。
func (s *CourseService) GetChapterSlides(chapterID int) (map[string]any, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}

	// 1. 优先读 DB 持久化的 slide_urls
	if chapter.SlideUrls != "" {
		var urls []string
		if err := json.Unmarshal([]byte(chapter.SlideUrls), &urls); err == nil && len(urls) > 0 {
			return map[string]any{"chapter_id": chapterID, "slides": urls}, nil
		}
	}

	// 2. 查找 PPT 文件 URL
	pptURL := resolveChapterPPTURL(s.db, &chapter, chapterID)
	if pptURL == "" {
		return map[string]any{"chapter_id": chapterID, "slides": []string{}}, nil
	}

	// 3. 下载 PPT 并转图
	slideURLs := s.generateSlides(chapterID, pptURL)
	return map[string]any{"chapter_id": chapterID, "slides": slideURLs}, nil
}

// RegenerateChapterSlides 重新生成幻灯片。
// 总是重新下载 PPT 并转图，覆盖 chapter.slide_urls。
func (s *CourseService) RegenerateChapterSlides(chapterID int) (map[string]any, error) {
	var chapter model.Chapter
	if err := s.db.First(&chapter, chapterID).Error; err != nil {
		return nil, errors.New("章节不存在")
	}
	pptURL := resolveChapterPPTURL(s.db, &chapter, chapterID)
	if pptURL == "" {
		return nil, errors.New("该章节没有PPT文件")
	}
	slideURLs := s.generateSlides(chapterID, pptURL)
	if len(slideURLs) == 0 {
		return nil, errors.New("PPT转图失败，请检查文件是否损坏")
	}
	return map[string]any{"chapter_id": chapterID, "slides": slideURLs}, nil
}

// generateSlides 下载 PPT bytes 并调 FileService 转图，把 URL 列表持久化到 chapter.slide_urls。
func (s *CourseService) generateSlides(chapterID int, pptURL string) []string {
	if s.fileService == nil {
		return nil
	}
	pptBytes, err := downloadFile(pptURL)
	if err != nil {
		slog.Error("下载 PPT 失败", "url", pptURL, "error", err)
		return nil
	}
	slideURLs := s.fileService.ConvertPPTToImages(pptBytes, chapterID)
	if len(slideURLs) > 0 {
		slideURLsJSON, _ := json.Marshal(slideURLs)
		s.db.Model(&model.Chapter{}).Where("chapter_id = ?", chapterID).Update("slide_urls", string(slideURLsJSON))
	}
	return slideURLs
}

// resolveChapterPPTURL 查找章节的 PPT 文件 URL（优先 chapter_file 表，其次 chapter.file_url）。
func resolveChapterPPTURL(db *gorm.DB, chapter *model.Chapter, chapterID int) string {
	var pptFile model.ChapterFile
	if err := db.Where("chapter_id = ? AND content_type = ?", chapterID, "ppt").First(&pptFile).Error; err == nil {
		return pptFile.FileURL
	}
	if chapter.ContentType == "ppt" && chapter.FileURL != "" {
		return chapter.FileURL
	}
	return ""
}

// downloadFile 通过 HTTP GET 下载文件内容（R2 公开访问 URL 可直接下载）。
func downloadFile(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败,状态码: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// UpdateStudyProgress 更新学习进度（按学习时长自动完成章节，无需手动点击"完成"）。
// 每次上报把 duration（分钟）累加到对应章节记录；当章节累计学习时长达到章节时长
// （chapter.duration 分钟；未设置时按 1 分钟）自动标记完成（progress=100）。
// 课程级进度 = 已完成章节数 / 总章节数。
func (s *CourseService) UpdateStudyProgress(studentID, courseID, chapterID, duration int) (map[string]any, error) {
	var totalChapters int64
	s.db.Model(&model.Chapter{}).Where("course_id = ?", courseID).Count(&totalChapters)
	if totalChapters == 0 {
		totalChapters = 1
	}

	// 1. 课程级记录（chapter_id IS NULL）：承载课程学习时长与课程进度。
	// 历史数据中不存在 NULL 记录时，从旧主记录（首条任意章节记录）继承时长，避免学习时长丢失。
	var record model.StudyRecord
	if err := s.db.Where("student_id = ? AND course_id = ? AND chapter_id IS NULL", studentID, courseID).
		Order("record_id ASC").Limit(1).Find(&record).Error; err == nil && record.RecordID == 0 {
		var legacy model.StudyRecord
		if e := s.db.Where("student_id = ? AND course_id = ?", studentID, courseID).
			Order("record_id ASC").Limit(1).Find(&legacy).Error; e == nil && legacy.RecordID > 0 {
			record.StudyDuration = legacy.StudyDuration
		}
		record = model.StudyRecord{
			StudentID:     studentID,
			CourseID:      courseID,
			StudyDuration: record.StudyDuration,
			Progress:      0,
			StudyDate:     beijingNow(),
		}
		if err := s.db.Create(&record).Error; err != nil {
			return nil, err
		}
	}

	// 2. 章节级记录：累加学习时长，达到阈值自动标记完成。
	if chapterID > 0 {
		threshold := 1
		var chapter model.Chapter
		if err := s.db.Select("duration").First(&chapter, chapterID).Error; err == nil && chapter.Duration > 0 {
			threshold = chapter.Duration
		}
		var ch model.StudyRecord
		if e := s.db.Where("student_id = ? AND course_id = ? AND chapter_id = ?", studentID, courseID, chapterID).
			Order("record_id ASC").Limit(1).Find(&ch).Error; e == nil && ch.RecordID > 0 {
			ch.StudyDuration += duration
			ch.StudyDate = beijingNow()
			updates := map[string]any{
				"study_duration": ch.StudyDuration,
				"study_date":     ch.StudyDate,
			}
			if ch.StudyDuration >= threshold {
				updates["progress"] = 100
			}
			if err := s.db.Model(&model.StudyRecord{}).Where("record_id = ?", ch.RecordID).
				Updates(updates).Error; err != nil {
				return nil, err
			}
		} else {
			chProgress := 0.0
			if duration >= threshold {
				chProgress = 100
			}
			newChapter := model.StudyRecord{
				StudentID:     studentID,
				CourseID:      courseID,
				ChapterID:     &chapterID,
				StudyDuration: duration,
				Progress:      chProgress,
				StudyDate:     beijingNow(),
			}
			if err := s.db.Create(&newChapter).Error; err != nil {
				return nil, err
			}
		}
	}

	// 3. 重算课程进度：统计 progress >= 100 的不同章节数。
	var completedChapters int64
	s.db.Model(&model.StudyRecord{}).
		Where("student_id = ? AND course_id = ? AND chapter_id IS NOT NULL AND progress >= 100", studentID, courseID).
		Distinct("chapter_id").Count(&completedChapters)

	record.Progress = roundFloat2(float64(completedChapters) / float64(totalChapters) * 100)
	if err := s.db.Save(&record).Error; err != nil {
		return nil, err
	}
	// 总学习时长 = 课程级历史时长 + 各章节累计时长（避免重复统计）
	var totalDuration int64
	s.db.Model(&model.StudyRecord{}).
		Where("student_id = ? AND course_id = ?", studentID, courseID).
		Select("COALESCE(SUM(study_duration), 0)").Scan(&totalDuration)
	return map[string]any{
		"record_id":      record.RecordID,
		"progress":       record.Progress,
		"study_duration": totalDuration,
	}, nil
}

// ===== 辅助 =====

// roundFloat2 保留 2 位小数。
func roundFloat2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// roundFloat1 保留 1 位小数。
func roundFloat1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}

// ===== dict 辅助 =====

func courseToDict(c *model.Course) map[string]any {
	return map[string]any{
		"course_id":               c.CourseID,
		"name":                    c.Name,
		"description":             c.Description,
		"cover_image":             c.CoverImage,
		"duration":                c.Duration,
		"specialty_id":            c.SpecialtyID,
		"level_id":                c.LevelID,
		"theory_hours":            c.TheoryHours,
		"practice_hours":          c.PracticeHours,
		"certificate_template_id": c.CertificateTemplateID,
		"sort_order":              c.SortOrder,
		"status":                  c.Status,
		"created_at":              formatISO(c.CreatedAt),
	}
}

// fillChapterCount 填充课程章节数。
func fillChapterCount(db *gorm.DB, courseID int, dict map[string]any) {
	var count int64
	db.Model(&model.Chapter{}).Where("course_id = ?", courseID).Count(&count)
	dict["chapter_count"] = count
}

// fillPrereqIDs 填充前置课程 ID 列表（供编辑表单回填，避免前端提交空数组清空关联）。
func fillPrereqIDs(db *gorm.DB, courseID int, dict map[string]any) {
	var ids []int
	db.Model(&model.CoursePrerequisite{}).Where("course_id = ?", courseID).
		Order("prerequisite_course_id ASC").Pluck("prerequisite_course_id", &ids)
	dict["prerequisite_course_ids"] = ids
}

func chapterToDict(c *model.Chapter) map[string]any {
	return map[string]any{
		"chapter_id":   c.ChapterID,
		"course_id":    c.CourseID,
		"title":        c.Title,
		"content":      c.Content,
		"content_url":  c.ContentURL,
		"content_type": c.ContentType,
		"file_url":     c.FileURL,
		"description":  c.Description,
		"duration":     c.Duration,
		"order_num":    c.OrderNum,
		"created_at":   formatISO(c.CreatedAt),
	}
}

func chapterFileToDict(f *model.ChapterFile) map[string]any {
	d := map[string]any{
		"file_id":      f.FileID,
		"file_url":     f.FileURL,
		"file_name":    f.FileName,
		"content_type": f.ContentType,
		"file_size":    f.FileSize,
		"created_at":   formatISO(f.CreatedAt),
	}
	if f.ChapterID != nil {
		d["chapter_id"] = *f.ChapterID
	} else {
		d["chapter_id"] = nil
	}
	return d
}

func legacyFileEntry(ch *model.Chapter) map[string]any {
	fileName := ""
	if ch.FileURL != "" {
		parts := strings.Split(ch.FileURL, "/")
		fileName = parts[len(parts)-1]
	}
	contentType := ch.ContentType
	if contentType == "" {
		contentType = "document"
	}
	return map[string]any{
		"file_id":      0,
		"chapter_id":   ch.ChapterID,
		"file_url":     ch.FileURL,
		"file_name":    fileName,
		"content_type": contentType,
		"file_size":    0,
		"created_at":   formatISO(ch.CreatedAt),
	}
}

// ===== 培训目录扩展辅助（课程等级/学时/前置课程/证书模板） =====

// applyCourseTrainingFields 应用课程培训扩展字段（专业方向/等级/学时/证书模板）。
// specialty_id / level_id / certificate_template_id 传 0 或空表示清空（置 NULL）。
func applyCourseTrainingFields(db *gorm.DB, course *model.Course, data map[string]any) error {
	if v, ok := data["specialty_id"]; ok {
		id := toInt(v)
		if id < 0 {
			return errors.New("专业方向ID无效")
		}
		if id == 0 {
			course.SpecialtyID = nil
		} else {
			var count int64
			if err := db.Model(&model.Specialty{}).Where("specialty_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("专业方向不存在")
			}
			course.SpecialtyID = ptrInt(id)
		}
	}
	if v, ok := data["level_id"]; ok {
		id := toInt(v)
		if id < 0 {
			return errors.New("课程等级ID无效")
		}
		if id == 0 {
			course.LevelID = nil
		} else {
			var count int64
			if err := db.Model(&model.CourseLevel{}).Where("level_id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("课程等级不存在")
			}
			course.LevelID = ptrInt(id)
		}
	}
	if v, ok := data["certificate_template_id"]; ok {
		id := toInt(v)
		if id < 0 {
			return errors.New("证书模板ID无效")
		}
		if id == 0 {
			course.CertificateTemplateID = nil
		} else {
			var count int64
			if err := db.Model(&model.CertificateTemplate{}).Where("id = ?", id).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("证书模板不存在")
			}
			course.CertificateTemplateID = ptrInt(id)
		}
	}
	if v, ok := data["theory_hours"]; ok {
		hours := toInt(v)
		if hours < 0 {
			return errors.New("理论学时不能为负数")
		}
		course.TheoryHours = hours
	}
	if v, ok := data["practice_hours"]; ok {
		hours := toInt(v)
		if hours < 0 {
			return errors.New("实操学时不能为负数")
		}
		course.PracticeHours = hours
	}
	if v, ok := data["sort_order"]; ok {
		order := toInt(v)
		if order < 0 {
			return errors.New("课程排序值不能为负数")
		}
		course.SortOrder = order
	}
	return nil
}

// prerequisiteIDsFromData 提取前置课程 ID 列表。
func prerequisiteIDsFromData(data map[string]any) []int {
	v, ok := data["prerequisite_course_ids"]
	if !ok {
		return nil
	}
	return toIntSlice(v)
}

// toIntSlice 将任意值转为 int 切片（支持 []any / []float64 等 JSON 解码结果）。
func toIntSlice(v any) []int {
	switch vals := v.(type) {
	case []any:
		out := make([]int, 0, len(vals))
		for _, item := range vals {
			out = append(out, toInt(item))
		}
		return out
	case []float64:
		out := make([]int, 0, len(vals))
		for _, item := range vals {
			out = append(out, int(item))
		}
		return out
	case []int:
		return vals
	}
	return nil
}

// replaceCoursePrerequisites 全量替换课程前置课程关联。
func replaceCoursePrerequisites(db *gorm.DB, courseID int, prereqIDs []int) error {
	prereqIDs = dedupeInts(prereqIDs)
	// 校验前置课程存在且不能指向自己
	for _, id := range prereqIDs {
		if id == courseID {
			return errors.New("课程不能设置为自己的前置课程")
		}
		var count int64
		if err := db.Model(&model.Course{}).Where("course_id = ?", id).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return errors.New("前置课程不存在")
		}
	}
	if err := checkPrerequisiteCycle(db, courseID, prereqIDs); err != nil {
		return err
	}
	if len(prereqIDs) == 0 {
		return db.Where("course_id = ?", courseID).Delete(&model.CoursePrerequisite{}).Error
	}
	rels := make([]model.CoursePrerequisite, 0, len(prereqIDs))
	for _, id := range prereqIDs {
		rels = append(rels, model.CoursePrerequisite{
			CourseID:             courseID,
			PrerequisiteCourseID: id,
			CreatedAt:            beijingNow(),
		})
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("course_id = ?", courseID).Delete(&model.CoursePrerequisite{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&rels).Error; err != nil {
			return err
		}
		return nil
	})
}

// checkPrerequisiteCycle 检测前置课程依赖是否成环（A→B→A 等）。
// 从每个新前置课程出发，沿"它的前置课程"向下游遍历，若能回到 courseID 则成环；
// visited 防止既有数据中已存在的环导致死循环。
func checkPrerequisiteCycle(db *gorm.DB, courseID int, prereqIDs []int) error {
	if len(prereqIDs) == 0 {
		return nil
	}
	visited := make(map[int]bool, len(prereqIDs))
	stack := make([]int, 0, len(prereqIDs))
	stack = append(stack, prereqIDs...)
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if cur == courseID {
			return errors.New("前置课程关系存在循环依赖")
		}
		if visited[cur] {
			continue
		}
		visited[cur] = true
		var next []int
		if err := db.Model(&model.CoursePrerequisite{}).
			Where("course_id = ?", cur).
			Pluck("prerequisite_course_id", &next).Error; err != nil {
			return err
		}
		stack = append(stack, next...)
	}
	return nil
}

// fillCourseMeta 填充课程详情的扩展元数据：
// specialty / level / certificate_template / prerequisites（前置课程列表）。
func fillCourseMeta(db *gorm.DB, course *model.Course, detail map[string]any) {
	if course.SpecialtyID != nil {
		var spec model.Specialty
		if err := db.First(&spec, *course.SpecialtyID).Error; err == nil {
			detail["specialty"] = map[string]any{
				"specialty_id": spec.SpecialtyID,
				"code":         spec.Code,
				"name":         spec.Name,
			}
		}
	}
	if course.LevelID != nil {
		var level model.CourseLevel
		if err := db.First(&level, *course.LevelID).Error; err == nil {
			detail["level"] = map[string]any{
				"level_id": level.LevelID,
				"code":     level.Code,
				"name":     level.Name,
			}
		}
	}
	if course.CertificateTemplateID != nil {
		var tpl model.CertificateTemplate
		if err := db.First(&tpl, *course.CertificateTemplateID).Error; err == nil {
			detail["certificate_template"] = map[string]any{
				"id":            tpl.ID,
				"code":          tpl.Code,
				"name":          tpl.Name,
				"description":   tpl.Description,
				"validity_days": tpl.ValidityDays,
				"template_url":  tpl.TemplateURL,
			}
		}
	}
	// 前置课程（仅返回 id/name，避免嵌套过深）
	type prereqRow struct {
		CourseID int
		Name     string
	}
	var prereqs []prereqRow
	db.Table("course_prerequisite AS cp").
		Select("cp.prerequisite_course_id AS course_id, c.name").
		Joins("JOIN course AS c ON c.course_id = cp.prerequisite_course_id").
		Where("cp.course_id = ?", course.CourseID).
		Order("c.created_at DESC, c.course_id ASC").
		Scan(&prereqs)
	prereqList := make([]map[string]any, 0, len(prereqs))
	prereqIDList := make([]int, 0, len(prereqs))
	for _, p := range prereqs {
		prereqList = append(prereqList, map[string]any{"course_id": p.CourseID, "name": p.Name})
		prereqIDList = append(prereqIDList, p.CourseID)
	}
	detail["prerequisites"] = prereqList
	detail["prerequisite_course_ids"] = prereqIDList
}
