// Package service 实现业务服务层。
// 本文件：资料投稿（contribution）域（#517 / ADR-0026）——学员上传资料换积分。
//
// 词汇（CONTEXT.md）：投稿（contribution）≠ 学习资料（material）。material 是课程附件
// （讲师/管理员发布），contribution 是学员提交、平台审核的独立浏览面。
//
// 生命周期：pending → approved / rejected；pending → withdrawn（作者撤回）；
// approved → archived（管理员下架）。rejected 不可恢复——重提 = 新建投稿（新行新审核）。
// 过审分与达阶分是「审核/达阶时点即发」的预付奖励；违规下架必须追回（与问答采纳的
// 「删帖不回滚」有意相反，见 ADR-0026）。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// 投稿域常量（单一事实源，调用侧不得另立）。
const (
	// ContributionFileDirPrefix 投稿文件前缀。先传后交：文件先落此前缀，提交后由
	// user_contribution_file 行引用即转正式（无物理搬移——引用即归属）。
	// 未引用的悬空文件由悬空扫描守护按 ContributionOrphanTTL 口径回收（与论坛图片同模式）。
	ContributionFileDirPrefix = "contributions"

	ContributionMaxFiles     = 5
	ContributionMaxFileSize  = 20 * 1024 * 1024 // 单文件 20MB
	ContributionMaxTotalSize = 50 * 1024 * 1024 // 合计 50MB
	ContributionTitleMaxLen  = 120
	ContributionIntroMaxLen  = 2000
	ContributionOrphanTTL    = 24 * time.Hour

	// 配额背压（供给侧防刷，以 user_contribution 表为计数事实源）。
	ContributionDailyMax   = 3 // 自然日（Asia/Shanghai）最多提交份数
	ContributionPendingMax = 5 // 名下 pending 积压上限（达此数不能再投）

	// 状态值域。
	ContributionStatusPending   = "pending"
	ContributionStatusApproved  = "approved"
	ContributionStatusRejected  = "rejected"
	ContributionStatusWithdrawn = "withdrawn"
	ContributionStatusArchived  = "archived"
)

// 过审奖励（#517 定价）：过审 +50（略高于问答采纳 +40，体现重贡献）。
const ContributionApprovedPoints = 50

// 积分流水原因（投稿域）。
const (
	ReasonContributionApproved = "contribution_approved" // 过审直记
	ReasonContributionTier     = "contribution_tier"     // 达阶追加（幂等键含档位）
	RefTypeContribution        = "contribution"          // 投稿域 ref_type
)

// ContributionTier 达阶档位。
type ContributionTier struct {
	Threshold int // 下载量达到该阈值
	Points    int // 追加奖励
}

// ContributionTiers 达阶档位（升序）。跨档判定：下载落库后当次判定是否刚好跨过阈值，
// 幂等键含档位（contribution_tier:{id}:{threshold}），一天然只发一档。
var ContributionTiers = []ContributionTier{
	{Threshold: 10, Points: 30},
	{Threshold: 50, Points: 80},
	{Threshold: 200, Points: 200},
}

// 错误哨兵（ADR-0024：一语义一哨兵；handler 以 errors.Is 映射 HTTP 状态码）。
var (
	ErrContributionNotFound            = errors.New("投稿不存在")
	ErrContributionNotOwner            = errors.New("只有投稿作者可以执行此操作")
	ErrContributionNotPending          = errors.New("只有待审核投稿可执行此操作")
	ErrContributionNotApproved         = errors.New("只有已上架投稿可执行此操作")
	ErrContributionQuotaDaily          = errors.New("今日投稿已达上限（3 份），请明天再试")
	ErrContributionQuotaPending        = errors.New("待审核投稿已达 5 份，请等待审核后再投")
	ErrContributionNoCredential        = errors.New("请先选定目标证件再投稿")
	ErrContributionTitleRequired       = errors.New("标题不能为空")
	ErrContributionIntroRequired       = errors.New("简介不能为空")
	ErrContributionFilesRequired       = errors.New("请至少上传 1 个文件")
	ErrContributionFilesTooMany        = errors.New("一份投稿最多 5 个文件")
	ErrContributionFileTooLarge        = errors.New("单文件不能超过 20MB")
	ErrContributionTotalTooLarge       = errors.New("投稿文件合计不能超过 50MB")
	ErrContributionFileInvalid         = errors.New("不支持的文件格式")
	ErrContributionRejectReason        = errors.New("驳回原因不能为空")
	ErrContributionArchiveReason       = errors.New("下架原因不能为空")
	ErrContributionInvalidReportReason = errors.New("举报理由无效")
)

// ContributionAuthor 投稿作者信息（展示名 = 昵称；匿名投稿不展示）。
type ContributionAuthor struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	Anonymous bool   `json:"anonymous"`
}

