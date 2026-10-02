package api

import (
	"forklift-training/internal/practicemode"
	"forklift-training/internal/questionbank"
	"forklift-training/internal/service"
	"forklift-training/internal/training"
)

// provideExam 练习与考试域（题库/练习/模考/真题/错题/互动/目录）。
func provideExam(c *coreSingletons, d *Deps) {
	d.QuestionBankSvc = questionbank.NewService(c.db, c.fileSvc, c.logger)
	d.PracticeModeSvc = practicemode.NewService(c.db, c.aiSvc, c.logger)
	d.MockExamSvc = service.NewMockExamService(c.db, c.aiSvc, c.logger)
	d.RealExamSvc = service.NewRealExamService(c.db, c.pointsSvc, c.logger)
	d.WrongQuestionSvc = service.NewWrongQuestionService(c.db, c.aiSvc, c.logger)
	d.QuestionCommentSvc = service.NewQuestionCommentService(c.db, c.logger)
	d.NoteSvc = service.NewNoteService(c.db, c.logger)
	d.QuestionKnowledgeSvc = service.NewQuestionKnowledgeService(c.db)
	d.TrainingCatalogSvc = training.NewService(c.db, c.logger)
	d.PointsSvc = c.pointsSvc
}
