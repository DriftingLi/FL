// Package service 学员简历卡模块：1:1 常驻实体于 hrwai_users，无审核队列。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

type JobCardService struct {
	db      *gorm.DB
	fileSvc *FileStore
	logger  *zap.Logger
}

func NewJobCardService(db *gorm.DB, fileSvc *FileStore, logger *zap.Logger) *JobCardService {
	return &JobCardService{db: db, fileSvc: fileSvc, logger: logger}
}

type JobCardDTO struct {
	UserID                int             `json:"user_id"`
	RealName              string          `json:"real_name"`
	ContactPhone          string          `json:"contact_phone"`
	Wechat                string          `json:"wechat"`
	Region                string          `json:"region"`
	ExpectedPositionID    *int            `json:"expected_position_id,omitempty"`
	ExpectedPositionExtra string          `json:"expected_position_extra"`
	ExpectedRegions       json.RawMessage `json:"expected_regions"`
	SalaryMin             *int            `json:"salary_min,omitempty"`
	SalaryMax             *int            `json:"salary_max,omitempty"`
	SalaryNegotiable      bool            `json:"salary_negotiable"`
	AvailableIn           string          `json:"available_in"`
	JobNature             string          `json:"job_nature"`
	ExperienceYears       int             `json:"experience_years"`
	SelfIntro             string          `json:"self_intro"`
	ResumeExperiences     json.RawMessage `json:"resume_experiences"`
	ResumeCertifications  json.RawMessage `json:"resume_certifications"`
	ResumeFileURL         string          `json:"resume_file_url"`
	Photos                json.RawMessage `json:"photos"`
	Visibility            string          `json:"visibility"`
	CreatedAt             string          `json:"created_at"`
	UpdatedAt             string          `json:"updated_at"`
}

type JobCardInput struct {
	RealName              *string          `json:"real_name"`
	ContactPhone          *string          `json:"contact_phone"`
	Wechat                *string          `json:"wechat"`
	Region                *string          `json:"region"`
	ExpectedPositionID    *int             `json:"expected_position_id"`
	ExpectedPositionExtra *string          `json:"expected_position_extra"`
	ExpectedRegions       *json.RawMessage `json:"expected_regions"`
	SalaryMin             *int             `json:"salary_min"`
	SalaryMax             *int             `json:"salary_max"`
	SalaryNegotiable      *bool            `json:"salary_negotiable"`
	AvailableIn           *string          `json:"available_in"`
	JobNature             *string          `json:"job_nature"`
	ExperienceYears       *int             `json:"experience_years"`
	SelfIntro             *string          `json:"self_intro"`
	ResumeExperiences     *json.RawMessage `json:"resume_experiences"`
	ResumeCertifications  *json.RawMessage `json:"resume_certifications"`
	ResumeFileURL         *string          `json:"resume_file_url"`
	Photos                *json.RawMessage `json:"photos"`
	Visibility            *string          `json:"visibility"`
}

type resumeCertificationRow struct {
	CredentialID *int     `json:"credential_id"`
	CertNo       string   `json:"cert_no"`
	ExpireDate   string   `json:"expire_date"`
	ImageURLs    []string `json:"image_urls"`
}

func (s *JobCardService) Get(userID int) (*JobCardDTO, error) {
	var card model.JobCard
	if err := s.db.First(&card, "user_id = ?", userID).Error; err != nil {
		return nil, err
	}
	dto := toJobCardDTO(&card)
	return &dto, nil
}

