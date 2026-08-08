// Package service 管理员服务。
package service

import (
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// AdminService 管理员服务。
type AdminService struct {
	db *gorm.DB

	logger *zap.Logger
}

// NewAdminService 创建管理员服务实例。
func NewAdminService(db *gorm.DB, logger *zap.Logger) *AdminService {
	return &AdminService{db: db, logger: logger}
}

// ===== HRWAI 用户管理(统一) =====
// 操作 hrwai_users 表,合并原学员管理与评估用户管理两套接口。

// HrwaiUserSummary HRWAI 用户摘要(列表项,不含密码)。
type HrwaiUserSummary struct {
	ID        int       `json:"id"`
	Username  string    `json:"username"`
	Name      string    `json:"name"`
	Nickname  string    `json:"nickname"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	Company   string    `json:"company"`
	Status    int16     `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// HrwaiUserPageResult HRWAI 用户分页结果（JSON 与既有契约一致，无 pages 字段）。
type HrwaiUserPageResult struct {
	List     []HrwaiUserSummary `json:"list"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
}

// ListHrwaiUsers 分页查询 HRWAI 用户,支持按用户名/姓名/手机号模糊搜索。
func (s *AdminService) ListHrwaiUsers(page, pageSize int, keyword string) (*HrwaiUserPageResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	q := s.db.Model(&model.HrwaiUser{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR name LIKE ? OR nickname LIKE ? OR phone LIKE ?", like, like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, err
	}
	var users []model.HrwaiUser
	if err := q.Order("created_at DESC, id ASC").Limit(pageSize).Offset(offset).Find(&users).Error; err != nil {
		return nil, err
	}
	list := make([]HrwaiUserSummary, 0, len(users))
	for _, u := range users {
		list = append(list, HrwaiUserSummary{
			ID:        u.ID,
			Username:  u.Username,
			Name:      u.Name,
			Nickname:  u.Nickname,
			Phone:     u.Phone,
			Email:     u.Email,
			Company:   u.Company,
			Status:    u.Status,
			CreatedAt: u.CreatedAt,
		})
	}
	return &HrwaiUserPageResult{
		List:     list,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// CreateHrwaiUser 管理员新增 HRWAI 用户。username 缺省时用手机号充填。
func (s *AdminService) CreateHrwaiUser(phone, password, name, email, company string) (*model.HrwaiUser, error) {
	if phone == "" || password == "" || name == "" {
		return nil, errors.New("手机号、密码和姓名不能为空")
	}
	var count int64
	s.db.Model(&model.HrwaiUser{}).Where("phone = ?", phone).Count(&count)
	if count > 0 {
		return nil, errors.New("手机号已被注册")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := model.HrwaiUser{
		Username:  phone,
		Password:  hashed,
		Name:      name,
		Nickname:  generateDefaultNickname(s.db),
		Phone:     phone,
		Email:     email,
		Company:   company,
		Status:    1,
		CreatedAt: beijingNow(),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateHrwaiUser 管理员更新 HRWAI 用户资料(不含密码)。
func (s *AdminService) UpdateHrwaiUser(id int, name, email, company string, status int16) error {
	if id <= 0 {
		return errors.New("用户 ID 非法")
	}
	updates := map[string]interface{}{
		"name":    name,
		"email":   email,
		"company": company,
		"status":  status,
	}
	return s.db.Model(&model.HrwaiUser{}).Where("id = ?", id).Updates(updates).Error
}

// ResetHrwaiUserPassword 管理员重置 HRWAI 用户密码。
func (s *AdminService) ResetHrwaiUserPassword(id int, newPassword string) error {
	if id <= 0 {
		return errors.New("用户 ID 非法")
	}
	if newPassword == "" {
		return errors.New("新密码不能为空")
	}
	hashed, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.db.Model(&model.HrwaiUser{}).Where("id = ?", id).Update("password", hashed).Error
}

// DeleteHrwaiUser 管理员删除 HRWAI 用户。
func (s *AdminService) DeleteHrwaiUser(id int) error {
	if id <= 0 {
		return errors.New("用户 ID 非法")
	}
	return s.db.Delete(&model.HrwaiUser{}, id).Error
}

// ToggleHrwaiUserStatus 切换 HRWAI 用户启用/禁用状态,返回切换后的新状态。
func (s *AdminService) ToggleHrwaiUserStatus(id int) (int16, error) {
	if id <= 0 {
		return 0, errors.New("用户 ID 非法")
	}
	var user model.HrwaiUser
	if err := s.db.First(&user, id).Error; err != nil {
		return 0, errors.New("用户不存在")
	}
	next := int16(1)
	if user.Status == 1 {
		next = 0
	}
	if err := s.db.Model(&user).Update("status", next).Error; err != nil {
		return 0, err
	}
	return next, nil
}

// GetTutors 导师列表。
func (s *AdminService) GetTutors(page, pageSize int, keyword string) map[string]any {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.Tutor{})
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR name LIKE ?", like, like)
	}
	var total int64
	q.Count(&total)
	var tutors []model.Tutor
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tutors)
	items := make([]map[string]any, 0, len(tutors))
	for i := range tutors {
		items = append(items, tutorToDict(&tutors[i]))
	}
	return map[string]any{
		"total":  total,
		"page":   page,
		"tutors": items,
	}
}

// DeleteTutor 删除导师。
func (s *AdminService) DeleteTutor(tutorID int) (map[string]any, error) {
	var tutor model.Tutor
	if err := s.db.First(&tutor, tutorID).Error; err != nil {
		return nil, errors.New("导师不存在")
	}
	if err := s.db.Delete(&tutor).Error; err != nil {
		return nil, err
	}
	return map[string]any{"tutor_id": tutorID}, nil
}

// ResetTutorPassword 重置导师密码。
func (s *AdminService) ResetTutorPassword(tutorID int, password string) error {
	var tutor model.Tutor
	if err := s.db.First(&tutor, tutorID).Error; err != nil {
		return errors.New("导师不存在")
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return err
	}
	return s.db.Model(&model.Tutor{}).Where("tutor_id = ?", tutorID).
		Update("password", hashed).Error
}

// ToggleTutorStatus 切换导师启用/禁用状态，返回切换后的新状态。
func (s *AdminService) ToggleTutorStatus(tutorID int) (int, error) {
	var tutor model.Tutor
	if err := s.db.First(&tutor, tutorID).Error; err != nil {
		return 0, errors.New("导师不存在")
	}
	next := 1
	if tutor.Status == 1 {
		next = 0
	}
	if err := s.db.Model(&tutor).Update("status", next).Error; err != nil {
		return 0, err
	}
	return next, nil
}

// GetStatistics 统计看板。
func (s *AdminService) GetStatistics() map[string]any {
	return s.queryStatistics()
}

// queryStatistics 执行实际的统计查询。
func (s *AdminService) queryStatistics() map[string]any {
	var totalStudents, totalCourses, totalStudyDuration int64
	s.db.Model(&model.Student{}).Count(&totalStudents)
	s.db.Model(&model.Course{}).Count(&totalCourses)
	s.db.Model(&model.StudyRecord{}).Select("COALESCE(SUM(study_duration), 0)").Scan(&totalStudyDuration)

	todayStart := beijingNow()
	startOfDay := todayStart
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Hour()) * time.Hour)
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Minute()) * time.Minute)
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Second()) * time.Second)
	startOfDay = startOfDay.Add(-time.Duration(startOfDay.Nanosecond()) * time.Nanosecond)

	var activeToday int64
	s.db.Model(&model.StudyRecord{}).Where("study_date >= ?", startOfDay).
		Distinct("student_id").Count(&activeToday)

	// 课程统计
	type courseStatRow struct {
		CourseID      int
		Name          string
		StudyCount    int64
		AvgProgress   float64
		TotalDuration int64
	}
	var rows []courseStatRow
	s.db.Model(&model.Course{}).
		Select(`course.course_id, course.name,
			COUNT(DISTINCT study_record.student_id) as study_count,
			COALESCE(AVG(study_record.progress), 0) as avg_progress,
			COALESCE(SUM(study_record.study_duration), 0) as total_duration`).
		Joins("LEFT JOIN study_record ON study_record.course_id = course.course_id").
		Group("course.course_id").
		Scan(&rows)

	courseStats := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		courseStats = append(courseStats, map[string]any{
			"course_id":      r.CourseID,
			"name":           r.Name,
			"study_count":    r.StudyCount,
			"total_duration": r.TotalDuration,
			"avg_progress":   roundFloat2(r.AvgProgress),
		})
	}

	return map[string]any{
		"overview": map[string]any{
			"total_students":       totalStudents,
			"active_today":         activeToday,
			"total_courses":        totalCourses,
			"total_study_duration": totalStudyDuration,
		},
		"course_stats": courseStats,
	}
}

// ===== dict 辅助 =====

func tutorToDict(t *model.Tutor) map[string]any {
	return map[string]any{
		"tutor_id":   t.TutorID,
		"username":   t.Username,
		"name":       t.Name,
		"status":     t.Status,
		"created_at": formatISO(t.CreatedAt),
	}
}
