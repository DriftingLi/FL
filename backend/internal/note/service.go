// Package note 笔记域（ADR-0055）：题目笔记与独立笔记共用一张 note 表 —— 「我的笔记」列表与 CRUD。
//
// 形态（ADR-0070）：handler.go 是 HTTP 出口（/api/notes 蓝图），service.go 是域实现。
// 题目维度上的单条笔记读写（/api/questions/:question_id/note）由 internal/questioninteraction
// 侧的 handler 承载，它持有本包的 Service；本包不反向依赖它。
//
// 图片（#2069）：复用论坛图片上传通道与 images/forum/ 前缀（ADR-0066 决策 5，不新建通道），
// 每条笔记 0–3 张、顺序即用户顺序；因此本表的图必须被论坛悬空图片清理算进引用集
// （见 internal/forum/image_service.go 的 collectReferencedImages）。
package note

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/timefmt"
	"forklift-training/pkg/paging"
)

// 笔记域（ADR-0055）：题目笔记与独立笔记共用一张 note 表，靠 question_id 是否为空区分。
// 读写一律以 user_id 收口——越权访问他人笔记按「不存在」处理，不泄漏存在性。

// maxNoteLen 笔记正文长度上限（与前端 textarea 的 maxlength 同源）。
// 语义照实现**沿用字节数**（历史口径，非本次议题）：中文一字三字节，故实际字数上限低于 2000。
const maxNoteLen = 2000

// NoteImagesMax 每条笔记的图片上限（#2069）：与论坛回复同档（3 张）——
// 笔记是「一条记录 + 两张佐证图」的形态，不是图集。
const NoteImagesMax = 3

// ErrNoteNotFound 笔记不存在或不属于当前用户（同一哨兵：不区分「没有」与「不是你的」）。
var ErrNoteNotFound = errors.New("笔记不存在")

// 笔记内容的两条输入事实（同评论域，ADR-0065 决策 7 的连带件）：有名字才谈得上分档。
var (
	ErrNoteContentEmpty   = errors.New("笔记内容不能为空")
	ErrNoteContentTooLong = errors.New("笔记不能超过2000字")
)

// 笔记图片的四条输入事实（#2069；判据与论坛图片通道同一套）：
// 张数超限、地址不是本站 images/forum/ 前缀、不是图片扩展名、文件在存储侧不存在。
var (
	ErrNoteImagesTooMany       = errors.New("最多只能添加 3 张图片")
	ErrNoteImageNotAllowed     = errors.New("图片地址不在允许范围内")
	ErrNoteImageMissing        = errors.New("图片文件不存在")
	ErrNoteStorageUnconfigured = errors.New("存储后端未配置")
)

// 笔记列表的筛选口径（scope 查询参数）。
const (
	// NoteScopeAll 全部笔记（题目笔记 + 独立笔记）。
	NoteScopeAll = "all"
	// NoteScopeQuestion 只列题目笔记（question_id 非空）。
	NoteScopeQuestion = "question"
	// NoteScopeStandalone 只列独立笔记（question_id 为空）。
	NoteScopeStandalone = "standalone"
)

// NoteDTO 笔记条目。字段按 JSON key 字母序声明（content / id / images / question_content /
// question_id / updated_at），与同域其它 DTO 同口径——换 struct 后序列化字节序不变。
// QuestionID 为指针且**无 omitempty**：独立笔记该键存在且为 null，而不是整个键消失。
type NoteDTO struct {
	Content string `json:"content"`
	ID      int    `json:"id"`
	// Images 恒发 []（无图时不缺席）：消费端可直接遍历，不必先判空。
	Images          []string `json:"images" nullability:"nonnil"`
	QuestionContent string   `json:"question_content"`
	QuestionID      *int     `json:"question_id" extensions:"x-nullable"`
	UpdatedAt       string   `json:"updated_at"`
}