// ContributionFileDTO 投稿文件对象。
type ContributionFileDTO struct {
	FileID      int64  `json:"file_id,omitempty"`
	FileName    string `json:"file_name"`
	FileURL     string `json:"file_url"`
	FileSize    int64  `json:"file_size"`
	ContentType string `json:"content_type"`
}

// ContributionItemDTO 投稿对象。
type ContributionItemDTO struct {
	ID             int64                 `json:"id"`
	CredentialID   int                   `json:"credential_id"`
	Title          string                `json:"title"`
	Intro          string                `json:"intro"`
	Status         string                `json:"status"`
	IsAnonymous    bool                  `json:"is_anonymous"`
	DownloadsCount int                   `json:"downloads_count"`
	RejectReason   string                `json:"reject_reason,omitempty"`
	Files          []ContributionFileDTO `json:"files,omitempty"`
	Author         ContributionAuthor    `json:"author,omitempty"`
	CreatedAt      string                `json:"created_at"`
}

// ContributionPageResult 分页结果。
type ContributionPageResult struct {
	Items    []ContributionItemDTO `json:"items"`
	Total    int64                 `json:"total"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"page_size"`
}

// ContributionService 投稿服务。
type ContributionService struct {
	db              *gorm.DB
	fileSvc         *FileStore
	notificationSvc *NotificationService
	points          *PointsService
	logger          *zap.Logger
	clk             clock.Clock
}

// NewContributionService 构造投稿服务。clk 为空时回退生产实钟（Asia/Shanghai）。
func NewContributionService(db *gorm.DB, fileSvc *FileStore, notificationSvc *NotificationService, points *PointsService, logger *zap.Logger, clk clock.Clock) *ContributionService {
	if clk == nil {
		clk = clock.Real()
	}
	return &ContributionService{
		db:              db,
		fileSvc:         fileSvc,
		notificationSvc: notificationSvc,
		points:          points,
		logger:          logger,
		clk:             clk,
	}
}

// ===== 文件暂存（先传后交）=====

// allowedContributionExt 投稿文件扩展名白名单。
var allowedContributionExt = map[string]bool{
	"pdf": true, "doc": true, "docx": true,
	"ppt": true, "pptx": true,
	"xls": true, "xlsx": true,
	"zip": true,
	"mp4": true,
}

// contributionContentType 从扩展名推导内容类型（列表图标/展示用）。
func contributionContentType(filename string) string {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(filename)))
	if ext != "" {
		ext = ext[1:]
	}
	switch ext {
	case "pdf", "doc", "docx", "xls", "xlsx":
		return "document"
	case "ppt", "pptx":
		return "ppt"
	case "mp4":
		return "video"
	case "zip":
		return "zip"
	default:
		return "other"
	}
}

// UploadFile 上传投稿暂存文件：校验扩展名与单文件大小，落 contributions/ 前缀。
func (s *ContributionService) UploadFile(ctx context.Context, fileHeader *multipart.FileHeader) (*ContributionFileDTO, error) {
	if fileHeader.Filename == "" {
		return nil, errors.New("未选择文件")
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileHeader.Filename), "."))
	if !allowedContributionExt[ext] {
		return nil, ErrContributionFileInvalid
	}
	if fileHeader.Size > ContributionMaxFileSize {
		return nil, ErrContributionFileTooLarge
	}
	src, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	defer src.Close()
	content, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	url, err := s.fileSvc.Save(content, fileHeader.Filename, ContributionFileDirPrefix)
	if err != nil {
		return nil, fmt.Errorf("文件保存失败: %w", err)
	}
	return &ContributionFileDTO{
		FileName:    fileHeader.Filename,
		FileURL:     url,
		FileSize:    fileHeader.Size,
		ContentType: contributionContentType(fileHeader.Filename),
	}, nil
}

// contributionURLKey 提取 contributions/ 后的对象 key（兼容 local 与 R2 两种 URL 形态，
// 与 forumImageKey 同手法）。用于悬空回收与归属校验。
func contributionURLKey(u string) string {
	idx := strings.Index(u, "contributions/")
	if idx < 0 {
		return ""
	}
	return u[idx:]
}

// contributionFileTimestampRe 匹配 FileStore.Save 写入的毫秒时间戳（<name>_<ms>.<ext>）。
var contributionFileTimestampRe = regexp.MustCompile(`_([0-9]{10,})\.`)

// contributionFileTimestamp 从 URL 提取内嵌毫秒时间戳（悬空回收 TTL 判定用）。
func contributionFileTimestamp(url string) (int64, bool) {
	idx := strings.LastIndex(url, "/")
	name := url
	if idx >= 0 {
		name = url[idx+1:]
	}
	m := contributionFileTimestampRe.FindStringSubmatch(name)
	if len(m) < 2 {
		return 0, false
	}
	ms, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || ms <= 0 {
		return 0, false
	}
	return ms, true
}

