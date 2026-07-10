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
func (j *JSONB) Scan(value interface{}) error {
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

// ===== 1. 学员 =====

type Student struct {
	StudentID      int        `gorm:"column:student_id;primaryKey" json:"student_id"`
	Username       string     `gorm:"column:username;uniqueIndex" json:"username"`
	Password       string     `gorm:"column:password" json:"-"`
	Name           string     `gorm:"column:name" json:"name"`
	Phone          string     `gorm:"column:phone;uniqueIndex" json:"phone"`
	Email          string     `gorm:"column:email" json:"email,omitempty"`
	Company        string     `gorm:"column:company" json:"company,omitempty"`
	Status         int16      `gorm:"column:status;default:1" json:"status"`
	Level          string     `gorm:"column:level;default:beginner" json:"level"`
	LevelUpdatedAt *time.Time `gorm:"column:level_updated_at" json:"level_updated_at,omitempty"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (Student) TableName() string { return "student" }

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
	ChapterID   int       `gorm:"column:chapter_id;primaryKey" json:"chapter_id"`
	CourseID    int       `gorm:"column:course_id" json:"course_id"`
	Title       string    `gorm:"column:title" json:"title"`
	Content     string    `gorm:"column:content" json:"content"`
	ContentURL  string    `gorm:"column:content_url" json:"content_url"`
	ContentType string    `gorm:"column:content_type;default:text" json:"content_type"`
	FileURL     string    `gorm:"column:file_url" json:"file_url"`
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
	Level       string    `gorm:"column:level" json:"level"`
	ParentID    *int      `gorm:"column:parent_id" json:"parent_id,omitempty"`
	Description string    `gorm:"column:description" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
}

func (KnowledgePoint) TableName() string { return "knowledge_point" }

// ===== 11. 题目 =====

type Question struct {
	ID               int       `gorm:"column:id;primaryKey" json:"id"`
	Type             string    `gorm:"column:type" json:"type"`
	Level            string    `gorm:"column:level" json:"level"`
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
	Level          string    `gorm:"column:level" json:"level"`
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
	Level        string    `gorm:"column:level" json:"level"`
	IsCorrect    bool      `gorm:"column:is_correct;default:false" json:"is_correct"`
	PracticeType string    `gorm:"column:practice_type;default:free" json:"practice_type"`
	UserAnswer   string    `gorm:"column:user_answer" json:"user_answer"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (QuestionPracticeRecord) TableName() string { return "question_practice_record" }

// ===== 16. 实操练习记录（叉车实操模拟，对应 Python PracticeRecord） =====

type PracticeRecord struct {
	RecordID      int       `gorm:"column:record_id;primaryKey" json:"record_id"`
	StudentID     int       `gorm:"column:student_id" json:"student_id"`
	PracticeType  string    `gorm:"column:practice_type" json:"practice_type"`
	Duration      int       `gorm:"column:duration;default:0" json:"duration"`
	Score         int       `gorm:"column:score;default:0" json:"score"`
	Operations    JSONB     `gorm:"column:operations;type:jsonb" json:"operations,omitempty"`
	Status        string    `gorm:"column:status;default:completed" json:"status"`
	Difficulty    string    `gorm:"column:difficulty;default:normal" json:"difficulty"`
	ScenarioID    *int      `gorm:"column:scenario_id" json:"scenario_id,omitempty"`
	TimeLimit     *int      `gorm:"column:time_limit" json:"time_limit,omitempty"`
	CorrectParts  JSONB     `gorm:"column:correct_parts;type:jsonb" json:"correct_parts,omitempty"`
	WrongAttempts int       `gorm:"column:wrong_attempts;default:0" json:"wrong_attempts"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

func (PracticeRecord) TableName() string { return "practice_record" }

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
	Level         string     `gorm:"column:level" json:"level"`
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
