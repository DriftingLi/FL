// Package model 定义全部 GORM 数据模型，与 migrations/000001_init.up.sql 一一对应。
package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// JSONB 是 PostgreSQL JSONB 字段的 Go 映射，支持任意 JSON 值。
type JSONB json.RawMessage

// Scan 实现 sql.Scanner。
func (j *JSONB) Scan(value any) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = append((*j)[:0], v...)
		return nil
	case string:
		*j = JSONB(v)
		return nil
	}
	return errors.New("JSONB.Scan: 不支持的类型")
}

// Value 实现 driver.Valuer。
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// MarshalJSON 实现 json.Marshaler。
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON 实现 json.Unmarshaler。
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("JSONB.UnmarshalJSON: 指针为空")
	}
	*j = append((*j)[:0], data...)
	return nil
}

// ===== 1. HRWAI 账号(统一用户表) =====

// HrwaiUser 统一用户表,合并原 student 与 valuation_users 两表。
// 三套登录鉴权(培训学员端 / 残值评估 / AI 助手)共用此表与主体系 JWT。
// admin / tutor 账号仍保留独立表,不并入此表。
type HrwaiUser struct {
	ID        int       `gorm:"column:id;primaryKey" json:"id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	Nickname  string    `gorm:"column:nickname" json:"nickname"`
	AvatarURL string    `gorm:"column:avatar_url" json:"avatar_url"`
	Phone     string    `gorm:"column:phone;uniqueIndex" json:"phone"`
	Email     string    `gorm:"column:email" json:"email,omitempty"`
	Company   string    `gorm:"column:company" json:"company,omitempty"`
	Status    int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (HrwaiUser) TableName() string { return "hrwai_users" }

// Student 兼容旧代码引用,实际映射同一张 hrwai_users 表。
// StudentID Go 字段名保留(避免大量改下游代码),但 gorm column 改为 id(对应新表主键)。
// deprecated: 后续请改用 model.HrwaiUser
type Student struct {
	StudentID int       `gorm:"column:id;primaryKey" json:"student_id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	Nickname  string    `gorm:"column:nickname" json:"nickname"`
	AvatarURL string    `gorm:"column:avatar_url" json:"avatar_url"`
	Phone     string    `gorm:"column:phone;uniqueIndex" json:"phone"`
	Email     string    `gorm:"column:email" json:"email,omitempty"`
	Company   string    `gorm:"column:company" json:"company,omitempty"`
	Status    int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Student) TableName() string { return "hrwai_users" }

// ===== 2. 管理员 =====

type Admin struct {
	AdminID   int       `gorm:"column:admin_id;primaryKey" json:"admin_id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Admin) TableName() string { return "admin" }

// ===== 3. 导师 =====

type Tutor struct {
	TutorID   int       `gorm:"column:tutor_id;primaryKey" json:"tutor_id"`
	Username  string    `gorm:"column:username;uniqueIndex" json:"username"`
	Password  string    `gorm:"column:password" json:"-"`
	Name      string    `gorm:"column:name" json:"name"`
	Status    int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Tutor) TableName() string { return "tutor" }

// ===== 3.5 残值评估模块独立用户(已并入 hrwai_users) =====

// ValuationUser 别名,实际映射同一张 hrwai_users 表。
// deprecated: 后续请改用 model.HrwaiUser
type ValuationUser = HrwaiUser

// ===== 4. 课程 =====

type Course struct {
	CourseID    int       `gorm:"column:course_id;primaryKey" json:"course_id"`
	Name        string    `gorm:"column:name" json:"name"`
	Category    string    `gorm:"column:category" json:"category"`
	Description string    `gorm:"column:description" json:"description"`
	CoverImage  string    `gorm:"column:cover_image" json:"cover_image"`
	Duration    int       `gorm:"column:duration;default:0" json:"duration"`
	Status      int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Course) TableName() string { return "course" }

// ===== 5. 章节 =====

type Chapter struct {
	ChapterID   int    `gorm:"column:chapter_id;primaryKey" json:"chapter_id"`
	CourseID    int    `gorm:"column:course_id" json:"course_id"`
	Title       string `gorm:"column:title" json:"title"`
	Content     string `gorm:"column:content" json:"content"`
	ContentURL  string `gorm:"column:content_url" json:"content_url"`
	ContentType string `gorm:"column:content_type;default:text" json:"content_type"`
	FileURL     string `gorm:"column:file_url" json:"file_url"`
	// SlideUrls PPT 转图后的幻灯片 URL 列表（JSON 数组字符串）。
	// 为空表示未生成；由 FileService.ConvertPPTToImages 生成后写入。
	SlideUrls   string    `gorm:"column:slide_urls" json:"slide_urls"`
	Description string    `gorm:"column:description" json:"description"`
	Duration    int       `gorm:"column:duration;default:0" json:"duration"`
	OrderNum    int       `gorm:"column:order_num;default:0" json:"order_num"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Chapter) TableName() string { return "chapter" }