// collectReferencedContributionFiles 收集全部投稿引用文件 key 集合（悬空回收差集用）。
func (s *ContributionService) collectReferencedContributionFiles() map[string]bool {
	ref := map[string]bool{}
	var urls []string
	if err := s.db.Model(&model.UserContributionFile{}).Pluck("file_url", &urls).Error; err == nil {
		for _, u := range urls {
			if key := contributionURLKey(u); key != "" {
				ref[key] = true
			}
		}
	}
	return ref
}

// CleanupOrphanFiles 清理投稿悬空文件：List(contributions/) 与全量引用集差集，
// 仅删除文件名时间戳超过 ContributionOrphanTTL 且未被任何投稿文件行引用的文件。
// 返回清理数（尽力而为，存储错误不中断）；ctx 取消语义贯穿到存储调用。
func (s *ContributionService) CleanupOrphanFiles(ctx context.Context) int {
	if s.fileSvc == nil {
		return 0
	}
	if ctx == nil {
		ctx = context.Background()
	}
	stored, err := s.fileSvc.ListWithContext(ctx, ContributionFileDirPrefix)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("[contribution] List 失败", zap.Error(err))
		}
		return 0
	}
	if len(stored) == 0 {
		return 0
	}
	referenced := s.collectReferencedContributionFiles()
	cleaned := 0
	cutoff := time.Now().Add(-ContributionOrphanTTL)
	for _, u := range stored {
		select {
		case <-ctx.Done():
			return cleaned
		default:
		}
		key := contributionURLKey(u)
		if key == "" || referenced[key] {
			continue
		}
		if ms, ok := contributionFileTimestamp(u); ok && time.UnixMilli(ms).Before(cutoff) {
			if err := s.fileSvc.DeleteWithContext(ctx, u); err == nil {
				cleaned++
			} else if ctx.Err() != nil {
				return cleaned
			}
		}
	}
	return cleaned
}

// ===== 资格与配额 =====

// startOfShanghaiDay 返回业务时区（Asia/Shanghai）自然日起点 00:00。
// 实现委托 clock.DayStart（ADR-0027 自然日边界单点收编）。
func (s *ContributionService) startOfShanghaiDay(t time.Time) time.Time {
	return clock.DayStart(t)
}

// countDaily 当日提交数（Asia/Shanghai 自然日起点之后的行数；time.Time 边界双方言可用）。
func (s *ContributionService) countDaily(userID int) (int64, error) {
	var cnt int64
	err := s.db.Model(&model.UserContribution{}).
		Where("user_id = ? AND created_at >= ?", userID, s.startOfShanghaiDay(s.clk.Now())).Count(&cnt).Error
	return cnt, err
}

// countPending 名下 pending 积压数。
func (s *ContributionService) countPending(userID int) (int64, error) {
	var cnt int64
	err := s.db.Model(&model.UserContribution{}).
		Where("user_id = ? AND status = ?", userID, ContributionStatusPending).Count(&cnt).Error
	return cnt, err
}

// checkQuota 校验投稿配额两臂（读路径/写路径共用单实现）。
func (s *ContributionService) checkQuota(userID int) error {
	daily, err := s.countDaily(userID)
	if err != nil {
		return err
	}
	if daily >= ContributionDailyMax {
		return ErrContributionQuotaDaily
	}
	pending, err := s.countPending(userID)
	if err != nil {
		return err
	}
	if pending >= ContributionPendingMax {
		return ErrContributionQuotaPending
	}
	return nil
}

// ===== 创建 / 列表 / 详情 / 撤回 =====

// CreateContributionInput 创建投稿入参。
type CreateContributionInput struct {
	UserID       int
	CredentialID int
	Title        string
	Intro        string
	IsAnonymous  bool
	Files        []ContributionFileDTO
	// FileURLs 直接传文件 URL 列表（已上传暂存）时使用（后端二次校验归属）。
	FileURLs []string
}

// checkCredential 校验目标证件存在且为学员当前证件（投稿挂自己的当前证件，
// 前端默认带当前证件；后端强制校验证件真实存在，防止伪造分区）。
func (s *ContributionService) checkCredential(userID, credentialID int) error {
	if credentialID <= 0 {
		return ErrContributionNoCredential
	}
	var user model.HrwaiUser
	if err := s.db.Select("current_credential_id").First(&user, userID).Error; err != nil {
		return err
	}
	if user.CurrentCredentialID == nil {
		return ErrContributionNoCredential
	}
	if *user.CurrentCredentialID != credentialID {
		return errors.New("投稿证件需与当前证件一致")
	}
	// 证件存在性
	var cnt int64
	if err := s.db.Model(&model.Credential{}).Where("id = ?", credentialID).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return errors.New("目标证件不存在")
	}
	return nil
}

