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
	UID       int64     `json:"uid,string"`
	Account   string    `json:"account"`
	Username  string    `json:"username"`
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

// ListHrwaiUsers 分页查询 HRWAI 用户,支持按账号/昵称/手机号模糊搜索。
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
		q = q.Where("account LIKE ? OR username LIKE ? OR phone LIKE ?", like, like, like)
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
			UID:       u.UID,
			Account:   u.Account,
			Username:  u.Username,
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

// CreateHrwaiUser 管理员新增 HRWAI 用户。account 缺省时随机生成，昵称缺省时自动生成。
func (s *AdminService) CreateHrwaiUser(phone, password, account, username, email, company string) (*model.HrwaiUser, error) {
	if phone == "" || password == "" {
		return nil, errors.New("手机号、密码不能为空")
	}
	var count int64
	s.db.Model(&model.HrwaiUser{}).Where("phone = ?", phone).Count(&count)
	if count > 0 {
		return nil, errors.New("手机号已被注册")
	}
	if account != "" {
		if !IsValidAccount(account) {
			return nil, errors.New("账号格式非法（4-20 位字母/数字/下划线）")
		}
		var acctCount int64
		s.db.Model(&model.HrwaiUser{}).Where("account = ?", account).Count(&acctCount)
		if acctCount > 0 {
			return nil, errors.New("账号已被占用")
		}
	} else {
		var err error
		account, err = generateRandomAccount()
		if err != nil {
			return nil, errors.New("注册失败，请稍后再试")
		}
	}
	if username == "" {
		username = generateDefaultNickname(s.db)
	}
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := model.HrwaiUser{
		UID:       NextUID(),
		Account:   account,
		Username:  username,
		Password:  hashed,
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
func (s *AdminService) UpdateHrwaiUser(id int, username, email, company string, status int16) error {
	if id <= 0 {
		return errors.New("用户 ID 非法")
	}
	updates := map[string]interface{}{
		"username": username,
		"email":    email,
		"company":  company,
		"status":   status,
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

// ===== DTO（JSON 契约与 B7 前的 map key 逐字一致，前端零改动约束）=====

// TutorDTO 导师（列表项）。
type TutorDTO struct {
	TutorID   int    `json:"tutor_id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Status    int16  `json:"status"`
	CreatedAt string `json:"created_at"`
}

// TutorListDTO 导师列表信封。
type TutorListDTO struct {
	Total  int64      `json:"total"`
	Page   int        `json:"page"`
	Tutors []TutorDTO `json:"tutors"`
}

// TutorDeletedDTO 删除导师结果。
type TutorDeletedDTO struct {
	TutorID int `json:"tutor_id"`
}

// AdminOverviewDTO 统计看板概览。
type AdminOverviewDTO struct {
	TotalStudents      int64 `json:"total_students"`
	ActiveToday        int64 `json:"active_today"`
	TotalCourses       int64 `json:"total_courses"`
	TotalStudyDuration int64 `json:"total_study_duration"`
}

// CourseStatDTO 课程统计条目。
type CourseStatDTO struct {
	CourseID      int     `json:"course_id"`
	Name          string  `json:"name"`
	StudyCount    int64   `json:"study_count"`
	TotalDuration int64   `json:"total_duration"`
	AvgProgress   float64 `json:"avg_progress"`
}

// AdminStatisticsDTO 统计看板。
type AdminStatisticsDTO struct {
	Overview    AdminOverviewDTO `json:"overview"`
	CourseStats []CourseStatDTO  `json:"course_stats"`
}

// GetTutors 导师列表。
func (s *AdminService) GetTutors(page, pageSize int, keyword string) *TutorListDTO {
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
	items := make([]TutorDTO, 0, len(tutors))
	for i := range tutors {
		items = append(items, tutorToDTO(&tutors[i]))
	}
	return &TutorListDTO{
		Total:  total,
		Page:   page,
		Tutors: items,
	}
}

// DeleteTutor 删除导师。
func (s *AdminService) DeleteTutor(tutorID int) (*TutorDeletedDTO, error) {
	var tutor model.Tutor
	if err := s.db.First(&tutor, tutorID).Error; err != nil {
		return nil, errors.New("导师不存在")
	}
	if err := s.db.Delete(&tutor).Error; err != nil {
		return nil, err
	}
	return &TutorDeletedDTO{TutorID: tutorID}, nil
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
func (s *AdminService) GetStatistics() *AdminStatisticsDTO {
	return s.queryStatistics()
}

// queryStatistics 执行实际的统计查询。
func (s *AdminService) queryStatistics() *AdminStatisticsDTO {
	var totalStudents, totalCourses, totalStudyDuration int64
	s.db.Model(&model.HrwaiUser{}).Count(&totalStudents)
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

	courseStats := make([]CourseStatDTO, 0, len(rows))
	for _, r := range rows {
		courseStats = append(courseStats, CourseStatDTO{
			CourseID:      r.CourseID,
			Name:          r.Name,
			StudyCount:    r.StudyCount,
			TotalDuration: r.TotalDuration,
			AvgProgress:   roundFloat2(r.AvgProgress),
		})
	}

	return &AdminStatisticsDTO{
		Overview: AdminOverviewDTO{
			TotalStudents:      totalStudents,
			ActiveToday:        activeToday,
			TotalCourses:       totalCourses,
			TotalStudyDuration: totalStudyDuration,
		},
		CourseStats: courseStats,
	}
}

// ===== DTO 构造（原 tutorToDict 折叠入内）=====

func tutorToDTO(t *model.Tutor) TutorDTO {
	return TutorDTO{
		TutorID:   t.TutorID,
		Username:  t.Username,
		Name:      t.Name,
		Status:    t.Status,
		CreatedAt: formatISO(t.CreatedAt),
	}
}