// ===== 6. 章节文件 =====

type ChapterFile struct {
	FileID      int       `gorm:"column:file_id;primaryKey" json:"file_id"`
	ChapterID   *int      `gorm:"column:chapter_id" json:"chapter_id,omitempty"`
	FileURL     string    `gorm:"column:file_url" json:"file_url"`
	FileName    string    `gorm:"column:file_name" json:"file_name"`
	ContentType string    `gorm:"column:content_type;default:document" json:"content_type"`
	FileSize    int64     `gorm:"column:file_size;default:0" json:"file_size"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ChapterFile) TableName() string { return "chapter_file" }

// ===== 7. 学习记录 =====

type StudyRecord struct {
	RecordID      int       `gorm:"column:record_id;primaryKey" json:"record_id"`
	StudentID     int       `gorm:"column:student_id" json:"student_id"`
	CourseID      int       `gorm:"column:course_id" json:"course_id"`
	ChapterID     *int      `gorm:"column:chapter_id" json:"chapter_id,omitempty"`
	StudyDuration int       `gorm:"column:study_duration;default:0" json:"study_duration"`
	Progress      float64   `gorm:"column:progress;type:numeric(5,2);default:0" json:"progress"`
	StudyDate     time.Time `gorm:"column:study_date" json:"study_date"`
}

func (StudyRecord) TableName() string { return "study_record" }

// ===== 8. 考核记录 =====

type ExamRecord struct {
	ExamID    int       `gorm:"column:exam_id;primaryKey" json:"exam_id"`
	StudentID int       `gorm:"column:student_id" json:"student_id"`
	CourseID  int       `gorm:"column:course_id" json:"course_id"`
	Score     *float64  `gorm:"column:score;type:numeric(5,2)" json:"score,omitempty"`
	Answers   JSONB     `gorm:"column:answers;type:jsonb" json:"answers,omitempty"`
	ExamDate  time.Time `gorm:"column:exam_date" json:"exam_date"`
}

func (ExamRecord) TableName() string { return "exam_record" }

// ===== 9. AI 生成记录 =====

type AIGenerationLog struct {
	LogID          int       `gorm:"column:log_id;primaryKey" json:"log_id"`
	UserID         int       `gorm:"column:user_id" json:"user_id"`
	UserType       string    `gorm:"column:user_type" json:"user_type"`
	GenerationType string    `gorm:"column:generation_type" json:"generation_type"`
	InputParams    JSONB     `gorm:"column:input_params;type:jsonb" json:"input_params,omitempty"`
	OutputResult   string    `gorm:"column:output_result" json:"output_result"`
	Status         int16     `gorm:"column:status;default:1" json:"status"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AIGenerationLog) TableName() string { return "ai_generation_log" }

// ===== 10. 知识点 =====