// Create 创建投稿（pending）。配额两臂 + 证件校验在事务外先查一遍，事务内再守卫（防并发超投）。
func (s *ContributionService) Create(in CreateContributionInput) (*ContributionItemDTO, error) {
	// 标题/简介校验
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, ErrContributionTitleRequired
	}
	if utf8.RuneCountInString(title) > ContributionTitleMaxLen {
		return nil, errors.New("标题不能超过 120 字")
	}
	intro := strings.TrimSpace(in.Intro)
	if intro == "" {
		return nil, ErrContributionIntroRequired
	}
	if utf8.RuneCountInString(intro) > ContributionIntroMaxLen {
		return nil, errors.New("简介不能超过 2000 字")
	}
	if err := s.checkCredential(in.UserID, in.CredentialID); err != nil {
		return nil, err
	}
	if err := s.checkQuota(in.UserID); err != nil {
		return nil, err
	}
	if len(in.Files) == 0 {
		return nil, ErrContributionFilesRequired
	}
	if len(in.Files) > ContributionMaxFiles {
		return nil, ErrContributionFilesTooMany
	}
	var totalSize int64
	for _, f := range in.Files {
		if f.FileSize <= 0 {
			return nil, ErrContributionFileInvalid
		}
		if f.FileSize > ContributionMaxFileSize {
			return nil, ErrContributionFileTooLarge
		}
		totalSize += f.FileSize
	}
	if totalSize > ContributionMaxTotalSize {
		return nil, ErrContributionTotalTooLarge
	}
	now := s.clk.Now()
	var created model.UserContribution
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 事务内配额守卫（防并发超投）
		daily, err := s.countDailyTx(tx, in.UserID)
		if err != nil {
			return err
		}
		if daily >= ContributionDailyMax {
			return ErrContributionQuotaDaily
		}
		pending, err := s.countPendingTx(tx, in.UserID)
		if err != nil {
			return err
		}
		if pending >= ContributionPendingMax {
			return ErrContributionQuotaPending
		}
		created = model.UserContribution{
			UserID:       in.UserID,
			CredentialID: in.CredentialID,
			Title:        title,
			Intro:        intro,
			Status:       ContributionStatusPending,
			IsAnonymous:  in.IsAnonymous,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		for _, f := range in.Files {
			fi := model.UserContributionFile{
				ContributionID: created.ID,
				FileURL:        f.FileURL,
				FileName:       f.FileName,
				FileSize:       f.FileSize,
				ContentType:    f.ContentType,
				CreatedAt:      now,
			}
			if err := tx.Create(&fi).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetDetail(created.ID, in.UserID)
}

// countDailyTx / countPendingTx 事务内配额计数（与事务外共用条件形态）。
func (s *ContributionService) countDailyTx(tx *gorm.DB, userID int) (int64, error) {
	var cnt int64
	err := tx.Model(&model.UserContribution{}).
		Where("user_id = ? AND created_at >= ?", userID, s.startOfShanghaiDay(s.clk.Now())).Count(&cnt).Error
	return cnt, err
}

func (s *ContributionService) countPendingTx(tx *gorm.DB, userID int) (int64, error) {
	var cnt int64
	err := tx.Model(&model.UserContribution{}).
		Where("user_id = ? AND status = ?", userID, ContributionStatusPending).Count(&cnt).Error
	return cnt, err
}

// ListPublicInput 公开广场列表入参。
type ListPublicInput struct {
	CredentialID int
	Sort         string // latest / hot
	Page         int
	PageSize     int
}

// ListPublic 公开广场列表：仅 approved（非 archived——archived 是 approved 的下游状态，
// 列表口径「仅 approved」，archived 不出现）。按证件过滤 + 排序。
func (s *ContributionService) ListPublic(in ListPublicInput) (*ContributionPageResult, error) {
	order := "created_at DESC"
	if in.Sort == "hot" {
		order = "downloads_count DESC, created_at DESC"
	}
	items, total, page, pageSize := paging.QueryWithMax[model.UserContribution](
		s.db, in.Page, in.PageSize, 20, 50, order,
		func(q *gorm.DB) *gorm.DB {
			return q.Where("credential_id = ? AND status = ?", in.CredentialID, ContributionStatusApproved)
		},
	)
	dto := make([]ContributionItemDTO, 0, len(items))
	for i := range items {
		dto = append(dto, *s.toDTO(&items[i], false))
	}
	return &ContributionPageResult{Items: dto, Total: total, Page: page, PageSize: pageSize}, nil
}

// ListMine 我的投稿：全部状态，按创建时间倒序。
func (s *ContributionService) ListMine(userID, page, pageSize int) (*ContributionPageResult, error) {
	items, total, page, pageSize := paging.QueryWithMax[model.UserContribution](
		s.db, page, pageSize, 20, 50, "created_at DESC",
		func(q *gorm.DB) *gorm.DB {
			return q.Where("user_id = ?", userID)
		},
	)
	dto := make([]ContributionItemDTO, 0, len(items))
	for i := range items {
		dto = append(dto, *s.toDTO(&items[i], true))
	}
	return &ContributionPageResult{Items: dto, Total: total, Page: page, PageSize: pageSize}, nil
}

// GetDetail 投稿详情（含文件清单）。公开仅 approved；作者本人可见全部状态（含驳回原因）。
func (s *ContributionService) GetDetail(contributionID int64, viewerID int) (*ContributionItemDTO, error) {
	var c model.UserContribution
	if err := s.db.First(&c, contributionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContributionNotFound
		}
		return nil, err
	}
	if c.Status != ContributionStatusApproved && c.UserID != viewerID {
		// 非公开状态仅作者可见（防未上架稿件被路人打探）
		return nil, ErrContributionNotFound
	}
	return s.toDTO(&c, true), nil
}

