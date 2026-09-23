package service

import (
	"errors"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
)

// 笔记域（ADR-0055）：题目笔记与独立笔记共用一张 note 表，靠 question_id 是否为空区分。
// 读写一律以 user_id 收口——越权访问他人笔记按「不存在」处理，不泄漏存在性。

// maxNoteLen 笔记正文长度上限（与前端 textarea 的 maxlength 同源）。
// 语义照实现**沿用字节数**（历史口径，非本次议题）：中文一字三字节，故实际字数上限低于 2000。
const maxNoteLen = 2000

// ErrNoteNotFound 笔记不存在或不属于当前用户（同一哨兵：不区分「没有」与「不是你的」）。
var ErrNoteNotFound = errors.New("笔记不存在")

// 笔记列表的筛选口径（scope 查询参数）。
const (
	// NoteScopeAll 全部笔记（题目笔记 + 独立笔记）。
	NoteScopeAll = "all"
	// NoteScopeQuestion 只列题目笔记（question_id 非空）。
	NoteScopeQuestion = "question"
	// NoteScopeStandalone 只列独立笔记（question_id 为空）。
	NoteScopeStandalone = "standalone"
)

// NoteDTO 笔记条目。字段按 JSON key 字母序声明（content / id / question_content /
// question_id / updated_at），与同域其它 DTO 同口径——换 struct 后序列化字节序不变。
// QuestionID 为指针且**无 omitempty**：独立笔记该键存在且为 null，而不是整个键消失。
type NoteDTO struct {
	Content         string `json:"content"`
	ID              int    `json:"id"`
	QuestionContent string `json:"question_content"`
	QuestionID      *int   `json:"question_id" extensions:"x-nullable"`
	UpdatedAt       string `json:"updated_at"`
}

// NotePageDTO 我的笔记分页（字段按 JSON key 字母序：items / page / page_size / total）。
type NotePageDTO struct {
	Items    []NoteDTO `json:"items" nullability:"nullable"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
	Total    int64     `json:"total"`
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
		QuestionContent: questionContent,
		QuestionID:      n.QuestionID,
		UpdatedAt:       formatISO(n.UpdatedAt),
	}
}

// NoteService 学员笔记服务：题目笔记（每人每题一条）+ 独立笔记（可多条）。
type NoteService struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewNoteService 创建笔记服务。
func NewNoteService(db *gorm.DB, logger *zap.Logger) *NoteService {
	return &NoteService{db: db, logger: logger}
}

// normalizeNoteContent 正文归一：去首尾空白 → 非空校验 → 长度上限。
func normalizeNoteContent(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", errors.New("笔记内容不能为空")
	}
	if len(content) > maxNoteLen {
		return "", errors.New("笔记不能超过2000字")
	}
	return content, nil
}

// GetForQuestion 取本人对某题的笔记；没有则返回 (nil, nil)（不是错误）。
// 题目须在本 scope 内可见（ADR-0062 决策 4：题目维度的学员读面一律收 scope）——
// 池外题按「不存在」处理，与题目 by-id 读面同口径（笔记行本身仍是本人的私有数据，列表照列）。
func (s *NoteService) GetForQuestion(questionID, userID int, scope QuestionReadScope) (*model.Note, error) {
	if !scope.VisibleByID(s.db, questionID) {
		return nil, ErrQuestionNotFound
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
func (s *NoteService) UpsertForQuestion(questionID, userID int, content string, scope QuestionReadScope) (*model.Note, error) {
	content, err := normalizeNoteContent(content)
	if err != nil {
		return nil, err
	}
	if !scope.VisibleByID(s.db, questionID) {
		return nil, ErrQuestionNotFound
	}
	var n model.Note
	err = s.db.Where("question_id = ? AND user_id = ?", questionID, userID).First(&n).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if n.ID != 0 {
		n.Content = content
		n.UpdatedAt = beijingNow()
		if err := s.db.Save(&n).Error; err != nil {
			return nil, err
		}
		return &n, nil
	}
	qid := questionID
	n = model.Note{QuestionID: &qid, UserID: userID, Content: content, UpdatedAt: beijingNow()}
	if err := s.db.Create(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// DeleteForQuestion 删除本人对某题的笔记（不存在时静默成功，与旧口径一致）。
// 题目维度同 GetForQuestion：池外题按「不存在」，判据由 scope 承载。
func (s *NoteService) DeleteForQuestion(questionID, userID int, scope QuestionReadScope) error {
	if !scope.VisibleByID(s.db, questionID) {
		return ErrQuestionNotFound
	}
	return s.db.Where("question_id = ? AND user_id = ?", questionID, userID).Delete(&model.Note{}).Error
}

// Create 新建一条独立笔记（question_id 为空）。
func (s *NoteService) Create(userID int, content string) (*model.Note, error) {
	content, err := normalizeNoteContent(content)
	if err != nil {
		return nil, err
	}
	n := model.Note{UserID: userID, Content: content, UpdatedAt: beijingNow()}
	if err := s.db.Create(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// Update 按笔记 id 改正文（**只认本人**：他人笔记按不存在处理）。
func (s *NoteService) Update(id, userID int, content string) (*model.Note, error) {
	content, err := normalizeNoteContent(content)
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
	n.UpdatedAt = beijingNow()
	if err := s.db.Save(&n).Error; err != nil {
		return nil, err
	}
	return &n, nil
}

// Delete 按笔记 id 删除（只认本人；删不到报 ErrNoteNotFound）。
func (s *NoteService) Delete(id, userID int) error {
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
func (s *NoteService) List(userID int, noteScope string, page, pageSize int, qScope QuestionReadScope) (*NotePageDTO, error) {
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
		UpdatedAt       time.Time
		QuestionContent string
	}
	rows, total, page, pageSize, err := paging.QueryWithScan[row](s.db, page, pageSize, 20, 100,
		"note.updated_at DESC, note.id DESC",
		func(q *gorm.DB) *gorm.DB {
			return base().
				Select("note.id, note.question_id, note.content, note.updated_at, COALESCE(question.content, '') AS question_content").
				Joins("LEFT JOIN question ON question.id = note.question_id AND "+poolSQL, poolArgs...)
		})
	if err != nil {
		return nil, err
	}
	items := make([]NoteDTO, 0, len(rows))
	for _, r := range rows {
		items = append(items, NoteToDTO(&model.Note{ID: r.ID, QuestionID: r.QuestionID, Content: r.Content, UpdatedAt: r.UpdatedAt}, r.QuestionContent))
	}
	return &NotePageDTO{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}