func (s *JobCardService) Upsert(userID int, in JobCardInput) (*JobCardDTO, error) {
	if err := s.validateInput(in); err != nil {
		return nil, err
	}
	var card model.JobCard
	err := s.db.First(&card, "user_id = ?", userID).Error
	isCreate := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !isCreate {
		return nil, err
	}
	now := time.Now()
	if isCreate {
		card = model.JobCard{
			UserID:     userID,
			Visibility: "hidden",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if err := ensureJSONBDefaults(&card); err != nil {
			return nil, err
		}
	} else {
		card.UpdatedAt = now
	}
	applyInput(&card, in)
	if in.Visibility != nil {
		v := strings.TrimSpace(*in.Visibility)
		if v != "hidden" && v != "open" {
			return nil, errors.New("visibility 仅支持 hidden / open")
		}
		card.Visibility = v
	} else if isCreate {
		card.Visibility = "hidden"
	}
	if isCreate {
		if err := s.db.Create(&card).Error; err != nil {
			return nil, err
		}
	} else {
		if err := s.db.Save(&card).Error; err != nil {
			return nil, err
		}
	}
	dto := toJobCardDTO(&card)
	return &dto, nil
}

func (s *JobCardService) UpdateVisibility(userID int, visibility string) (*JobCardDTO, error) {
	v := strings.TrimSpace(visibility)
	if v != "hidden" && v != "open" {
		return nil, errors.New("visibility 仅支持 hidden / open")
	}
	var card model.JobCard
	err := s.db.First(&card, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		now := time.Now()
		card = model.JobCard{
			UserID:     userID,
			Visibility: v,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = ensureJSONBDefaults(&card)
		if err := s.db.Create(&card).Error; err != nil {
			return nil, err
		}
		dto := toJobCardDTO(&card)
		return &dto, nil
	}
	if err != nil {
		return nil, err
	}
	card.Visibility = v
	card.UpdatedAt = time.Now()
	if err := s.db.Save(&card).Error; err != nil {
		return nil, err
	}
	dto := toJobCardDTO(&card)
	return &dto, nil
}

func ensureJSONBDefaults(card *model.JobCard) error {
	if len(card.ExpectedRegions) == 0 {
		card.ExpectedRegions = model.JSONB([]byte("[]"))
	}
	if len(card.ResumeExperiences) == 0 {
		card.ResumeExperiences = model.JSONB([]byte("[]"))
	}
	if len(card.ResumeCertifications) == 0 {
		card.ResumeCertifications = model.JSONB([]byte("[]"))
	}
	if len(card.Photos) == 0 {
		card.Photos = model.JSONB([]byte("[]"))
	}
	return nil
}

func applyInput(card *model.JobCard, in JobCardInput) {
	if in.RealName != nil {
		card.RealName = strings.TrimSpace(*in.RealName)
	}
	if in.ContactPhone != nil {
		card.ContactPhone = strings.TrimSpace(*in.ContactPhone)
	}
	if in.Wechat != nil {
		card.Wechat = strings.TrimSpace(*in.Wechat)
	}
	if in.Region != nil {
		card.Region = strings.TrimSpace(*in.Region)
	}
	if in.ExpectedPositionID != nil {
		if *in.ExpectedPositionID <= 0 {
			card.ExpectedPositionID = nil
		} else {
			v := *in.ExpectedPositionID
			card.ExpectedPositionID = &v
		}
	}
	if in.ExpectedPositionExtra != nil {
		card.ExpectedPositionExtra = strings.TrimSpace(*in.ExpectedPositionExtra)
	}
	if in.ExpectedRegions != nil {
		if len(*in.ExpectedRegions) == 0 || string(*in.ExpectedRegions) == "null" {
			card.ExpectedRegions = model.JSONB([]byte("[]"))
		} else {
			card.ExpectedRegions = model.JSONB([]byte(*in.ExpectedRegions))
		}
	}
	if in.SalaryMin != nil {
		card.SalaryMin = in.SalaryMin
	}
	if in.SalaryMax != nil {
		card.SalaryMax = in.SalaryMax
	}
	if in.SalaryNegotiable != nil {
		card.SalaryNegotiable = *in.SalaryNegotiable
	}
	if in.AvailableIn != nil {
		card.AvailableIn = strings.TrimSpace(*in.AvailableIn)
	}
	if in.JobNature != nil {
		card.JobNature = strings.TrimSpace(*in.JobNature)
	}
	if in.ExperienceYears != nil {
		card.ExperienceYears = *in.ExperienceYears
	}
	if in.SelfIntro != nil {
		card.SelfIntro = *in.SelfIntro
	}
	if in.ResumeExperiences != nil {
		if len(*in.ResumeExperiences) == 0 || string(*in.ResumeExperiences) == "null" {
			card.ResumeExperiences = model.JSONB([]byte("[]"))
		} else {
			card.ResumeExperiences = model.JSONB([]byte(*in.ResumeExperiences))
		}
	}
	if in.ResumeCertifications != nil {
		if len(*in.ResumeCertifications) == 0 || string(*in.ResumeCertifications) == "null" {
			card.ResumeCertifications = model.JSONB([]byte("[]"))
		} else {
			card.ResumeCertifications = model.JSONB([]byte(*in.ResumeCertifications))
		}
	}
	if in.ResumeFileURL != nil {
		card.ResumeFileURL = strings.TrimSpace(*in.ResumeFileURL)
	}
	if in.Photos != nil {
		if len(*in.Photos) == 0 || string(*in.Photos) == "null" {
			card.Photos = model.JSONB([]byte("[]"))
		} else {
			card.Photos = model.JSONB([]byte(*in.Photos))
		}
	}
}

func (s *JobCardService) validateInput(in JobCardInput) error {
	if in.SelfIntro != nil && len([]rune(*in.SelfIntro)) > 1000 {
		return errors.New("自我介绍不能超过 1000 字")
	}
	if in.ExpectedPositionID != nil && *in.ExpectedPositionID > 0 {
		var cnt int64
		if err := s.db.Model(&model.Position{}).Where("position_id = ?", *in.ExpectedPositionID).Count(&cnt).Error; err != nil {
			return err
		}
		if cnt == 0 {
			return errors.New("期望岗位不存在")
		}
	}
	if in.SalaryMin != nil && *in.SalaryMin < 0 {
		return errors.New("期望薪资不能为负")
	}
	if in.SalaryMax != nil && *in.SalaryMax < 0 {
		return errors.New("期望薪资不能为负")
	}
	if in.SalaryMin != nil && in.SalaryMax != nil && *in.SalaryMin > 0 && *in.SalaryMax > 0 && *in.SalaryMin > *in.SalaryMax {
		return errors.New("最低薪资不能高于最高薪资")
	}
	if in.ExperienceYears != nil && *in.ExperienceYears < 0 {
		return errors.New("工作年限不能为负")
	}
	if in.ResumeCertifications != nil && len(*in.ResumeCertifications) > 0 && string(*in.ResumeCertifications) != "null" && string(*in.ResumeCertifications) != "[]" {
		var rows []resumeCertificationRow
		if err := json.Unmarshal([]byte(*in.ResumeCertifications), &rows); err != nil {
			return errors.New("持证信息格式错误")
		}
		for i, r := range rows {
			if r.CredentialID != nil && *r.CredentialID > 0 {
				var cnt int64
				if err := s.db.Model(&model.Credential{}).Where("id = ?", *r.CredentialID).Count(&cnt).Error; err != nil {
					return err
				}
				if cnt == 0 {
					return fmt.Errorf("第 %d 条持证的证件不存在", i+1)
				}
			}
		}
	}
	if in.Photos != nil && len(*in.Photos) > 0 && string(*in.Photos) != "null" {
		var arr []string
		if err := json.Unmarshal([]byte(*in.Photos), &arr); err == nil {
			if len(arr) > 6 {
				return errors.New("工作照最多 6 张")
			}
		}
	}
	if in.ExpectedRegions != nil && len(*in.ExpectedRegions) > 0 && string(*in.ExpectedRegions) != "null" {
		var arr []string
		if err := json.Unmarshal([]byte(*in.ExpectedRegions), &arr); err != nil {
			return errors.New("意向地区格式错误")
		}
	}
	if in.ResumeExperiences != nil && len(*in.ResumeExperiences) > 0 && string(*in.ResumeExperiences) != "null" {
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(*in.ResumeExperiences), &arr); err != nil {
			return errors.New("工作经历格式错误")
		}
	}
	if in.ResumeFileURL != nil && strings.TrimSpace(*in.ResumeFileURL) != "" {
		url := strings.TrimSpace(*in.ResumeFileURL)
		ext := fileExtension(url)
		if ext != "pdf" {
			return errors.New("简历附件仅支持 PDF")
		}
	}
	if in.Visibility != nil {
		v := strings.TrimSpace(*in.Visibility)
		if v != "" && v != "hidden" && v != "open" {
			return errors.New("visibility 仅支持 hidden / open")
		}
	}
	return nil
}

func toJobCardDTO(m *model.JobCard) JobCardDTO {
	return JobCardDTO{
		UserID:                m.UserID,
		RealName:              m.RealName,
		ContactPhone:          m.ContactPhone,
		Wechat:                m.Wechat,
		Region:                m.Region,
		ExpectedPositionID:    m.ExpectedPositionID,
		ExpectedPositionExtra: m.ExpectedPositionExtra,
		ExpectedRegions:       json.RawMessage(m.ExpectedRegions),
		SalaryMin:             m.SalaryMin,
		SalaryMax:             m.SalaryMax,
		SalaryNegotiable:      m.SalaryNegotiable,
		AvailableIn:           m.AvailableIn,
		JobNature:             m.JobNature,
		ExperienceYears:       m.ExperienceYears,
		SelfIntro:             m.SelfIntro,
		ResumeExperiences:     json.RawMessage(m.ResumeExperiences),
		ResumeCertifications:  json.RawMessage(m.ResumeCertifications),
		ResumeFileURL:         m.ResumeFileURL,
		Photos:                json.RawMessage(m.Photos),
		Visibility:            m.Visibility,
		CreatedAt:             m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:             m.UpdatedAt.Format(time.RFC3339),
	}
}

// DeleteResumeFile 删除上传的 PDF 附件（#491：预览页操作区「删除 PDF 附件」）。
// DB 置空为事实源；对象存储文件 best-effort 回收（沿用论坛「删除即清理」惯例，失败仅日志）。
func (s *JobCardService) DeleteResumeFile(userID int) error {
	var card model.JobCard
	if err := s.db.First(&card, "user_id = ?", userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil // 无卡即无附件，幂等成功
		}
		return err
	}
	oldURL := card.ResumeFileURL
	card.ResumeFileURL = ""
	card.UpdatedAt = time.Now()
	if err := s.db.Save(&card).Error; err != nil {
		return err
	}
	// 存储文件回收：best-effort 且绝不让清理失败反噬业务（测试环境 storage 可为 nil）。
	if oldURL != "" && s.fileSvc != nil {
		func() {
			defer func() {
				if r := recover(); r != nil && s.logger != nil {
					s.logger.Warn("resume pdf 文件删除 panic（DB 已置空）", zap.Any("panic", r), zap.String("url", oldURL), zap.Int("user", userID))
				}
			}()
			if err := s.fileSvc.Delete(oldURL); err != nil && s.logger != nil {
				s.logger.Warn("resume pdf 文件删除失败（DB 已置空）", zap.Error(err), zap.String("url", oldURL), zap.Int("user", userID))
			}
		}()
	}
	return nil
}

func (s *JobCardService) ValidateAndStorePDF(filename string, size int64, content []byte) (string, error) {
	ext := fileExtension(filename)
	if ext != "pdf" {
		return "", errors.New("仅支持 PDF 文件")
	}
	if !allowedFile(filename) {
		return "", errors.New("仅支持 PDF 文件")
	}
	if !validateFileSize(size, filename) {
		return "", fmt.Errorf("文件大小超出限制，最大允许%dMB", maxFileSizes["default"]/(1024*1024))
	}
	url, err := s.fileSvc.Save(content, filename, "resumes")
	if err != nil {
		return "", err
	}
	return url, nil
}