// toDTO 装配 DTO（author 与 files 可选加载）。
func (s *ContributionService) toDTO(c *model.UserContribution, includeAuthorFiles bool) *ContributionItemDTO {
	dto := &ContributionItemDTO{
		ID:             c.ID,
		CredentialID:   c.CredentialID,
		Title:          c.Title,
		Intro:          c.Intro,
		Status:         c.Status,
		IsAnonymous:    c.IsAnonymous,
		DownloadsCount: c.DownloadsCount,
		CreatedAt:      c.CreatedAt.Format("2006-01-02 15:04:05"),
	}
	if c.Status == ContributionStatusRejected || c.Status == ContributionStatusArchived {
		dto.RejectReason = c.RejectReason
	}
	dto.Author = ContributionAuthor{UserID: c.UserID, Anonymous: c.IsAnonymous}
	if !c.IsAnonymous {
		var u model.HrwaiUser
		if err := s.db.Select("username").First(&u, c.UserID).Error; err == nil {
			dto.Author.Username = u.Username
		}
	}
	if includeAuthorFiles {
		var files []model.UserContributionFile
		if err := s.db.Where("contribution_id = ?", c.ID).Order("id ASC").Find(&files).Error; err == nil {
			for _, f := range files {
				dto.Files = append(dto.Files, ContributionFileDTO{
					FileID:      f.ID,
					FileName:    f.FileName,
					FileURL:     f.FileURL,
					FileSize:    f.FileSize,
					ContentType: f.ContentType,
				})
			}
		}
	}
	return dto
}

// Withdraw 作者撤回 pending 投稿（withdrawn）。
func (s *ContributionService) Withdraw(userID int, contributionID int64) error {
	res := s.db.Model(&model.UserContribution{}).
		Where("id = ? AND user_id = ? AND status = ?", contributionID, userID, ContributionStatusPending).
		Updates(map[string]any{"status": ContributionStatusWithdrawn, "updated_at": s.clk.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var c model.UserContribution
		if err := s.db.Select("user_id", "status").First(&c, contributionID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrContributionNotFound
			}
			return err
		}
		if c.UserID != userID {
			return ErrContributionNotOwner
		}
		return ErrContributionNotPending
	}
	return nil
}

// ===== 审核（T2：approve/reject 与积分直记）=====

// contributionPayload 构造投稿事件结构化标记（contribution_id + title + points + reason），
// 加性扩展，不依赖标题文案判定（照 forumAcceptPayload 形状）。
func contributionPayload(contributionID int64, title string, points int, reason string) model.JSONB {
	b, err := json.Marshal(struct {
		ContributionID int64  `json:"contribution_id"`
		Title          string `json:"title"`
		Points         int    `json:"points"`
		Reason         string `json:"reason"`
	}{ContributionID: contributionID, Title: title, Points: points, Reason: reason})
	if err != nil {
		return nil
	}
	return model.JSONB(b)
}

// 投稿站内信通知类型（#517 typed event constructor 形状：业务侧一行触发，
// 不在调用点手拼文案）。
const (
	NotifTypeContributionApproved = "contribution_approved" // 投稿过审（作者）
	NotifTypeContributionRejected = "contribution_rejected" // 投稿被驳回（作者）
	NotifTypeContributionArchived = "contribution_archived" // 投稿被下架（作者，含追回）
	NotifTypeContributionTier     = "contribution_tier"     // 投稿达阶（作者）
)

// ListPending 审核队列（pending 分页；管理端/讲师端共用）。
func (s *ContributionService) ListPending(page, pageSize int) (*ContributionPageResult, error) {
	items, total, page, pageSize := paging.QueryWithMax[model.UserContribution](
		s.db, page, pageSize, 20, 50, "created_at ASC",
		func(q *gorm.DB) *gorm.DB {
			return q.Where("status = ?", ContributionStatusPending)
		},
	)
	dto := make([]ContributionItemDTO, 0, len(items))
	for i := range items {
		dto = append(dto, *s.toDTO(&items[i], true))
	}
	return &ContributionPageResult{Items: dto, Total: total, Page: page, PageSize: pageSize}, nil
}

