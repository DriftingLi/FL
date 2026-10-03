package api

import (
	"forklift-training/internal/mockexam"
	"forklift-training/internal/note"
	"forklift-training/internal/practicemode"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/questioninteraction"
	"forklift-training/internal/realexam"
	"forklift-training/internal/training"
	"forklift-training/internal/wrongquestion"
)

// provideExam 练习与考试域（题库/练习/模考/真题/错题/互动/目录）。
func provideExam(c *coreSingletons, d *Deps) {
	d.QuestionBankSvc = questionbank.NewService(c.db, c.fileSvc, c.logger)
	d.PracticeModeSvc = practicemode.NewService(c.db, c.aiSvc, c.logger)
	d.MockExamSvc = mockexam.NewService(c.db, c.aiSvc, c.logger)
	d.RealExamSvc = realexam.NewService(c.db, c.pointsSvc, c.logger)
	d.WrongQuestionSvc = wrongquestion.NewService(c.db, c.aiSvc, c.logger)
	d.QuestionCommentSvc = questioninteraction.NewService(c.db, c.logger)
	d.NoteSvc = note.NewService(c.db, c.logger)
	d.QuestionKnowledgeSvc = questioninteraction.NewKnowledgeService(c.db)
	d.TrainingCatalogSvc = training.NewService(c.db, c.logger)
	d.PointsSvc = c.pointsSvc
}