type KnowledgePoint struct {
	ID          int       `gorm:"column:id;primaryKey" json:"id"`
	Name        string    `gorm:"column:name" json:"name"`
	Category    string    `gorm:"column:category" json:"category,omitempty"`
	ParentID    *int      `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Description string    `gorm:"column:description" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (KnowledgePoint) TableName() string { return "knowledge_point" }

// ===== 11. 题目 =====

type Question struct {
	ID               int       `gorm:"column:id;primaryKey" json:"id"`
	Type             string    `gorm:"column:type" json:"type"`
	Content          string    `gorm:"column:content" json:"content"`
	Options          JSONB     `gorm:"column:options;type:jsonb" json:"options,omitempty"`
	Answer           string    `gorm:"column:answer" json:"answer"`
	Explanation      string    `gorm:"column:explanation" json:"explanation"`
	ImageURL         string    `gorm:"column:image_url" json:"image_url"`
	ReferenceAnswer  string    `gorm:"column:reference_answer" json:"reference_answer"`
	ScoringCriteria  string    `gorm:"column:scoring_criteria" json:"scoring_criteria"`
	Score            int       `gorm:"column:score;default:0" json:"score"`
	KnowledgePointID *int      `gorm:"column:knowledge_point_id" json:"knowledge_point_id,omitempty"`
	Status           string    `gorm:"column:status;default:draft" json:"status"`
	RejectReason     string    `gorm:"column:reject_reason" json:"reject_reason"`
	CreatedBy        *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	CreatedByType    string    `gorm:"column:created_by_type;default:tutor" json:"created_by_type"`
	CreatedAt        time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Question) TableName() string { return "question" }

// ===== 12. 考试场次 =====

type ExamSession struct {
	ID             int       `gorm:"column:id;primaryKey" json:"id"`
	Name           string    `gorm:"column:name" json:"name"`
	StartTime      time.Time `gorm:"column:start_time" json:"start_time"`
	EndTime        time.Time `gorm:"column:end_time" json:"end_time"`
	Duration       int       `gorm:"column:duration" json:"duration"`
	Status         string    `gorm:"column:status;default:upcoming" json:"status"`
	CreatedBy      *int      `gorm:"column:created_by" json:"created_by,omitempty"`
	QuestionConfig JSONB     `gorm:"column:question_config;type:jsonb" json:"question_config,omitempty"`
	TotalScore     int       `gorm:"column:total_score;default:0" json:"total_score"`
	PassScore      int       `gorm:"column:pass_score;default:60" json:"pass_score"`
	CreatedAt      time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (ExamSession) TableName() string { return "exam_session" }

// ===== 13. 考试参与记录 =====

type ExamParticipant struct {
	ID              int        `gorm:"column:id;primaryKey" json:"id"`
	ExamSessionID   int        `gorm:"column:exam_session_id" json:"exam_session_id"`
	StudentID       int        `gorm:"column:student_id" json:"student_id"`
	Status          string     `gorm:"column:status;default:not_started" json:"status"`
	StartTime       *time.Time `gorm:"column:start_time" json:"start_time,omitempty"`
	SubmitTime      *time.Time `gorm:"column:submit_time" json:"submit_time,omitempty"`
	RemainingTime   int        `gorm:"column:remaining_time;default:0" json:"remaining_time"`
	Score           *float64   `gorm:"column:score;type:numeric(5,2)" json:"score,omitempty"`
	ObjectiveScore  *float64   `gorm:"column:objective_score;type:numeric(5,2)" json:"objective_score,omitempty"`
	SubjectiveScore *float64   `gorm:"column:subjective_score;type:numeric(5,2)" json:"subjective_score,omitempty"`
	IsPassed        bool       `gorm:"column:is_passed;default:false" json:"is_passed"`
	AnswersSnapshot JSONB      `gorm:"column:answers_snapshot;type:jsonb" json:"answers_snapshot,omitempty"`
	QuestionIDs     JSONB      `gorm:"column:question_ids;type:jsonb" json:"question_ids,omitempty"`
	CreatedAt       time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (ExamParticipant) TableName() string { return "exam_participant" }

// ===== 14. 考试答题记录 =====

type ExamAnswer struct {
	ID                int        `gorm:"column:id;primaryKey" json:"id"`
	ExamParticipantID int        `gorm:"column:exam_participant_id" json:"exam_participant_id"`
	QuestionID        int        `gorm:"column:question_id" json:"question_id"`
	UserAnswer        string     `gorm:"column:user_answer" json:"user_answer"`
	IsCorrect         *bool      `gorm:"column:is_correct" json:"is_correct,omitempty"`
	Score             float64    `gorm:"column:score;type:numeric(5,2);default:0" json:"score"`
	GraderID          *int       `gorm:"column:grader_id" json:"grader_id,omitempty"`
	GradedAt          *time.Time `gorm:"column:graded_at" json:"graded_at,omitempty"`
	GradingComment    string     `gorm:"column:grading_comment" json:"grading_comment"`
	AIScore           *float64   `gorm:"column:ai_score;type:numeric(5,2)" json:"ai_score,omitempty"`
	AIComment         string     `gorm:"column:ai_comment" json:"ai_comment"`
	AIGradedAt        *time.Time `gorm:"column:ai_graded_at" json:"ai_graded_at,omitempty"`
}

func (ExamAnswer) TableName() string { return "exam_answer" }

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
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (MockExam) TableName() string { return "mock_exam" }

// ===== 18.5 练习进度（顺序练习断点续练） =====

type PracticeProgress struct {
	ID           int       `gorm:"column:id;primaryKey" json:"id"`
	StudentID    int       `gorm:"column:student_id" json:"student_id"`
	PracticeMode string    `gorm:"column:practice_mode" json:"practice_mode"`
	QuestionIDs  JSONB     `gorm:"column:question_ids;type:jsonb" json:"question_ids,omitempty"`
	CurrentIndex int       `gorm:"column:current_index;default:0" json:"current_index"`
	Total        int       `gorm:"column:total;default:0" json:"total"`
	AnswersState JSONB     `gorm:"column:answers_state;type:jsonb;default:'{}'::jsonb" json:"answers_state"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (PracticeProgress) TableName() string { return "practice_progress" }

// ===== 19. 异步任务 =====

type AsyncTask struct {
	ID        int       `gorm:"column:id;primaryKey" json:"id"`
	TaskType  string    `gorm:"column:task_type" json:"task_type"`
	Status    string    `gorm:"column:status;default:pending" json:"status"`
	Payload   JSONB     `gorm:"column:payload;type:jsonb" json:"payload,omitempty"`
	Result    JSONB     `gorm:"column:result;type:jsonb" json:"result,omitempty"`
	Error     string    `gorm:"column:error" json:"error"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AsyncTask) TableName() string { return "async_task" }

// ===== 20. 内容精选（公司动态 / 行业新闻 等） =====

type FeaturedContent struct {
	ContentID   int        `gorm:"column:content_id;primaryKey" json:"content_id"`
	Title       string     `gorm:"column:title" json:"title"`
	Summary     string     `gorm:"column:summary" json:"summary"`
	CoverImage  string     `gorm:"column:cover_image" json:"cover_image"`
	Content     string     `gorm:"column:content" json:"content"`
	Category    string     `gorm:"column:category;default:industry" json:"category"`
	Source      string     `gorm:"column:source" json:"source"`
	Status      int16      `gorm:"column:status;default:0" json:"status"`
	ViewCount   int        `gorm:"column:view_count;default:0" json:"view_count"`
	SortOrder   int        `gorm:"column:sort_order;default:0" json:"sort_order"`
	PublishedAt *time.Time `gorm:"column:published_at" json:"published_at,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (FeaturedContent) TableName() string { return "featured_content" }

// ===== 21. 系统设置 =====

// SystemSetting 系统设置表（key-value 结构，承载 AI 等模块的动态配置）。
type SystemSetting struct {
	Key         string    `gorm:"column:key;primaryKey" json:"key"`
	Value       string    `gorm:"column:value" json:"value"`
	Description string    `gorm:"column:description" json:"description"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SystemSetting) TableName() string { return "system_settings" }

// ===== 22. AI 多配置 =====

// AIConfig AI 服务配置（多套命名配置，可绑定到不同 AI 功能）。
type AIConfig struct {
	ID          int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"column:name;uniqueIndex" json:"name"`
	APIKey      string    `gorm:"column:api_key" json:"api_key"`
	BaseURL     string    `gorm:"column:base_url" json:"base_url"`
	Model       string    `gorm:"column:model" json:"model"`
	Description string    `gorm:"column:description" json:"description"`
	IsActive    bool      `gorm:"column:is_active" json:"is_active"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIConfig) TableName() string { return "ai_configs" }

// AIFeatureBinding AI 功能-配置绑定。
// 单绑定功能（如简答题评分）在应用层限制每个 feature_key 只能有一条记录；
// 多绑定功能（如 AI 助手）允许一个 feature_key 绑定多个 config_id（DB 通过 (feature_key, config_id) 复合唯一约束保证不重复）。
type AIFeatureBinding struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FeatureKey string    `gorm:"column:feature_key" json:"feature_key"`
	ConfigID   int       `gorm:"column:config_id" json:"config_id"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIFeatureBinding) TableName() string { return "ai_feature_bindings" }

// ===== 23. AI 助手模块 =====

// AIChatSession AI 助手会话（归属 valuation_users）。
type AIChatSession struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Title     string    `gorm:"column:title" json:"title"`
	ModelName string    `gorm:"column:model_name" json:"model_name"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIChatSession) TableName() string { return "ai_chat_sessions" }

// AIChatMessage AI 助手会话消息。
type AIChatMessage struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SessionID int       `gorm:"column:session_id" json:"session_id"`
	Role      string    `gorm:"column:role" json:"role"` // 'user' | 'assistant' | 'system'
	Content   string    `gorm:"column:content" json:"content"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AIChatMessage) TableName() string { return "ai_chat_messages" }

// AIUserModel 用户自定义 AI 模型配置（openai 兼容格式，归属 valuation_users）。
type AIUserModel struct {
	ID        int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	Name      string    `gorm:"column:name" json:"name"`
	APIKey    string    `gorm:"column:api_key" json:"api_key"`
	BaseURL   string    `gorm:"column:base_url" json:"base_url"`
	Model     string    `gorm:"column:model" json:"model"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AIUserModel) TableName() string { return "ai_user_models" }

// ===== 24. 论坛模块 =====

// ForumTopic 论坛主题（chapter_id 为 NULL 表示综合讨论区，非 NULL 表示章节讨论区）。
type ForumTopic struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ChapterID   *int       `gorm:"column:chapter_id" json:"chapter_id,omitempty"`
	UserID      int        `gorm:"column:user_id" json:"user_id"`
	Title       string     `gorm:"column:title" json:"title"`
	Content     string     `gorm:"column:content" json:"content"`
	ViewCount   int        `gorm:"column:view_count;default:0" json:"view_count"`
	ReplyCount  int        `gorm:"column:reply_count;default:0" json:"reply_count"`
	LastReplyAt *time.Time `gorm:"column:last_reply_at" json:"last_reply_at"`
	CreatedAt   time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ForumTopic) TableName() string { return "forum_topics" }

// ForumReply 论坛回复。
type ForumReply struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TopicID   int64     `gorm:"column:topic_id" json:"topic_id"`
	UserID    int       `gorm:"column:user_id" json:"user_id"`
	ParentID  *int64    `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Content   string    `gorm:"column:content" json:"content"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (ForumReply) TableName() string { return "forum_replies" }

// ===== 25. 资料修改审核 =====

// ProfileChangeRequest 用户资料（昵称/头像）修改审核请求。
type ProfileChangeRequest struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID       int        `gorm:"column:user_id" json:"user_id"`
	FieldType    string     `gorm:"column:field_type" json:"field_type"` // nickname / avatar
	OldValue     string     `gorm:"column:old_value" json:"old_value"`
	NewValue     string     `gorm:"column:new_value" json:"new_value"`
	Status       string     `gorm:"column:status;default:pending" json:"status"`
	RejectReason string     `gorm:"column:reject_reason" json:"reject_reason"`
	ReviewedBy   *int       `gorm:"column:reviewed_by" json:"reviewed_by,omitempty"`
	ReviewedAt   *time.Time `gorm:"column:reviewed_at" json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (ProfileChangeRequest) TableName() string { return "profile_change_requests" }