// Approve 通过投稿：pending → approved（CAS 防并发），直记 +50（幂等占坑防双发），
// 站内信同事务。审核者信息落 reviewed_by/reviewed_at。
func (s *ContributionService) Approve(reviewerID int, contributionID int64) (*ContributionItemDTO, error) {
	var c model.UserContribution
	if err := s.db.Select("id", "user_id", "title", "status").First(&c, contributionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContributionNotFound
		}
		return nil, err
	}
	if c.Status != ContributionStatusPending {
		return nil, ErrContributionNotPending
	}
	now := s.clk.Now()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// CAS：仅当仍 pending 时置 approved（并发双审由 RowsAffected 守卫）
		res := tx.Model(&model.UserContribution{}).
			Where("id = ? AND status = ?", contributionID, ContributionStatusPending).
			Updates(map[string]any{
				"status":      ContributionStatusApproved,
				"reviewed_by": reviewerID,
				"reviewed_at": now,
				"updated_at":  now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrContributionNotPending
		}
		// 过审 +50 直记（幂等占坑：contribution_approved:{id}，重复/并发只发一次）
		if err := s.points.SettleRewardTx(tx, PointsEntry{
			UserID: c.UserID, Delta: ContributionApprovedPoints,
			Reason: ReasonContributionApproved, RefType: RefTypeContribution, RefID: fmt.Sprintf("%d", contributionID),
			IdemKey: "contribution_approved:" + fmt.Sprintf("%d", contributionID),
		}); err != nil {
			return err
		}
		// 站内信（与入账同事务）
		if err := s.notificationSvc.CreateWithTx(tx, c.UserID, NotifTypeContributionApproved,
			"资料投稿通过审核",
			fmt.Sprintf("你的投稿「%s」已通过审核，+%d 分已到账", c.Title, ContributionApprovedPoints),
			"/training/materials?tab=contribution",
			contributionPayload(contributionID, c.Title, ContributionApprovedPoints, ReasonContributionApproved),
			now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetDetail(contributionID, c.UserID)
}

// Reject 驳回投稿：pending → rejected（必填原因），不发分。驳回原因站内信送达。
func (s *ContributionService) Reject(reviewerID int, contributionID int64, reason string) (*ContributionItemDTO, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrContributionRejectReason
	}
	var c model.UserContribution
	if err := s.db.Select("id", "user_id", "title", "status").First(&c, contributionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContributionNotFound
		}
		return nil, err
	}
	if c.Status != ContributionStatusPending {
		return nil, ErrContributionNotPending
	}
	now := s.clk.Now()
	res := s.db.Model(&model.UserContribution{}).
		Where("id = ? AND status = ?", contributionID, ContributionStatusPending).
		Updates(map[string]any{
			"status":        ContributionStatusRejected,
			"reject_reason": reason,
			"reviewed_by":   reviewerID,
			"reviewed_at":   now,
			"updated_at":    now,
		})
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrContributionNotPending
	}
	// 驳回站内信（含原因；link 到我的投稿）
	if err := s.notificationSvc.Create(c.UserID, NotifTypeContributionRejected,
		"资料投稿被驳回",
		fmt.Sprintf("你的投稿「%s」未通过审核：%s", c.Title, reason),
		"/training/materials?tab=contribution&view=mine",
		contributionPayload(contributionID, c.Title, 0, "")); err != nil {
		return nil, err
	}
	return s.GetDetail(contributionID, c.UserID)
}

// ===== 下载与达阶（T3/T4）=====

// DownloadResult 下载结果。
type DownloadResult struct {
	IsNew       bool `json:"is_new"`       // 是否新增一次计数（重复点击=false）
	TierAwarded int  `json:"tier_awarded"` // 本次触发的达阶奖励（0=未触发）
}

