// Package service 实现业务服务层。
// 本文件：数据导出取数（xlsx 由 handler 层生成，本服务只负责取数）。
package service

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ExportService 数据导出服务。
type ExportService struct {
	db *gorm.DB
}

// NewExportService 构造导出服务。
func NewExportService(db *gorm.DB) *ExportService {
	return &ExportService{db: db}
}

// Students 学员名单导出行（首行为表头）。
func (s *ExportService) Students() ([][]any, error) {
	var rows []struct {
		ID        int
		Username  string
		Name      string
		Nickname  string
		Phone     string
		Email     string
		Company   string
		Status    int16
		CreatedAt time.Time
	}
	if err := s.db.Table("hrwai_users").Order("id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := [][]any{{"ID", "用户名", "姓名", "昵称", "手机号", "邮箱", "公司", "状态", "注册时间"}}
	for _, r := range rows {
		status := "禁用"
		if r.Status == 1 {
			status = "启用"
		}
		out = append(out, []any{
			r.ID, r.Username, r.Name, r.Nickname, r.Phone, r.Email, r.Company, status, formatISO(r.CreatedAt),
		})
	}
	return out, nil
}

// ExamRecords 成绩单导出行（已提交的考试参与记录）。
func (s *ExportService) ExamRecords() ([][]any, error) {
	var rows []struct {
		StudentID  int
		Username   string
		Name       string
		Nickname   string
		Phone      string
		ExamName   string
		Score      *float64
		IsPassed   bool
		SubmitTime *time.Time
	}
	err := s.db.Table("exam_participant AS ep").
		Select("ep.student_id, u.username, u.name, u.nickname, u.phone, es.name AS exam_name, " +
			"ep.score, ep.is_passed, ep.submit_time").
		Joins("JOIN exam_session AS es ON es.id = ep.exam_session_id").
		Joins("JOIN hrwai_users AS u ON u.id = ep.student_id").
		Where("ep.submit_time IS NOT NULL").
		Order("ep.submit_time DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := [][]any{{"学员ID", "用户名", "姓名", "昵称", "手机号", "考试名称", "分数", "是否通过", "提交时间"}}
	for _, r := range rows {
		score := ""
		if r.Score != nil {
			score = fmt.Sprintf("%.2f", *r.Score)
		}
		passed := "否"
		if r.IsPassed {
			passed = "是"
		}
		submit := ""
		if r.SubmitTime != nil {
			submit = formatISO(*r.SubmitTime)
		}
		out = append(out, []any{
			r.StudentID, r.Username, r.Name, r.Nickname, r.Phone, r.ExamName, score, passed, submit,
		})
	}
	return out, nil
}

// Questions 题库导出行。
func (s *ExportService) Questions() ([][]any, error) {
	var rows []struct {
		ID                 int
		Type               string
		Content            string
		Options            []byte
		Answer             string
		Explanation        string
		KnowledgePointName string
		Status             string
		CreatedAt          time.Time
	}
	err := s.db.Table("question AS q").
		Select("q.id, q.type, q.content, q.options, q.answer, q.explanation, " +
			"COALESCE(kp.name, '') AS knowledge_point_name, q.status, q.created_at").
		Joins("LEFT JOIN knowledge_point AS kp ON kp.id = q.knowledge_point_id").
		Order("q.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := [][]any{{"ID", "类型", "题干", "选项", "答案", "解析", "知识点", "状态", "创建时间"}}
	for _, r := range rows {
		options := ""
		if len(r.Options) > 0 {
			options = string(r.Options)
		}
		out = append(out, []any{
			r.ID, r.Type, r.Content, options, r.Answer, r.Explanation,
			r.KnowledgePointName, r.Status, formatISO(r.CreatedAt),
		})
	}
	return out, nil
}

// Evaluations 残值评估记录导出行。
func (s *ExportService) Evaluations() ([][]any, error) {
	var rows []struct {
		ID                    int64
		Username              string
		Name                  string
		Brand                 string
		VehicleType           string
		Series                string
		Tonnage               float64
		ConfigType            string
		MastType              string
		MastHeightMM          int
		FactoryYear           int
		SaleYear              int
		UsageHours            int
		OriginalPaint         bool
		Province              string
		City                  string
		HasLicensePlate       bool
		HasRegistrationCert   bool
		HasMaintenanceRecords bool
		ConditionRating       string
		OriginalPrice         float64
		KTime                 *float64
		KHours                *float64
		KBrand                *float64
		KCondition            *float64
		KMarket               *float64
		EstimatedValue        float64
		ConfidenceLow         *float64
		ConfidenceHigh        *float64
		ReportPDFPath         string
		CreatedAt             time.Time
	}
	err := s.db.Table("evaluations AS e").
		Select("e.id, COALESCE(u.username, '') AS username, COALESCE(u.name, '') AS name, " +
			"e.brand, e.vehicle_type, e.series, e.tonnage, e.config_type, e.mast_type, " +
			"e.mast_height_mm, e.factory_year, e.sale_year, e.usage_hours, e.original_paint, " +
			"e.province, e.city, e.has_license_plate, e.has_registration_certificate AS has_registration_cert, " +
			"e.has_maintenance_records, e.condition_rating, e.original_price, e.k_time, e.k_hours, " +
			"e.k_brand, e.k_condition, e.k_market, e.estimated_value, e.confidence_low, e.confidence_high, " +
			"e.report_pdf_path, e.created_at").
		Joins("LEFT JOIN hrwai_users AS u ON u.id = e.user_id").
		Order("e.id DESC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := [][]any{{
		"ID", "用户名", "姓名", "品牌", "车型", "系列", "吨位", "配置", "门架类型", "门架高度mm",
		"出厂年份", "销售年份", "工时", "原厂漆", "省份", "城市", "有牌照", "有登记证", "有维保记录",
		"车况", "原价", "Kt", "Kh", "Kb", "Kc", "Km", "评估值", "置信下限", "置信上限", "报告PDF", "创建时间",
	}}
	for _, r := range rows {
		out = append(out, []any{
			r.ID, r.Username, r.Name, r.Brand, r.VehicleType, r.Series, r.Tonnage, r.ConfigType,
			r.MastType, r.MastHeightMM, r.FactoryYear, r.SaleYear, r.UsageHours, yesNo(r.OriginalPaint),
			r.Province, r.City, yesNo(r.HasLicensePlate), yesNo(r.HasRegistrationCert),
			yesNo(r.HasMaintenanceRecords), r.ConditionRating, r.OriginalPrice,
			coeff(r.KTime), coeff(r.KHours), coeff(r.KBrand), coeff(r.KCondition), coeff(r.KMarket),
			r.EstimatedValue, nullableFloat(r.ConfidenceLow), nullableFloat(r.ConfidenceHigh),
			r.ReportPDFPath, formatISO(r.CreatedAt),
		})
	}
	return out, nil
}

// yesNo 布尔值转中文。
func yesNo(v bool) string {
	if v {
		return "是"
	}
	return "否"
}

// coeff 系数指针格式化（保留 4 位小数）。
func coeff(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.4f", *v)
}

// nullableFloat 浮点指针格式化（保留 2 位小数）。
func nullableFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%.2f", *v)
}
