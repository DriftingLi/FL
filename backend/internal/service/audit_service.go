// Package service 实现业务服务层。
// 本文件：审计日志——写记录（Write）+ 操作命名（DescribeAction）+ 分页读（List）收敛为单一 module。
// 此前审计概念撕成两半：middleware 持裸 db 直写 + 命名，api handler 持裸 db 手写分页查询。
package service

import (
	"net/http"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// AuditService 审计日志服务：写、命名、分页读的单点归属。
// auditResourceNames 与 describeAuditAction 也从 middleware 迁入，命名文案不变。
type AuditService struct {
	db *gorm.DB
}

// NewAuditService 创建审计日志服务。
func NewAuditService(db *gorm.DB) *AuditService {
	return &AuditService{db: db}
}

// Write 落库一条审计记录（合规用途，与系统运行日志区分）。
func (s *AuditService) Write(record model.AuditLog) error {
	return s.db.Create(&record).Error
}

// List 审计日志分页查询：过滤 + count + find，返回 (items, total, page, pageSize)。
// 页大小上限 100 在本层钳制（ClampMax 语义：超上限回退默认值，而非截断到上限）。
func (s *AuditService) List(page, pageSize, actorID int, role, keyword string) ([]model.AuditLog, int64, int, int) {
	build := func(q *gorm.DB) *gorm.DB {
		if actorID > 0 {
			q = q.Where("actor_id = ?", actorID)
		}
		if role != "" {
			q = q.Where("actor_role = ?", role)
		}
		if keyword != "" {
			like := "%" + keyword + "%"
			q = q.Where("path ILIKE ? OR action ILIKE ? OR actor_name ILIKE ?", like, like, like)
		}
		return q
	}
	return paging.QueryWithMax[model.AuditLog](s.db, page, pageSize, 20, 100, "id DESC", build)
}

// DescribeAction 将 HTTP 方法与路径转成普通人能看懂的操作描述（命名单点）。
func (s *AuditService) DescribeAction(method, path string) string {
	lower := strings.ToLower(path)

	if strings.HasSuffix(lower, "/approve") {
		return "通过审核"
	}
	if strings.HasSuffix(lower, "/reject") {
		return "驳回申请"
	}
	if strings.HasSuffix(lower, "/password") {
		return "重置密码"
	}
	if strings.HasSuffix(lower, "/status") {
		return "调整状态"
	}

	verb := ""
	switch method {
	case http.MethodPost:
		verb = "新增"
	case http.MethodPut, http.MethodPatch:
		verb = "修改"
	case http.MethodDelete:
		verb = "删除"
	default:
		return method + " " + path
	}

	for _, r := range auditResourceNames {
		if strings.Contains(lower, r.key) {
			return verb + r.name
		}
	}
	return verb + "数据"
}

// auditResourceNames 路径资源关键词 → 大白话资源名（按关键词长度降序，先匹配更具体的资源）。
var auditResourceNames = []struct {
	key  string
	name string
}{
	{"profile-reviews", "资料审核"},
	{"hrwai-users", "学员"},
	{"exam-sessions", "考试场次"},
	{"featured-content", "精选内容"},
	{"ai-configs", "AI配置"},
	{"forum/replies", "论坛回复"},
	{"forum/topics", "论坛帖子"},
	{"question", "题目"},
	{"chapter", "章节"},
	{"course", "课程"},
	{"grading", "阅卷"},
	{"tutor", "导师"},
}
