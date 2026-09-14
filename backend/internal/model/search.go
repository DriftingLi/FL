package model

import "time"

// SearchFact 检索事实（ADR-0049 决策 7）：搜索「发生过什么」的**匿名**记录。
//
// 三列之外刻意什么都不加：不记 user_id / 当前证件 / 设备 / IP ——
// 它的用途只有两个（零结果词运营、口径复盘），指向人就会变成画像面。
// 搜索历史属学员个人痕迹，只留在各端本地，不进本表。
type SearchFact struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Keyword    string `gorm:"column:keyword;not null;default:''" json:"keyword"`
	SearchType string `gorm:"column:search_type;not null;default:''" json:"search_type"`
	// 各分区命中数（聚合搜索逐区落数，指定类型搜索只落该区，其余 0）——
	// 分区的粒度让「哪一类内容搜不到」可回答，总命中数由它们相加得出。
	CourseHits   int64 `gorm:"column:course_hits;not null;default:0" json:"course_hits"`
	ChapterHits  int64 `gorm:"column:chapter_hits;not null;default:0" json:"chapter_hits"`
	QuestionHits int64 `gorm:"column:question_hits;not null;default:0" json:"question_hits"`
	ContentHits  int64 `gorm:"column:content_hits;not null;default:0" json:"content_hits"`
	TopicHits    int64 `gorm:"column:topic_hits;not null;default:0" json:"topic_hits"`
	// TotalHits 该次搜索的总命中数（= 上面五列之和；0 且 search_type 为空 = 零结果搜索）。
	TotalHits int64     `gorm:"column:total_hits;not null;default:0" json:"total_hits"`
	CreatedAt time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SearchFact) TableName() string { return "search_fact" }