// Download 下载投稿：仅 approved 可下载。
// - 落 contribution_download（user_id+contribution_id 唯一 = 下载量唯一事实源）。
// - 作者本人下载不计（不落表、不加计数）。
// - 同一事务维护 downloads_count 反范式列。
// - 达阶判定：跨过哪档补哪档（幂等键含档位，并发/重试不多发）。
func (s *ContributionService) Download(userID int, contributionID int64) (*DownloadResult, error) {
	var c model.UserContribution
	if err := s.db.Select("id", "user_id", "title", "status", "downloads_count").First(&c, contributionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContributionNotFound
		}
		return nil, err
	}
	if c.Status != ContributionStatusApproved {
		return nil, ErrContributionNotApproved
	}
	// 作者本人不计
	if c.UserID == userID {
		return &DownloadResult{IsNew: false}, nil
	}
	now := s.clk.Now()
	var result DownloadResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// 1. 落事实源（唯一约束幂等：同人重复点只算 1 次）
		dl := model.ContributionDownload{UserID: userID, ContributionID: contributionID, CreatedAt: now}
		if err := tx.Create(&dl).Error; err != nil {
			if isDuplicateError(err) {
				// 已下载过：幂等返回（不新增计数）
				return nil
			}
			return err
		}
		result.IsNew = true
		// 2. 反范式计数 +1（与事实源同事务）
		if err := tx.Model(&model.UserContribution{}).Where("id = ?", contributionID).
			UpdateColumn("downloads_count", gorm.Expr("downloads_count + 1")).Error; err != nil {
			return err
		}
		newCount := c.DownloadsCount + 1
		// 3. 达阶判定：是否刚好跨过某档（跨多档只发最高一档——每次下载只可能跨一档）
		for _, tier := range ContributionTiers {
			if c.DownloadsCount < tier.Threshold && newCount >= tier.Threshold {
				if err := s.points.SettleRewardTx(tx, PointsEntry{
					UserID: c.UserID, Delta: tier.Points,
					Reason: ReasonContributionTier, RefType: RefTypeContribution, RefID: fmt.Sprintf("%d", contributionID),
					IdemKey: fmt.Sprintf("contribution_tier:%d:%d", contributionID, tier.Threshold),
				}); err != nil {
					return err
				}
				result.TierAwarded = tier.Points
				if err := s.notificationSvc.CreateWithTx(tx, c.UserID, NotifTypeContributionTier,
					"资料投稿下载量达阶",
					fmt.Sprintf("你的投稿「%s」下载量达 %d 次，+%d 分已到账", c.Title, tier.Threshold, tier.Points),
					"/training/materials?tab=contribution",
					contributionPayload(contributionID, c.Title, tier.Points, ReasonContributionTier),
					now); err != nil {
					return err
				}
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ===== 举报与处置（T5）=====

// 举报理由枚举（与前端选项对齐）。
const (
	ReportReasonPiracy       = "piracy"        // 盗版
	ReportReasonContentError = "content_error" // 内容错误
	ReportReasonViolation    = "violation"     // 违规
	ReportReasonStale        = "stale"         // 已失效
)

// validReportReason 校验举报理由。
func validReportReason(reason string) bool {
	switch reason {
	case ReportReasonPiracy, ReportReasonContentError, ReportReasonViolation, ReportReasonStale:
		return true
	}
	return false
}

// Report 举报已上架投稿（同一学员对同一投稿唯一；重复举报合并更新理由）。
func (s *ContributionService) Report(reporterID int, contributionID int64, reason string) error {
	reason = strings.TrimSpace(reason)
	if !validReportReason(reason) {
		return ErrContributionInvalidReportReason
	}
	var c model.UserContribution
	if err := s.db.Select("status").First(&c, contributionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrContributionNotFound
		}
		return err
	}
	if c.Status != ContributionStatusApproved {
		return ErrContributionNotApproved
	}
	now := s.clk.Now()
	// 唯一约束兜底并发：先查后插不幂等（重复举报合并语义），直接尝试插入，
	// 唯一冲突则更新既有行理由（不新增计数）。
	rep := model.ContributionReport{
		ReporterID: reporterID, ContributionID: contributionID, Reason: reason,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.db.Create(&rep).Error; err != nil {
		if !isDuplicateError(err) {
			return err
		}
		// 重复举报：合并（更新理由与状态回待处理），不新增行
		if err := s.db.Model(&model.ContributionReport{}).
			Where("reporter_id = ? AND contribution_id = ?", reporterID, contributionID).
			Updates(map[string]any{"reason": reason, "status": 0, "updated_at": now}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ContributionReportItemDTO 举报条目（管理端队列）。
type ContributionReportItemDTO struct {
	ID                int64  `json:"id"`
	ReporterID        int    `json:"reporter_id"`
	ContributionID    int64  `json:"contribution_id"`
	ContributionTitle string `json:"contribution_title"`
	Reason            string `json:"reason"`
	Status            int16  `json:"status"` // 0 待处理 / 1 已处理
	CreatedAt         string `json:"created_at"`
}

// ContributionReportPageResult 举报分页结果。
type ContributionReportPageResult struct {
	Items    []ContributionReportItemDTO `json:"items"`
	Total    int64                       `json:"total"`
	Page     int                         `json:"page"`
	PageSize int                         `json:"page_size"`
}

// ListReports 举报队列（status 0 待处理 / 1 已处理；nil=全部）。
func (s *ContributionService) ListReports(page, pageSize int, status *int) (*ContributionReportPageResult, error) {
	items, total, page, pageSize := paging.QueryWithMax[model.ContributionReport](
		s.db, page, pageSize, 20, 50, "created_at DESC",
		func(q *gorm.DB) *gorm.DB {
			if status != nil {
				q = q.Where("status = ?", *status)
			}
			return q
		},
	)
	dto := make([]ContributionReportItemDTO, 0, len(items))
	for i := range items {
		it := &items[i]
		title := ""
		var c model.UserContribution
		if err := s.db.Select("title").First(&c, it.ContributionID).Error; err == nil {
			title = c.Title
		}
		dto = append(dto, ContributionReportItemDTO{
			ID: it.ID, ReporterID: it.ReporterID, ContributionID: it.ContributionID,
			ContributionTitle: title, Reason: it.Reason, Status: it.Status,
			CreatedAt: it.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return &ContributionReportPageResult{Items: dto, Total: total, Page: page, PageSize: pageSize}, nil
}

// HandleReport 处置举报：action=archive 下架被举报投稿（追回积分）并标记处理；
// action=dismiss 驳回举报（标记处理，不动作）。
func (s *ContributionService) HandleReport(reviewerID int, reportID int64, action string) error {
	var rep model.ContributionReport
	if err := s.db.First(&rep, reportID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("举报不存在")
		}
		return err
	}
	switch action {
	case "archive":
		// 下架被举报投稿（举报理由作为下架原因；审核者 = 处置人）
		if _, err := s.Archive(reviewerID, rep.ContributionID, reportReasonLabel(rep.Reason)); err != nil {
			return err
		}
	case "dismiss":
		// 驳回举报：不动投稿
	default:
		return errors.New("无效的处置动作")
	}
	return s.db.Model(&model.ContributionReport{}).Where("id = ?", reportID).
		Updates(map[string]any{"status": 1, "updated_at": s.clk.Now()}).Error
}

// reportReasonLabel 举报理由的中文标签（作下架原因用）。
func reportReasonLabel(reason string) string {
	switch reason {
	case ReportReasonPiracy:
		return "被举报盗版"
	case ReportReasonContentError:
		return "被举报内容错误"
	case ReportReasonViolation:
		return "被举报违规"
	case ReportReasonStale:
		return "被举报已失效"
	}
	return "被举报"
}

// ===== 下架与追回（T5：archive + rollback）=====

// Archive 下架已上架投稿：approved → archived（必填原因，写 reject_reason 列复用）
// 并追回该投稿累计投稿分（过审 +50 与达阶分，rollback 对冲封底 0，幂等占坑防双扣）。
func (s *ContributionService) Archive(reviewerID int, contributionID int64, reason string) (*ContributionItemDTO, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, ErrContributionArchiveReason
	}
	var c model.UserContribution
	if err := s.db.Select("id", "user_id", "title", "status").First(&c, contributionID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrContributionNotFound
		}
		return nil, err
	}
	if c.Status != ContributionStatusApproved {
		return nil, ErrContributionNotApproved
	}
	now := s.clk.Now()
	var clawedBack int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// CAS：仅当仍 approved 时置 archived
		res := tx.Model(&model.UserContribution{}).
			Where("id = ? AND status = ?", contributionID, ContributionStatusApproved).
			Updates(map[string]any{
				"status":        ContributionStatusArchived,
				"reject_reason": reason,
				"reviewed_by":   reviewerID,
				"reviewed_at":   now,
				"updated_at":    now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrContributionNotApproved
		}
		// 累计投稿分（过审 + 达阶两 reason 的正向流水合计）
		var earned int64
		if err := tx.Model(&model.PointsLedger{}).
			Where("user_id = ? AND ref_type = ? AND ref_id = ? AND reason IN ? AND delta > 0",
				c.UserID, RefTypeContribution, fmt.Sprintf("%d", contributionID),
				[]string{ReasonContributionApproved, ReasonContributionTier}).
			Select("COALESCE(SUM(delta),0)").Scan(&earned).Error; err != nil {
			return err
		}
		if earned > 0 {
			// rollback 对冲（封底 0；幂等键防双扣：并发下架/重试只扣一次）
			if err := s.points.SettleRewardTx(tx, PointsEntry{
				UserID: c.UserID, Delta: -int(earned), Reason: ReasonRollback,
				RefType: RefTypeContribution, RefID: fmt.Sprintf("%d", contributionID),
				IdemKey:   "contribution_rollback:" + fmt.Sprintf("%d", contributionID),
				FloorZero: true,
			}); err != nil {
				return err
			}
			clawedBack = int(earned)
		}
		// 下架站内信（含原因与扣减；同事务）
		msg := fmt.Sprintf("你的投稿「%s」已下架：%s", c.Title, reason)
		if clawedBack > 0 {
			msg = fmt.Sprintf("你的投稿「%s」已下架：%s（已回收该稿奖励 %d 分）", c.Title, reason, clawedBack)
		}
		if err := s.notificationSvc.CreateWithTx(tx, c.UserID, NotifTypeContributionArchived,
			"资料投稿已下架", msg,
			"/training/materials?tab=contribution&view=mine",
			contributionPayload(contributionID, c.Title, -clawedBack, ReasonRollback),
			now); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetDetail(contributionID, c.UserID)
}
