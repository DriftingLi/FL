package model

// 模型清单一处声明（ADR-0047 §6 / spec #933）：模型按**域块**成块声明，块内与块间顺序
// **逐字保持**原 testutil 中心表的顺序——AutoMigrate 依赖这个顺序建外键。
//
// 为什么放在 model 包：清单是领域事实（哪些表属于哪个域），不是测试设施细节；
// testutil 只做汇总（NewMemoryDB/NewFileDB 调 AllModels()），不再维护第二份平表。
//
// 新增模型 = 在所属域块里加一行；漏加的症状是「测试里表不存在」，由本包的形状锁兜住。

// ModelBlock 一个域块：域名 + 该块的模型（按外键依赖顺序）。
type ModelBlock struct {
	Name   string
	Models []any
}

// ModelBlocks 全部域块（顺序即建表顺序）。
var ModelBlocks = []ModelBlock{
	{
		Name: "账号与身份",
		Models: []any{
			&Credential{},
			&HrwaiUser{},
			&RecruiterUser{},
			&JobCard{},
		},
	},
	{
		Name: "通知、审计与账号管理",
		Models: []any{
			&Notification{},
			&AuditLog{},
			&ProfileChangeRequest{},
			&Admin{},
			&Tutor{},
		},
	},
	{
		Name: "培训目录与学习",
		Models: []any{
			&Course{},
			&Specialty{},
			&Position{},
			&CourseLevel{},
			&CertificateTemplate{},
			&CoursePrerequisite{},
			&Chapter{},
			&ChapterFile{},
			&StudyRecord{},
		},
	},
	{
		Name: "题库与练习",
		Models: []any{
			&Question{},
			&QuestionTag{},
			&QuestionTagRelation{},
			&QuestionComment{},
			&QuestionNote{},
			&QuestionPracticeRecord{},
			&PracticeProgress{},
			&WrongQuestion{},
		},
	},
	{
		Name: "考试",
		Models: []any{
			&MockExam{},
			&RealExamPaper{},
			&RealExamPaperQuestion{},
		},
	},
	{
		Name: "论坛与互动",
		Models: []any{
			&ForumTopic{},
			&ForumReply{},
			&ForumTopicLike{},
			&ForumReplyLike{},
			&ForumCheckIn{},
			&ForumTopicView{},
			&Favorite{},
			&ForumReport{},
		},
	},
	{
		Name: "横切与配置",
		Models: []any{
			&AIGenerationLog{},
			&AsyncTask{},
			&FeaturedContent{},
			&SearchFact{},
			&SystemSetting{},
		},
	},
	{
		Name: "AI",
		Models: []any{
			&AIConfig{},
			&AIFeatureBinding{},
			&AIChatSession{},
			&AIChatMessage{},
			&AIUserModel{},
		},
	},
	{
		Name: "积分",
		Models: []any{
			&PointsLedger{},
			&PointsTaskConfig{},
			&PointsTaskClaim{},
			&PointsUserProgress{},
			&UserDailyLogin{},
			&PointsShopItem{},
			&UserEntitlement{},
			&PointsEntryIdem{},
		},
	},
	{
		Name: "招聘",
		Models: []any{
			&RecruitResumeView{},
			&ContactRequest{},
			&JobPosting{},
			&JobApplication{},
			&JobReport{},
		},
	},
	{
		Name: "投稿",
		Models: []any{
			&UserContribution{},
			&UserContributionFile{},
			&ContributionDownload{},
			&ContributionReport{},
		},
	},
}

// AllModels 按域块顺序汇总全部模型（建表顺序的唯一来源）。
func AllModels() []any {
	total := 0
	for _, b := range ModelBlocks {
		total += len(b.Models)
	}
	out := make([]any, 0, total)
	for _, b := range ModelBlocks {
		out = append(out, b.Models...)
	}
	return out
}
