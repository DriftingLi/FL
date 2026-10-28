// Package feedback 反馈小域（ADR-0089）：学员面向平台的**单向消息**。
//
// 四个端点、三个动作：学员提交 / 本人历史；管理端列表 / 标记已处理。
// 只有两态 pending / handled——**没有**工单号、分配、优先级、SLA、流转历史、回复线程与通知下发
// （ADR-0089 决策 1 与「备选」段）。与相邻四面的边界：不是公开内容（论坛）、不是下发通道
// （站内信）、不是编辑内容（内容精选）、不进治理队列（举报）。
//
// 图片复用论坛图片上传通道与 `images/forum/` 前缀（ADR-0066 决策 5，不新建通道）；
// 因此本表里的图片 URL 必须被论坛悬空图片清理算进引用集——见 internal/forum/image_service.go。
package feedback

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/timefmt"
	"forklift-training/pkg/paging"
)

// 反馈的输入上限（ADR-0089 决策 4）：文字 1–1000 字、图片 0–3 张、联系方式可空且不校验格式。
const (
	FeedbackContentMaxLen = 1000
	FeedbackContactMaxLen = 100
	FeedbackImagesMax     = 3
	// FeedbackListMaxPageSize 列表页大小上限（学员面与管理端同口径）。
	FeedbackListMaxPageSize = 100
)

// 反馈域哨兵（ADR-0024：一语义一哨兵，handler 用 errors.Is 映射状态码，不比对文案）。
var (
	ErrFeedbackContentRequired     = errors.New("反馈内容不能为空")
	ErrFeedbackContentTooLong      = errors.New("反馈内容不能超过 1000 字")
	ErrFeedbackContactTooLong      = errors.New("联系方式不能超过 100 字")
	ErrFeedbackImagesTooMany       = errors.New("最多只能提交 3 张图片")
	ErrFeedbackImageNotAllowed     = errors.New("图片地址不在允许范围内")
	ErrFeedbackImageMissing        = errors.New("图片文件不存在")
	ErrFeedbackStatusInvalid       = errors.New("状态筛选值无效：只接受 pending / handled / 空")
	ErrFeedbackNotFound            = errors.New("反馈不存在")
	ErrFeedbackStorageUnconfigured = errors.New("存储后端未配置")
)

// FeedbackInput 提交入参（ADR-0089 决策 4：文字必填、图片可空、联系方式可空且不校验格式）。
type FeedbackInput struct {
	Content string   `json:"content"`
	Images  []string `json:"images"`
	Contact string   `json:"contact"`
}

// FeedbackDTO 一条反馈的展示形状（学员面与管理端共用；user_id/username 只在管理端填）。
//
// handled_at 用 x-nullable：键**恒在**，未处理时是 null ——「还没处理」与「这个字段没实现」
// 必须能分开，否则后台列表分不清空列是哪一种。
type FeedbackDTO struct {
	ID       int64  `json:"id"`
	UserID   int    `json:"user_id,omitempty" extensions:"x-optional"`
	Username string `json:"username,omitempty" extensions:"x-optional"`
	Content  string `json:"content"`
	// Images 恒发 []（无图时不缺席）：消费端可直接遍历，不必先判空。
	Images    []string `json:"images" nullability:"nonnil"`
	Contact   string   `json:"contact"`
	Status    string   `json:"status"`
	HandledAt *string  `json:"handled_at" extensions:"x-nullable"`
	CreatedAt string   `json:"created_at"`
}

