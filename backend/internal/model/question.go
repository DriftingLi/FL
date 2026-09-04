// 题库与练习域：题目/标签/练习记录/错题/模拟考/真题卷/评论笔记（CONTEXT.md「题库」与「练习」）。
package model

import "time"

// ===== 11. 题目 =====

type Question struct {
	ID              int    `gorm:"column:id;primaryKey" json:"id"`
	Type            string `gorm:"column:type" json:"type"`
	Content         string `gorm:"column:content" json:"content"`
	Options         JSONB  `gorm:"column:options;type:jsonb" json:"options,omitempty"`
	Answer          string `gorm:"column:answer" json:"answer"`
	Explanation     string `gorm:"column:explanation" json:"explanation"`
	AIExplanation   string `gorm:"column:ai_explanation" json:"ai_explanation,omitempty"`
	ImageURL        string `gorm:"column:image_url" json:"image_url"`
	ReferenceAnswer string `gorm:"column:reference_answer" json:"reference_answer"`
	ScoringCriteria string `gorm:"column:scoring_criteria" json:"scoring_criteria"`
	Score           int    `gorm:"column:score;default:0" json:"score"`
	// CredentialID 所属目标证件（顶层分区，单归属）。
	CredentialID  *int      `gorm:"column:credential_id" json:"credential_id,omitempty"`
	Status        string    `gorm:"column:status;default:draft" json:"status"`
	RejectReason  string    `gorm:"column:reject_reason" json:"reject_reason"`
	CreatedBy     *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedByType string    `gorm:"column:created_by_type;default:tutor" json:"created_by_type"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Question) TableName() string { return "question" }

// ===== 11.5 题库标签 =====

// QuestionTag 题库标签（法规/结构/液压/电气/制动/故障诊断/应急等考点模块）。
type QuestionTag struct {
	ID          int       `gorm:"column:id;primaryKey" json:"id"`
	Code        string    `gorm:"column:code;uniqueIndex" json:"code"`
	Name        string    `gorm:"column:name" json:"name"`
	Description string    `gorm:"column:description" json:"description"`
	SortOrder   int       `gorm:"column:sort_order;default:0" json:"sort_order"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	IsSourceTag bool      `gorm:"column:is_source_tag;default:false" json:"is_source_tag"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (QuestionTag) TableName() string { return "question_tag" }

// QuestionTagRelation 题目-标签关联（多对多）。
type QuestionTagRelation struct {
	QuestionID int       `gorm:"column:question_id;primaryKey" json:"question_id"`
	TagID      int       `gorm:"column:tag_id;primaryKey" json:"tag_id"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (QuestionTagRelation) TableName() string { return "question_tag_relation" }

// ===== 15. 题库练习记录 =====

type QuestionPracticeRecord struct {
	ID           int       `gorm:"column:id;primaryKey" json:"id"`
	StudentID    int       `gorm:"column:student_id" json:"student_id"`
	QuestionID   int       `gorm:"column:question_id" json:"question_id"`
	IsCorrect    bool      `gorm:"column:is_correct;default:false" json:"is_correct"`
	PracticeType string    `gorm:"column:practice_type;default:free" json:"practice_type"`
	UserAnswer   string    `gorm:"column:user_answer" json:"user_answer"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (QuestionPracticeRecord) TableName() string { return "question_practice_record" }

// ===== 17. 错题记录 =====

type WrongQuestion struct {
	ID          int       `gorm:"column:id;primaryKey" json:"id"`
	StudentID   int       `gorm:"column:student_id" json:"student_id"`
	QuestionID  int       `gorm:"column:question_id" json:"question_id"`
	WrongCount  int       `gorm:"column:wrong_count;default:1" json:"wrong_count"`
	LastWrongAt time.Time `gorm:"column:last_wrong_at" json:"last_wrong_at"`
	IsRemoved   bool      `gorm:"column:is_removed;default:false" json:"is_removed"`
	IsRedone    bool      `gorm:"column:is_redone;default:false" json:"is_redone"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (WrongQuestion) TableName() string { return "wrong_question" }

// ===== 18. 模拟考试 =====

type MockExam struct {
	ID            int        `gorm:"column:id;primaryKey" json:"id"`
	StudentID     int        `gorm:"column:student_id" json:"student_id"`
	QuestionIDs   JSONB      `gorm:"column:question_ids;type:jsonb" json:"question_ids,omitempty"`
	Answers       JSONB      `gorm:"column:answers;type:jsonb" json:"answers,omitempty"`
	StartTime     *time.Time `gorm:"column:start_time" json:"start_time,omitempty"`
	SubmitTime    *time.Time `gorm:"column:submit_time" json:"submit_time,omitempty"`
	RemainingTime int        `gorm:"column:remaining_time;default:0" json:"remaining_time"`
	Duration      int        `gorm:"column:duration;default:90" json:"duration"`
	Score         *float64   `gorm:"column:score;type:numeric(5,2)" json:"score,omitempty"`
	Status        string     `gorm:"column:status;default:not_started" json:"status"`
	Result        JSONB      `gorm:"column:result;type:jsonb" json:"result,omitempty"`
	PaperID       *int       `gorm:"column:paper_id" json:"paper_id,omitempty"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (MockExam) TableName() string { return "mock_exam" }

// RealExamPaper 真题套卷（导入工具按真题源文件生成，credential 单归属分区）。
type RealExamPaper struct {
	PaperID         int       `gorm:"column:paper_id;primaryKey" json:"paper_id"`
	CredentialID    int       `gorm:"column:credential_id" json:"credential_id"`
	Title           string    `gorm:"column:title" json:"title"`
	Year            *int      `gorm:"column:year" json:"year,omitempty"`
	Source          *string   `gorm:"column:source" json:"source,omitempty"`
	DurationMinutes int       `gorm:"column:duration_minutes;default:90" json:"duration_minutes"`
	LevelID         *int      `gorm:"column:level_id" json:"level_id,omitempty"`
	SourceRef       string    `gorm:"column:source_ref" json:"source_ref"`
	QuestionCount   int       `gorm:"column:question_count;default:0" json:"question_count"`
	Status          int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (RealExamPaper) TableName() string { return "real_exam_paper" }

// RealExamPaperQuestion 真题卷-题目关联（order_num 维持卷内题序）。
type RealExamPaperQuestion struct {
	PaperID    int `gorm:"column:paper_id;primaryKey" json:"paper_id"`
	QuestionID int `gorm:"column:question_id;primaryKey" json:"question_id"`
	OrderNum   int `gorm:"column:order_num;default:0" json:"order_num"`
}

func (RealExamPaperQuestion) TableName() string { return "real_exam_paper_question" }

// ===== 18.5 练习进度（顺序练习断点续练） =====

type PracticeProgress struct {
	ID           int    `gorm:"column:id;primaryKey" json:"id"`
	StudentID    int    `gorm:"column:student_id" json:"student_id"`
	PracticeMode string `gorm:"column:practice_mode" json:"practice_mode"`
	// CredentialID 进度归属的证件分区（#414）：仅顺序练习携带（唯一键 (student, mode, credential)），
	// 标签/按卷练习保持 NULL（partial 唯一索引兜底，NULL 不判重），未预筛选学员亦为 NULL。
	CredentialID *int      `gorm:"column:credential_id" json:"credential_id,omitempty"`
	QuestionIDs  JSONB     `gorm:"column:question_ids;type:jsonb" json:"question_ids,omitempty"`
	CurrentIndex int       `gorm:"column:current_index;default:0" json:"current_index"`
	Total        int       `gorm:"column:total;default:0" json:"total"`
	AnswersState JSONB     `gorm:"column:answers_state;type:jsonb" json:"answers_state"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PracticeProgress) TableName() string { return "practice_progress" }

// ===== 24.5 题目评论（刷题解析） =====

type QuestionComment struct {
	ID         int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	QuestionID int       `gorm:"column:question_id" json:"question_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	Content    string    `gorm:"column:content" json:"content"`
	CreatedAt  time.Time `gorm:"column:created_at" json:"created_at"`
}

func (QuestionComment) TableName() string { return "question_comment" }

// QuestionNote 题目笔记（每人每题一条，私有）。
type QuestionNote struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	QuestionID int       `gorm:"column:question_id" json:"question_id"`
	UserID     int       `gorm:"column:user_id" json:"user_id"`
	Content    string    `gorm:"column:content" json:"content"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (QuestionNote) TableName() string { return "question_note" }