// NotePageDTO 我的笔记分页（字段按 JSON key 字母序：items / page / page_size / total）。
type NotePageDTO struct {
	Items    []NoteDTO `json:"items" nullability:"nonnil"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int64     `json:"total"`
}

// QuestionNoteDTO 题目维度单条笔记的返回形状（#2069）。
//
// 为什么不再直接回 model.Note：模型加 images 之后，JSONB 会落进响应闭包，被表态锁要求
// 「集合字段必须声明可空性」；而 model 层不适合挂契约 tag（同一列有多个消费面）。
// 这里用显式 DTO 把**既有五个键一个不少**地带回来（content / id / question_id / updated_at /
// user_id），再加 images——响应只增不减。
type QuestionNoteDTO struct {
	Content    string   `json:"content"`
	ID         int      `json:"id"`
	Images     []string `json:"images" nullability:"nonnil"`
	QuestionID *int     `json:"question_id" extensions:"x-nullable"`
	UpdatedAt  string   `json:"updated_at"`
	UserID     int      `json:"user_id"`
}

// NoteToDTO 笔记模型 → 对外 NoteDTO。questionContent 由调用方决定是否填充：
// 单条写路径不额外查题干（走空串），列表路径由一次 LEFT JOIN 带回。
func NoteToDTO(n *model.Note, questionContent string) NoteDTO {
	if n == nil {
		return NoteDTO{}
	}
	return NoteDTO{
		Content:         n.Content,
		ID:              n.ID,
		Images:          noteImages(n.Images),
		QuestionContent: questionContent,
		QuestionID:      n.QuestionID,
		UpdatedAt:       timefmt.FormatISO(n.UpdatedAt),
	}
}

// QuestionNoteToDTO 题目笔记模型 → 题目面返回形状（#2069）。
func QuestionNoteToDTO(n *model.Note) QuestionNoteDTO {
	if n == nil {
		return QuestionNoteDTO{}
	}
	return QuestionNoteDTO{
		Content:    n.Content,
		ID:         n.ID,
		Images:     noteImages(n.Images),
		QuestionID: n.QuestionID,
		UpdatedAt:  timefmt.FormatISO(n.UpdatedAt),
		UserID:     n.UserID,
	}
}

// noteImages 把 JSONB 列读成 []string：空列 / 脏数据一律读成空集（一条坏行不该让整页 500）。
func noteImages(raw model.JSONB) []string {
	out := make([]string, 0, NoteImagesMax)
	if len(raw) == 0 {
		return out
	}
	var parsed []string
	if err := json.Unmarshal(raw, &parsed); err == nil && parsed != nil {
		return parsed
	}
	return out
}

// noteImagesJSON 把 []string 落成 JSONB 列（[]string 的 marshal 不会失败，故不返 error）。
func noteImagesJSON(images []string) model.JSONB {
	if images == nil {
		images = []string{}
	}
	raw, _ := json.Marshal(images)
	return model.JSONB(raw)
}

// Service 学员笔记服务：题目笔记（每人每题一条）+ 独立笔记（可多条）。
type Service struct {
	db      *gorm.DB
	fileSvc *filestore.FileStore
	logger  *zap.Logger
}

// NewService 创建笔记服务。fileSvc 可为 nil：那样图片「真实存在」那一校验会报错而不是放行
// （放行等于把校验变成测试装配下的空洞；装配根永远带真存储上来）。
func NewService(db *gorm.DB, fileSvc *filestore.FileStore, logger *zap.Logger) *Service {
	return &Service{db: db, fileSvc: fileSvc, logger: logger}
}

// validateNoteImages 图片校验（#2069，与论坛图片通道同一套判据）：
// 张数 ≤ 3 → 本站 images/forum/ 前缀 → 图片扩展名 → 文件真实存在；同一张图只留一次。
// 投稿/反馈那套「前缀属本人」在这里不适用：images/forum/ 不是按用户分区的目录。
func (s *Service) validateNoteImages(images []string) ([]string, error) {
	if len(images) > NoteImagesMax {
		return nil, ErrNoteImagesTooMany
	}
	out := make([]string, 0, len(images))
	seen := make(map[string]bool, len(images))
	for _, raw := range images {
		u := strings.TrimSpace(raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		if !filestore.IsSiteAttachmentURL(u, filestore.ForumImageDirPrefix) {
			return nil, ErrNoteImageNotAllowed
		}
		if filestore.UploadCategoryOf(filestore.FileExtension(u)) != "image" {
			return nil, ErrNoteImageNotAllowed
		}
		exists, err := s.imageExists(u)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, ErrNoteImageMissing
		}
		out = append(out, u)
	}
	return out, nil
}

// imageExists 图片在存储侧是否真实存在（未装配存储时报错而不是放行）。
func (s *Service) imageExists(fileURL string) (bool, error) {
	if s.fileSvc == nil {
		return false, ErrNoteStorageUnconfigured
	}
	exists, err := s.fileSvc.Exists(context.Background(), fileURL)
	if errors.Is(err, filestore.ErrStorageUnconfigured) {
		return false, ErrNoteStorageUnconfigured
	}
	return exists, err
}

// normalizeNoteContent 正文归一：去首尾空白 → 非空校验 → 长度上限。
func normalizeNoteContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", ErrNoteContentEmpty
	}
	if len(content) > maxNoteLen {
		return "", ErrNoteContentTooLong
	}
	return content, nil
}

// GetForQuestion 取本人对某题的笔记；没有则返回 (nil, nil)（不是错误）。
// 题目须在本 scope 内可见（ADR-0062 决策 4：题目维度的学员读面一律收 scope）——
// 池外题按「不存在」处理，与题目 by-id 读面同口径（笔记行本身仍是本人的私有数据，列表照列）。
func (s *Service) GetForQuestion(questionID, userID int, scope questionbank.QuestionReadScope) (*model.Note, error) {
	if err := questionbank.QuestionVisibleOrErr(scope, s.db, questionID); err != nil {
		return nil, err
	}
	var n model.Note
	if err := s.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&n).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &n, nil
}

// UpsertForQuestion 保存本人对某题的笔记（每人每题一条，UNIQUE(question_id, user_id) 兜底）。
// scope 必传（ADR-0062 决策 4）：**写**在题上的东西也要先证明这道题对这位学员可读——
// 否则「挂到不可见题上」就成了绕过池的第二条通道（列表读面会把题干带回来）。
// 图片整组替换（#2069）：更新即「这条笔记现在配这几张图」，与正文同一语义。
func (s *Service) UpsertForQuestion(questionID, userID int, content string, images []string, scope questionbank.QuestionReadScope) (*model.Note, error) {
	content, err := normalizeNoteContent(content)
	if err != nil {
		return nil, err
	}
	normalized, err := s.validateNoteImages(images)
	if err != nil {
		return nil, err
	}
	if err := questionbank.QuestionVisibleOrErr(scope, s.db, questionID); err != nil {
		return nil, err
	}
	var n model.Note
	err = s.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&n).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if n.ID != 0 {
		n.Content = content
		n.Images = noteImagesJSON(normalized)
		n.UpdatedAt = clock.Now()
		if err := s.db.Save(&n).Error; err != nil {
			return nil, err
		}
		return &n, nil
	}
	qid := questionID
	n = model.Note{QuestionID: &qid, UserID: userID, Content: content, Images: noteImagesJSON(normalized), UpdatedAt: clock.Now()}
	if err := s.db.Create(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// DeleteForQuestion 删除本人对某题的笔记（不存在时静默成功，与旧口径一致）。
// 题目维度同 GetForQuestion：池外题按「不存在」，判据由 scope 承载。
func (s *Service) DeleteForQuestion(questionID, userID int, scope questionbank.QuestionReadScope) error {
	if err := questionbank.QuestionVisibleOrErr(scope, s.db, questionID); err != nil {
		return err
	}
	return s.db.Where("question_id = ? AND user_id = ?", questionID, userID).Delete(&model.Note{}).Error
}

// Create 新建一条独立笔记（question_id 为空）。
func (s *Service) Create(userID int, content string, images []string) (*model.Note, error) {
	content, err := normalizeNoteContent(content)
	if err != nil {
		return nil, err
	}
	normalized, err := s.validateNoteImages(images)
	if err != nil {
		return nil, err
	}
	n := model.Note{UserID: userID, Content: content, Images: noteImagesJSON(normalized), UpdatedAt: clock.Now()}
	if err := s.db.Create(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// Update 按笔记 id 改正文与图片（**只认本人**：他人笔记按不存在处理）。
// 图片整组替换（#2069）：传空数组即清空图片，与正文「传什么就是什么」同语义。
func (s *Service) Update(id, userID int, content string, images []string) (*model.Note, error) {
	content, err := normalizeNoteContent(content)
	if err != nil {
		return nil, err
	}
	normalized, err := s.validateNoteImages(images)
	if err != nil {
		return nil, err
	}
	var n model.Note
	if err := s.db.Where("id = ? AND user_id = ?", id, userID).First(&n).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNoteNotFound
		}
		return nil, err
	}
	n.Content = content
	n.Images = noteImagesJSON(normalized)
	n.UpdatedAt = clock.Now()
	if err := s.db.Save(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// Delete 按笔记 id 删除（只认本人；删不到报 ErrNoteNotFound）。
//
// 图片文件**不在删除时同步删**（#2069）：它们与论坛图片同前缀，退出引用集后由那轮 24h 悬空清理
// 回收（ADR-0027 C2 的收口在 sweep 里）。在删除路径里顺手删存储会让「删笔记」变成一次可能失败的
// 外部写，还要决定回不回滚——那不是本域该背的复杂度。
func (s *Service) Delete(id, userID int) error {
	res := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.Note{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNoteNotFound
	}
	return nil
}

// List 本人笔记分页（按 updated_at 倒序，同刻用 id 兜底）。noteScope 取 all / question /
// standalone（未知值按 all 处理）。题目笔记一并带回题干摘要：一次 LEFT JOIN 覆盖整页
// （禁 N+1），供列表卡片显示「这条笔记挂在哪道题上」与跳题。
//
// qScope（题目读 scope，ADR-0062 决策 4）挂在那次 LEFT JOIN 上：**摘要只对池内题回填**。
// 修复前 JOIN 不带池谓词 ⇒ 学员历史上（或经其他写面）挂在 draft / pending / 源标记真题题 /
// 非当前证件题上的笔记，可以经这一条批量收割题干。笔记行本身是本人的私有数据，
// 照常列出（摘要为空），与「题目已删除」同形态——不靠隐藏条目来判。
func (s *Service) List(userID int, noteScope string, page, pageSize int, qScope questionbank.QuestionReadScope) (*NotePageDTO, error) {
	// 页大小上限保留既有「超上限截断到上限」语义（与 ClampMax 的「超上限回退默认」不同），
	// 先归一化再交给 paging（其钳制对已归一化的值成为空操作）。
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	base := func() *gorm.DB {
		q := s.db.Model(&model.Note{}).Where("note.user_id = ?", userID)
		switch noteScope {
		case NoteScopeQuestion:
			q = q.Where("note.question_id IS NOT NULL")
		case NoteScopeStandalone:
			q = q.Where("note.question_id IS NULL")
		}
		return q
	}
	poolSQL, poolArgs := qScope.WhereSQL()
	type row struct {
		ID              int
		QuestionID      *int
		Content         string
		Images          model.JSONB
		UpdatedAt       time.Time
		QuestionContent string
	}
	rows, total, page, pageSize, err := paging.QueryWithScan[row](s.db, page, pageSize, 20, 100,
		"note.updated_at DESC, note.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return base().
				Select("note.id, note.question_id, note.content, note.images, note.updated_at, COALESCE(question.content, '') AS question_content").
				Joins("LEFT JOIN question ON question.id = note.question_id AND "+poolSQL, poolArgs...)
		})
	if err != nil {
		return nil, err
	}
	items := make([]NoteDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, NoteToDTO(&model.Note{ID: r.ID, QuestionID: r.QuestionID, Content: r.Content, Images: r.Images, UpdatedAt: r.UpdatedAt}, r.QuestionContent))
	}
	return &NotePageDTO{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}