// FeedbackListResult 反馈分页列表（学员面与管理端同一形状）。
// 字段声明序 = 输出字节序（ADR-0009 §2）：items / page / page_size / total 与信封登记表逐字对齐，
// 顺序写错由 envelope_registry_test.go 的形状锁判红。
type FeedbackListResult struct {
	Items    []FeedbackDTO `json:"items" nullability:"nonnil"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	Total    int64         `json:"total"`
}

// Service 反馈服务。
type Service struct {
	db      *gorm.DB
	fileSvc *filestore.FileStore
	logger  *zap.Logger
}

// NewService 构造反馈服务。fileSvc 可为 nil：那样「图片真实存在」那一校验会报错而不是放行
// （放行等于把校验变成测试装配下的空洞）。
func NewService(db *gorm.DB, fileSvc *filestore.FileStore, logger *zap.Logger) *Service {
	return &Service{db: db, fileSvc: fileSvc, logger: logger}
}

// Submit 学员提交一条反馈（pending）。
//
// 校验顺序：文字 → 联系方式 → 图片张数 → 图片三校验（ADR-0066 决策 5 的可迁移部分）。
// 三校验排在落库之前：不让一次存储侧 Exists 落在事务持有期内（与投稿创建同口径）。
func (s *Service) Submit(userID int, in *FeedbackInput) (*FeedbackDTO, error) {
	content := strings.TrimSpace(in.Content)
	if content == "" {
		return nil, ErrFeedbackContentRequired
	}
	if utf8.RuneCountInString(content) > FeedbackContentMaxLen {
		return nil, ErrFeedbackContentTooLong
	}
	contact := strings.TrimSpace(in.Contact)
	if utf8.RuneCountInString(contact) > FeedbackContactMaxLen {
		return nil, ErrFeedbackContactTooLong
	}
	if len(in.Images) > FeedbackImagesMax {
		return nil, ErrFeedbackImagesTooMany
	}
	images, err := s.validateImages(in.Images)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(images)
	if err != nil {
		return nil, err
	}
	now := clock.Now()
	row := model.Feedback{
		UserID:    userID,
		Content:   content,
		Images:    model.JSONB(raw),
		Contact:   contact,
		Status:    model.FeedbackStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	dto := toDTO(&row)
	return &dto, nil
}

// validateImages 图片校验（ADR-0066 决策 5 的可迁移那三条 + 本域自有的一条）：
//  1. 本站前缀（仅 images/forum/，防外链把人引到站外）；
//  2. 扩展名是图片（读服务端铸造的 URL，不读客户端自报的文件名）；
//  3. 文件在存储侧真实存在。
//
// 外加一条本域自有的：**同一次提交内去重**（同一张图在一条反馈里出现两次没有意义）。
//
// 投稿的「前缀属本人」与「未被任何投稿登记过」两条在这里**有意不迁**（ADR-0089 决策 5）：
// images/forum/ 不是按用户分区的目录，照抄只会造出「复用别人已用过的图」也判红的判据，
// 而那条判据在本域的后果只是几行重复图片，不是归属被洗白。
func (s *Service) validateImages(images []string) ([]string, error) {
	out := make([]string, 0, len(images))
	seen := make(map[string]bool, len(images))
	for _, raw := range images {
		u := strings.TrimSpace(raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		if !filestore.IsSiteAttachmentURL(u, filestore.ForumImageDirPrefix) {
			return nil, ErrFeedbackImageNotAllowed
		}
		if filestore.UploadCategoryOf(filestore.FileExtension(u)) != "image" {
			return nil, ErrFeedbackImageNotAllowed
		}
		exists, err := s.imageExists(u)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrFeedbackImageMissing
		}
		out = append(out, u)
	}
	return out, nil
}

// imageExists 图片在存储侧是否真实存在。
// 未装配存储时报错而不是放行：本仓的装配根（cmd/server → api.NewDeps）永远带一个真存储上来，
// 放行只会让这条校验在测试里变成恒真。
func (s *Service) imageExists(fileURL string) (bool, error) {
	if s.fileSvc == nil {
		return false, ErrFeedbackStorageUnconfigured
	}
	exists, err := s.fileSvc.Exists(context.Background(), fileURL)
	if errors.Is(err, filestore.ErrStorageUnconfigured) {
		return false, ErrFeedbackStorageUnconfigured
	}
	return exists, err
}

// ListMine 学员本人历史（按时间倒序；他人反馈一律不可见——判据是 WHERE user_id = 本人）。
func (s *Service) ListMine(userID, page, pageSize int) (*FeedbackListResult, error) {
	build := func(q *gorm.DB) *gorm.DB { return q.Where("user_id = ?", userID) }
	return s.list(page, pageSize, build, false)
}

// ListAdmin 管理端列表（可按状态筛；空 / all = 全部）。
func (s *Service) ListAdmin(status string, page, pageSize int) (*FeedbackListResult, error) {
	filter, err := normalizeFeedbackStatus(status)
	if err != nil {
		return nil, err
	}
	build := func(q *gorm.DB) *gorm.DB {
		if filter != "" {
			q = q.Where("status = ?", filter)
		}
		return q
	}
	return s.list(page, pageSize, build, true)
}

// list 两个列表的唯一实现：分页 + 排序 + 可选回填作者（管理端需要知道是谁提的）。
func (s *Service) list(page, pageSize int, build func(q *gorm.DB) *gorm.DB, withAuthor bool) (*FeedbackListResult, error) {
	rows, total, pageOut, pageSizeOut, err := paging.QueryWithMax[model.Feedback](
		s.db, page, pageSize, 20, FeedbackListMaxPageSize, "created_at DESC, id DESC", build)
	if err != nil {
		return nil, err
	}
	names, err := s.usernamesOf(rows, withAuthor)
	if err != nil {
		return nil, err
	}
	items := make([]FeedbackDTO, 0, len(rows))
	for i := range rows {
		dto := toDTO(&rows[i])
		if withAuthor {
			dto.UserID = rows[i].UserID
			dto.Username = names[rows[i].UserID]
		}
		items = append(items, dto)
	}
	return &FeedbackListResult{Items: items, Total: total, Page: pageOut, PageSize: pageSizeOut}, nil
}

// usernamesOf 批量取本页作者昵称（管理者要能对上人；一次 IN 查询，不逐行查）。
func (s *Service) usernamesOf(rows []model.Feedback, withAuthor bool) (map[int]string, error) {
	names := make(map[int]string, len(rows))
	if !withAuthor || len(rows) == 0 {
		return names, nil
	}
	ids := make([]int, 0, len(rows))
	for i := range rows {
		ids = append(ids, rows[i].UserID)
	}
	var users []model.HrwaiUser
	if err := s.db.Select("id, username").Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, err
	}
	for i := range users {
		names[users[i].ID] = users[i].Username
	}
	return names, nil
}

// MarkHandled 管理端标记已处理（**幂等**）：已处理的行再点一次，原样返回、不改 handled_at。
// 管理端列表里最容易被连点的就是这一列，报错只会让人以为操作失败。
func (s *Service) MarkHandled(adminID int, id int64) (*FeedbackDTO, error) {
	var row model.Feedback
	if err := s.db.First(&row, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFeedbackNotFound
		}
		return nil, err
	}
	if row.Status == model.FeedbackStatusHandled {
		dto := toDTO(&row)
		return &dto, nil
	}
	now := clock.Now()
	// WHERE 带 status 前置：两个管理员同时点，只有一个人改得动，另一个人读到的仍是同一行已处理结果。
	if err := s.db.Model(&model.Feedback{}).
		Where("id = ? AND status = ?", id, model.FeedbackStatusPending).
		Updates(map[string]any{
			"status":     model.FeedbackStatusHandled,
			"handled_at": now,
			"handled_by": adminID,
			"updated_at": now,
		}).Error; err != nil {
		return nil, err
	}
	if err := s.db.First(&row, id).Error; err != nil {
		return nil, err
	}
	dto := toDTO(&row)
	return &dto, nil
}

// normalizeFeedbackStatus 状态筛选归一：空 / all → 不过滤；pending / handled 原样；其余报错。
// 「非法取值静默当全部」会让管理端以为筛过了（同 P1/P3 的筛选参数判据）。
func normalizeFeedbackStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "all":
		return "", nil
	case model.FeedbackStatusPending:
		return model.FeedbackStatusPending, nil
	case model.FeedbackStatusHandled:
		return model.FeedbackStatusHandled, nil
	default:
		return "", ErrFeedbackStatusInvalid
	}
}

// toDTO 行 → DTO。脏数据（images 不是合法 JSON 数组）读成空集而不是报错：
// 一条坏行不该让整个列表 500（列表是运营的唯一入口）。
func toDTO(m *model.Feedback) FeedbackDTO {
	images := make([]string, 0, 4)
	if len(m.Images) > 0 {
		var parsed []string
		if err := json.Unmarshal(m.Images, &parsed); err == nil && parsed != nil {
			images = parsed
		}
	}
	var handledAt *string
	if m.HandledAt != nil {
		v := timefmt.FormatISO(*m.HandledAt)
		handledAt = &v
	}
	return FeedbackDTO{
		ID:        m.ID,
		Content:   m.Content,
		Images:    images,
		Contact:   m.Contact,
		Status:    m.Status,
		HandledAt: handledAt,
		CreatedAt: timefmt.FormatISO(m.CreatedAt),
	}
}
