// Package apitypes 前端契约类型的窄域 codegen（ADR-0019 专项第一步 / spec #940 片五③；
// 按域解冻的执行口径见 ADR-0048）。
//
// 与 ADR-0030 / ADR-0047 §1 / §5 的三条生成管道同构（声明表 → 渲染 → 生成物 → 字节级同步契约）：
// 输入是 swagger 产物（由注解经 make swagger 生成，CI 有新鲜度锁），
// 输出是 frontend/src/api/generated/<domain>.ts。
//
// 范围纪律：只做**声明表里登记的域**（ADR-0048 决策 2：Web 消费面 + 域内完整）。
package apitypes

// Endpoint 生成物头部列出的端点。
//
// 两个用途：一是写给生成物的读者看（这些类型从哪来），二是被 TestDomainEndpointsDeclareData
// 当作「本域注解完整性」的断言表 —— NoData 为真表示该端点**有意**没有返回载荷
// （注解显式为 response.R，浏览器只拿 errcode/errmsg），未登记又缺 data 指认的端点直接判红。
type Endpoint struct {
	Method string
	Path   string
	NoData bool
}

// Domain 一个域的生成声明。
//
// Roots 是该域消费的根类型（swagger definitions 里的键，如 service.CheckInResult）：
// 渲染时取它们的**传递闭包**（引用到的类型一并生成），故新增嵌套类型不必改声明表。
// 只增不改：域内新增端点/字段由注解层驱动，声明表只在「新增一个域」时动。
type Domain struct {
	// Name 生成物文件名（frontend/src/api/generated/<Name>.ts）。
	Name string
	// Title 生成物头部的一句话说明。
	Title string
	// Roots 根类型（Go 类型名，带 swagger definitions 的包前缀）。
	Roots []string
	// Endpoints 该域端点，仅用于生成物头部注释。
	Endpoints []Endpoint
}

// Domains 声明表。顺序 = ADR-0048 决策 7 的分片顺序，也是生成物的输出顺序。
// 域内完整性（决策 2）体现在 Roots/Endpoints 覆盖该域**全部** Web 消费端点，
// 而不是只挑「注解已就绪」的那几个。
var Domains = []Domain{
	{
		Name:  "checkin",
		Title: "每日打卡（/api/check-in/*，ADR-0028 独立蓝图）",
		Roots: []string{
			"service.CheckInResult",
			"service.CheckInCalendarResult",
			"service.CheckInRankResult",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/check-in"},
			{Method: "GET", Path: "/check-in/calendar"},
			{Method: "GET", Path: "/check-in/rank"},
		},
	},

	{
		Name:  "contribution",
		Title: "投稿与审核（/api/contributions/*、/api/admin/contributions/*，含举报队列）",
		Roots: []string{
			"service.ContributionFileDTO",
			"service.ContributionItemDTO",
			"service.ContributionPageResult",
			"service.ContributionReportPageResult",
			"service.DownloadResult",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/contributions/upload-file"},
			{Method: "POST", Path: "/contributions"},
			{Method: "GET", Path: "/contributions"},
			{Method: "GET", Path: "/contributions/mine"},
			{Method: "GET", Path: "/contributions/{id}"},
			{Method: "POST", Path: "/contributions/{id}/download"},
			{Method: "DELETE", Path: "/contributions/{id}", NoData: true},
			{Method: "POST", Path: "/contributions/{id}/report", NoData: true},
			{Method: "GET", Path: "/admin/contributions/pending"},
			{Method: "POST", Path: "/admin/contributions/{id}/approve"},
			{Method: "POST", Path: "/admin/contributions/{id}/reject"},
			{Method: "POST", Path: "/admin/contributions/{id}/archive"},
			{Method: "GET", Path: "/admin/contributions/reports"},
			{Method: "POST", Path: "/admin/contributions/reports/{id}/handle", NoData: true},
		},
	},

	{
		Name:  "practiceMode",
		Title: "题库练习模式（/api/practice-mode/*：随机 / 标签 / 顺序 / 进度 / 判定 / 统计 / 历史）",
		Roots: []string{
			"service.PracticeStartResultDTO",
			"service.ProgressResultDTO",
			"service.ProgressSaveResultDTO",
			"service.SubmitResultDTO",
			"service.PracticePracticeStatsDTO",
			"service.PracticeStatsDTO",
			"service.HistoryResultDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/practice-mode/free"},
			{Method: "GET", Path: "/practice-mode/tag"},
			{Method: "GET", Path: "/practice-mode/sequential"},
			{Method: "GET", Path: "/practice-mode/sequential-progress"},
			{Method: "POST", Path: "/practice-mode/progress"},
			{Method: "GET", Path: "/practice-mode/progress"},
			{Method: "POST", Path: "/practice-mode/submit"},
			{Method: "GET", Path: "/practice-mode/practice-stats"},
			{Method: "GET", Path: "/practice-mode/stats"},
			{Method: "GET", Path: "/practice-mode/history"},
		},
	},

	{
		Name:  "mockExam",
		Title: "模拟考试（/api/mock-exam/*，含真题卷整卷复用链路）",
		Roots: []string{
			"service.MockExamStartDTO",
			"service.MockExamResumeDTO",
			"service.MockExamSubmitDTO",
			"service.MockExamResultDTO",
			"service.MockExamHistoryDTO",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/mock-exam/start"},
			{Method: "POST", Path: "/mock-exam/{mock_exam_id}/save", NoData: true},
			{Method: "GET", Path: "/mock-exam/{mock_exam_id}/resume"},
			{Method: "POST", Path: "/mock-exam/{mock_exam_id}/submit"},
			{Method: "GET", Path: "/mock-exam/{mock_exam_id}/result"},
			{Method: "GET", Path: "/mock-exam/history"},
		},
	},

	{
		Name:  "auth",
		Title: "认证与账号（/api/auth/*、/api/captcha：登录 / 双令牌 / 资料 / 验证码 / 微信）",
		Roots: []string{
			"service.LoginResult",
			"service.ProfileDTO",
			"service.ProfileChangeRequestDTO",
			"service.RefreshResultDTO",
			"service.WxLoginResult",
			"service.WechatQRCodeInfoDTO",
			"api.GenerateCaptchaDTO",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/auth/login"},
			{Method: "POST", Path: "/auth/admin-login"},
			{Method: "POST", Path: "/auth/tutor-login"},
			{Method: "POST", Path: "/auth/recruiter-login"},
			{Method: "POST", Path: "/auth/logout", NoData: true},
			{Method: "POST", Path: "/auth/refresh"},
			{Method: "GET", Path: "/auth/me"},
			{Method: "PUT", Path: "/auth/profile"},
			{Method: "DELETE", Path: "/auth/account", NoData: true},
			{Method: "POST", Path: "/auth/avatar"},
			{Method: "POST", Path: "/auth/email/send-code", NoData: true},
			{Method: "POST", Path: "/auth/phone/send-code", NoData: true},
			{Method: "POST", Path: "/auth/email/register"},
			{Method: "POST", Path: "/auth/phone/register"},
			{Method: "POST", Path: "/auth/email/login"},
			{Method: "POST", Path: "/auth/phone/login"},
			{Method: "POST", Path: "/auth/email/reset-password", NoData: true},
			{Method: "POST", Path: "/auth/phone/reset-password", NoData: true},
			{Method: "GET", Path: "/captcha"},
			{Method: "POST", Path: "/auth/wechat/qrcode"},
			{Method: "POST", Path: "/auth/wx-login"},
			{Method: "POST", Path: "/auth/wechat/login"},
			{Method: "POST", Path: "/auth/profile/send-code", NoData: true},
			{Method: "POST", Path: "/auth/profile/email", NoData: true},
			{Method: "POST", Path: "/auth/profile/phone", NoData: true},
			{Method: "POST", Path: "/auth/profile/password", NoData: true},
			{Method: "POST", Path: "/auth/profile/password/send-code", NoData: true},
			{Method: "POST", Path: "/auth/account/send-code", NoData: true},
			{Method: "PUT", Path: "/auth/account"},
		},
	},

	{
		Name:  "forum",
		Title: "论坛（/api/forum/*、/api/admin/forum/*：帖子 / 回复 / 互动 / 举报 / 管理端）",
		Roots: []string{
			"service.ForumTopicPageResult",
			"service.ForumTopicDTO",
			"service.ForumTopicDetailDTO",
			"service.ForumReplyDTO",
			"service.MyReplyPageResult",
			"service.ForumReportPageResult",
			"service.ForumImageUploadResultDTO",
			"service.ForumLikeResultDTO",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/forum/upload-image"},
			{Method: "GET", Path: "/forum/topics"},
			{Method: "POST", Path: "/forum/topics"},
			{Method: "GET", Path: "/forum/topics/{id}"},
			{Method: "PUT", Path: "/forum/topics/{id}"},
			{Method: "POST", Path: "/forum/topics/{id}/replies"},
			{Method: "DELETE", Path: "/forum/topics/{id}", NoData: true},
			{Method: "DELETE", Path: "/forum/replies/{id}", NoData: true},
			{Method: "POST", Path: "/forum/topics/{id}/like"},
			{Method: "DELETE", Path: "/forum/topics/{id}/like"},
			{Method: "POST", Path: "/forum/topics/{id}/report", NoData: true},
			{Method: "POST", Path: "/forum/replies/{id}/report", NoData: true},
			{Method: "GET", Path: "/forum/my-topics"},
			{Method: "GET", Path: "/forum/my-replies"},
			{Method: "GET", Path: "/forum/my-liked-topics"},
			{Method: "GET", Path: "/forum/my-observed"},
			{Method: "GET", Path: "/forum/my-view-history"},
			{Method: "POST", Path: "/forum/replies/{id}/like"},
			{Method: "DELETE", Path: "/forum/replies/{id}/like"},
			{Method: "POST", Path: "/forum/topics/{id}/accept"},
			{Method: "DELETE", Path: "/forum/topics/{id}/accept"},
			{Method: "GET", Path: "/admin/forum/topics"},
			{Method: "GET", Path: "/admin/forum/topics/{id}"},
			{Method: "DELETE", Path: "/admin/forum/topics/{id}", NoData: true},
			{Method: "DELETE", Path: "/admin/forum/replies/{id}", NoData: true},
			{Method: "POST", Path: "/admin/forum/topics/{id}/featured"},
			{Method: "DELETE", Path: "/admin/forum/topics/{id}/featured"},
			{Method: "POST", Path: "/admin/forum/topics/{id}/experience"},
			{Method: "DELETE", Path: "/admin/forum/topics/{id}/experience"},
			{Method: "GET", Path: "/admin/forum/reports"},
			{Method: "PUT", Path: "/admin/forum/reports/{id}", NoData: true},
		},
	},

	{
		Name:  "notification",
		Title: "站内通知（/api/notifications/*：列表 / 未读数 / 已读标记）",
		Roots: []string{
			"service.NotificationListPageResult",
			"service.NotificationUnreadCountDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/notifications"},
			{Method: "GET", Path: "/notifications/unread-count"},
			{Method: "POST", Path: "/notifications/{id}/read", NoData: true},
			{Method: "POST", Path: "/notifications/read-all", NoData: true},
		},
	},

	{
		Name:  "favorite",
		Title: "收藏（/api/favorites/*：多态收藏列表 / 收藏 / 取消 / 状态查询，ADR-0018）",
		Roots: []string{
			"service.FavoritePageResult",
			"service.FavoriteDTO",
			"service.FavoriteCheckDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/favorites"},
			{Method: "POST", Path: "/favorites"},
			{Method: "DELETE", Path: "/favorites/{id}", NoData: true},
			{Method: "GET", Path: "/favorites/check"},
		},
	},

	{
		Name:  "wrongQuestion",
		Title: "错题本（/api/wrong-questions/*：列表 / 重做 / 移出 / 统计 / 导出）",
		Roots: []string{
			"service.WrongQuestionPageDTO",
			"service.WrongQuestionRemoveResultDTO",
			"service.WrongQuestionBatchRemoveResultDTO",
			"service.WrongQuestionStatsDTO",
			"service.SubmitResultDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/wrong-questions"},
			{Method: "POST", Path: "/wrong-questions/{question_id}/redo"},
			{Method: "POST", Path: "/wrong-questions/{question_id}/remove"},
			{Method: "POST", Path: "/wrong-questions/batch-remove"},
			{Method: "GET", Path: "/wrong-questions/stats"},
			{Method: "GET", Path: "/wrong-questions/export", NoData: true},
		},
	},

	{
		Name:  "questionInteraction",
		Title: "题目互动（/api/questions/*：评论 / 笔记 / 考点标签）",
		Roots: []string{
			"service.QuestionCommentPageResult",
			"service.QuestionCommentDTO",
			"model.QuestionNote",
			"model.QuestionTag",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/questions/{question_id}/comments"},
			{Method: "POST", Path: "/questions/{question_id}/comments"},
			{Method: "DELETE", Path: "/questions/comments/{comment_id}", NoData: true},
			{Method: "GET", Path: "/questions/{question_id}/note"},
			{Method: "PUT", Path: "/questions/{question_id}/note"},
			{Method: "DELETE", Path: "/questions/{question_id}/note", NoData: true},
			{Method: "GET", Path: "/questions/{question_id}/knowledge"},
		},
	},

	{
		Name:  "recruit",
		Title: "招聘者工作区（/api/recruit/me、/api/recruit/resumes*、/api/recruit/contact-requests：简历库与联系方式交换）",
		Roots: []string{
			"service.RecruitResumeCard",
			"service.RecruitListResult",
			"service.ContactRequestListResult",
			"service.ContactPlainDTO",
			"service.RecruitMeDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/recruit/me"},
			{Method: "GET", Path: "/recruit/resumes"},
			{Method: "GET", Path: "/recruit/resumes/{id}"},
			{Method: "GET", Path: "/recruit/resumes/{id}/contact"},
			{Method: "GET", Path: "/recruit/contact-requests"},
			{Method: "POST", Path: "/recruit/contact-requests"},
			// 非统一信封：招聘者预览的打码在线简历 PDF 是 inline 二进制流（无 data，NoData 显式登记）。
			{Method: "GET", Path: "/recruit/resumes/{id}/pdf", NoData: true},
		},
	},

	{
		Name:  "job",
		Title: "职位与投递（/api/recruit/jobs*、/api/jobs*、/api/resume/applications*、/api/recruit/applications*）",
		Roots: []string{
			"service.JobPostingDTO",
			"service.JobListResult",
			"service.ApplicationDTO",
			"service.ApplicationListResult",
			"service.RecruiterApplicationListResult",
			"service.ReportDTO",
		},
		Endpoints: []Endpoint{
			{Method: "POST", Path: "/recruit/jobs"},
			{Method: "PUT", Path: "/recruit/jobs/{id}"},
			{Method: "POST", Path: "/recruit/jobs/{id}/toggle-status"},
			{Method: "GET", Path: "/recruit/jobs"},
			{Method: "GET", Path: "/recruit/jobs/{id}"},
			{Method: "GET", Path: "/jobs"},
			{Method: "GET", Path: "/jobs/{id}"},
			{Method: "POST", Path: "/jobs/{id}/apply"},
			{Method: "POST", Path: "/jobs/{id}/report"},
			{Method: "GET", Path: "/resume/applications"},
			{Method: "POST", Path: "/resume/applications/{id}/withdraw"},
			{Method: "GET", Path: "/recruit/jobs/{id}/applications"},
			{Method: "GET", Path: "/recruit/applications/{id}"},
			{Method: "POST", Path: "/recruit/applications/{id}/reject"},
		},
	},

	{
		Name:  "resume",
		Title: "学员简历卡（/api/resume/*：简历 CRUD / 可见性 / PDF 与工作照附件 / 查看留痕 / 收到的联系方式申请）",
		Roots: []string{
			"service.JobCardDTO",
			"service.ContactRequestListResult",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/resume"},
			{Method: "PUT", Path: "/resume"},
			{Method: "PUT", Path: "/resume/visibility"},
			{Method: "POST", Path: "/resume/pdf"},
			// 有意无载荷：响应 data 是空对象（response.SuccessWithMsg(c, "附件已删除", gin.H{})），
			// 前端不消费任何字段（片二口径的「handler 手工拼响应体」；本片只补注解，不动构造）。
			{Method: "DELETE", Path: "/resume/pdf", NoData: true},
			{Method: "POST", Path: "/resume/image"},
			{Method: "GET", Path: "/resume/view-stats"},
			{Method: "GET", Path: "/resume/contact-requests"},
			{Method: "POST", Path: "/resume/contact-requests/{id}/approve"},
			{Method: "POST", Path: "/resume/contact-requests/{id}/reject"},
			{Method: "POST", Path: "/resume/contact-requests/{id}/revoke"},
			// 非统一信封：学员预览自己的在线简历 PDF 是 inline 二进制流（无 data，NoData 显式登记）。
			{Method: "GET", Path: "/resume/pdf", NoData: true},
		},
	},

	{
		Name:  "questionBank",
		Title: "题库管理（/api/question-bank/*：题目 CRUD / 批量发布驳回导入 / 统计 / 图片上传）",
		Roots: []string{
			"service.QuestionBankStatsDTO",
			"service.QuestionDTO",
			"service.QuestionImageUploadDTO",
			"service.QuestionImportResultDTO",
			"service.QuestionPageDTO",
			"service.QuestionPublishResultDTO",
			"service.QuestionRejectResultDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/question-bank/questions"},
			{Method: "POST", Path: "/question-bank/questions"},
			{Method: "GET", Path: "/question-bank/questions/{question_id}"},
			{Method: "PUT", Path: "/question-bank/questions/{question_id}"},
			{Method: "DELETE", Path: "/question-bank/questions/{question_id}", NoData: true},
			{Method: "POST", Path: "/question-bank/questions/{question_id}/publish"},
			{Method: "POST", Path: "/question-bank/questions/{question_id}/reject"},
			{Method: "POST", Path: "/question-bank/questions/batch-publish"},
			{Method: "POST", Path: "/question-bank/questions/batch-reject"},
			{Method: "POST", Path: "/question-bank/questions/batch-import"},
			{Method: "GET", Path: "/question-bank/stats"},
			{Method: "POST", Path: "/question-bank/upload-image"},
		},
	},

	{
		Name:  "tutor",
		Title: "讲师端课程与章节（/api/tutor/*：课程列表 / 章节详情 / 文件上传删除）",
		Roots: []string{
			"service.BatchDeleteFilesResult",
			"service.ChapterDTO",
			"service.ChapterDetailDTO",
			"service.ChapterFileDTO",
			"service.CoursePageResult",
			"service.DeleteFileResult",
			"service.TutorCourseChaptersDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/tutor/courses"},
			{Method: "GET", Path: "/tutor/course/{course_id}/chapters"},
			{Method: "GET", Path: "/tutor/chapter/{chapter_id}"},
			{Method: "POST", Path: "/tutor/chapter/{chapter_id}/upload"},
			{Method: "PUT", Path: "/tutor/chapter/{chapter_id}"},
			{Method: "DELETE", Path: "/tutor/file/{file_id}"},
			{Method: "POST", Path: "/tutor/files/batch-delete"},
		},
	},

	{
		Name:  "training",
		Title: "培训目录与字典（/api/catalog|levels|tags、/api/admin 下的专业方向 / 等级 / 证书模板 / 题库标签）",
		Roots: []string{
			"service.CatalogTreeDTO",
			"service.CertificateTemplateDict",
			"service.CertificateTemplateListDTO",
			"service.LevelDict",
			"service.LevelListDTO",
			"service.QuestionTagDict",
			"service.QuestionTagListDTO",
			"service.QuestionTagsResultDTO",
			"service.SpecialtyDict",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/catalog/tree"},
			{Method: "GET", Path: "/levels"},
			{Method: "GET", Path: "/admin/catalog/tree"},
			{Method: "POST", Path: "/admin/specialty"},
			{Method: "PUT", Path: "/admin/specialty/{specialty_id}"},
			{Method: "PUT", Path: "/admin/specialty/{specialty_id}/sort", NoData: true},
			{Method: "DELETE", Path: "/admin/specialty/{specialty_id}", NoData: true},
			{Method: "POST", Path: "/admin/level"},
			{Method: "PUT", Path: "/admin/level/{level_id}"},
			{Method: "PUT", Path: "/admin/level/{level_id}/sort", NoData: true},
			{Method: "DELETE", Path: "/admin/level/{level_id}", NoData: true},
			{Method: "GET", Path: "/admin/certificate-templates"},
			{Method: "POST", Path: "/admin/certificate-template"},
			{Method: "PUT", Path: "/admin/certificate-template/{id}"},
			{Method: "DELETE", Path: "/admin/certificate-template/{id}", NoData: true},
			{Method: "GET", Path: "/tags"},
			{Method: "GET", Path: "/admin/question-tags"},
			{Method: "POST", Path: "/admin/question-tag"},
			{Method: "PUT", Path: "/admin/question-tag/{id}"},
			{Method: "DELETE", Path: "/admin/question-tag/{id}", NoData: true},
			{Method: "PUT", Path: "/admin/question/{question_id}/tags"},
		},
	},

	{
		Name:  "search",
		Title: "全局搜索（/api/search：type 缺省为分区聚合，指定 type 为分页结果）",
		Roots: []string{
			"service.SearchAllDTO",
			"service.SearchPageDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/search"},
		},
	},

	{
		Name:  "course",
		Title: "课程与章节学习（/api/courses、/api/course/*、/api/chapter/*）",
		Roots: []string{
			"service.ChapterDetailDTO",
			"service.ChapterSlidesDTO",
			"service.CourseDetailDTO",
			"service.CoursePageResult",
			"service.StudyProgressDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/courses"},
			{Method: "GET", Path: "/course/{course_id}"},
			{Method: "POST", Path: "/course/{course_id}/progress"},
			{Method: "GET", Path: "/course/{course_id}/chapter/{chapter_id}"},
			{Method: "GET", Path: "/chapter/{chapter_id}/slides"},
			{Method: "POST", Path: "/chapter/{chapter_id}/slides/regenerate"},
		},
	},

	{
		Name:  "material",
		Title: "学习资料（/api/materials：chapter_file 聚合视图）",
		Roots: []string{
			"service.MaterialPageResult",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/materials"},
		},
	},

	{
		Name:  "realExam",
		Title: "真题套卷（/api/real-exam/*：列表 / 按卷练习 / 按卷开考 / 积分兑换）",
		Roots: []string{
			"service.MockExamStartDTO",
			"service.PracticeStartResultDTO",
			"service.RealExamPaperDTO",
			"service.RedeemResult",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/real-exam/papers"},
			{Method: "GET", Path: "/real-exam/papers/{paper_id}/practice"},
			{Method: "POST", Path: "/real-exam/papers/{paper_id}/exam"},
			{Method: "POST", Path: "/real-exam/papers/{paper_id}/redeem"},
		},
	},

	{
		Name:  "student",
		Title: "学员学习中心（/api/student/*：档案 / 学习记录 / 按天统计 / 我的课程）",
		Roots: []string{
			"service.StudentCourseDetailDTO",
			"service.StudentCoursesDTO",
			"service.StudentProfileDTO",
			"service.StudyDailyStatsDTO",
			"service.StudyRecordPageResult",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/student/profile"},
			{Method: "GET", Path: "/student/records"},
			{Method: "GET", Path: "/student/study-stats"},
			{Method: "GET", Path: "/student/courses"},
			{Method: "GET", Path: "/student/courses/{course_id}"},
		},
	},

	{
		Name:  "credential",
		Title: "目标证件（/api/credentials*、/api/me/credential：公开列表 / 分组 / 管理端 CRUD / 当前证件）",
		Roots: []string{
			"service.CredentialDict",
			"service.CredentialListDTO",
			"service.CurrentCredentialDTO",
			"service.GroupedCredentialsDTO",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/credentials"},
			{Method: "GET", Path: "/credentials/grouped"},
			{Method: "GET", Path: "/admin/credentials"},
			{Method: "POST", Path: "/admin/credential"},
			{Method: "PUT", Path: "/admin/credential/{id}"},
			{Method: "DELETE", Path: "/admin/credential/{id}", NoData: true},
			{Method: "PUT", Path: "/admin/credential/{id}/sort", NoData: true},
			{Method: "GET", Path: "/me/credential"},
			{Method: "PATCH", Path: "/me/credential"},
		},
	},

	{
		Name:  "admin",
		Title: "管理端（/api/admin/*：HRWAI 用户 / 导师 / 招聘者 / 课程 / 章节 / 统计 / AI 配置 / 资料审核 / 审计 / 数据导出）",
		// 本域根类型 = 全部 Web 消费端点的 data 指认（渲染取传递闭包：课程域类型随之重复包含，
		// 跨域共享文件不在本片范围，先例见 ADR-0048 片一「生成物按域重复包含共享类型」）。
		Roots: []string{
			"service.HrwaiUserPageResult",
			"service.HrwaiUserCreatedDTO",
			"service.StatusResultDTO",
			"service.TutorListDTO",
			"service.TutorRegisterResultDTO",
			"service.TutorDeletedDTO",
			"service.RecruiterListResult",
			"service.RecruiterCreatedDTO",
			"service.RecruiterUpdatedDTO",
			"service.RecruiterPasswordResetResult",
			"service.AdminStatisticsDTO",
			"service.GenerateContentResultDTO",
			"service.GenTaskStatus",
			"service.CoursePageResult",
			"service.AdminCourseDetailDTO",
			"service.DeleteCourseResult",
			"service.DeleteChapterResult",
			"service.AIConfigDTO",
			"service.FeatureBindingDTO",
			"service.ProfileChangeRequestPageResult",
			"api.AuditLogPageResult",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/admin/hrwai-users"},
			{Method: "POST", Path: "/admin/hrwai-users"},
			{Method: "PUT", Path: "/admin/hrwai-users/{id}", NoData: true},
			{Method: "PUT", Path: "/admin/hrwai-users/{id}/password", NoData: true},
			{Method: "PUT", Path: "/admin/hrwai-users/{id}/status"},
			{Method: "DELETE", Path: "/admin/hrwai-users/{id}", NoData: true},
			{Method: "GET", Path: "/admin/tutors"},
			{Method: "POST", Path: "/admin/tutor"},
			{Method: "DELETE", Path: "/admin/tutor/{tutor_id}"},
			{Method: "PUT", Path: "/admin/tutor/{tutor_id}/password", NoData: true},
			{Method: "PUT", Path: "/admin/tutor/{tutor_id}/status"},
			{Method: "GET", Path: "/admin/recruiters"},
			{Method: "POST", Path: "/admin/recruiters"},
			{Method: "PUT", Path: "/admin/recruiters/{id}/status"},
			{Method: "PUT", Path: "/admin/recruiters/{id}"},
			{Method: "PUT", Path: "/admin/recruiters/{id}/password"},
			{Method: "GET", Path: "/admin/statistics"},
			{Method: "POST", Path: "/admin/course/generate-content"},
			{Method: "GET", Path: "/admin/course/generate-content/{task_id}"},
			{Method: "GET", Path: "/admin/courses"},
			{Method: "GET", Path: "/admin/course/{course_id}"},
			{Method: "POST", Path: "/admin/course"},
			{Method: "PUT", Path: "/admin/course/{course_id}"},
			{Method: "PUT", Path: "/admin/course/{course_id}/sort", NoData: true},
			{Method: "DELETE", Path: "/admin/course/{course_id}"},
			{Method: "POST", Path: "/admin/course/{course_id}/chapter"},
			{Method: "PUT", Path: "/admin/chapter/{chapter_id}"},
			{Method: "DELETE", Path: "/admin/chapter/{chapter_id}"},
			{Method: "GET", Path: "/admin/ai-configs"},
			{Method: "POST", Path: "/admin/ai-configs", NoData: true},
			{Method: "PUT", Path: "/admin/ai-configs/{id}", NoData: true},
			{Method: "DELETE", Path: "/admin/ai-configs/{id}", NoData: true},
			{Method: "POST", Path: "/admin/ai-configs/{id}/test", NoData: true},
			{Method: "GET", Path: "/admin/ai-feature-bindings"},
			{Method: "PUT", Path: "/admin/ai-feature-bindings/{feature_key}", NoData: true},
			{Method: "DELETE", Path: "/admin/ai-feature-bindings/{feature_key}/configs/{config_id}", NoData: true},
			{Method: "GET", Path: "/admin/profile-reviews"},
			{Method: "POST", Path: "/admin/profile-reviews/{id}/approve"},
			{Method: "POST", Path: "/admin/profile-reviews/{id}/reject"},
			{Method: "GET", Path: "/admin/audit-logs"},
			{Method: "GET", Path: "/admin/export/students", NoData: true},
			{Method: "GET", Path: "/admin/export/questions", NoData: true},
			{Method: "GET", Path: "/admin/export/evaluations", NoData: true},
		},
	},

	{
		Name:  "points",
		Title: "积分（/api/points/*：余额 / 流水 / 任务中心 / 课程与商城兑换）",
		Roots: []string{
			"service.PointsBalanceResult",
			"service.PointsLedgerResult",
			"service.PointsTasksResult",
			"service.PointsClaimResult",
			"service.RedeemResult",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/points/balance"},
			{Method: "GET", Path: "/points/ledger"},
			{Method: "GET", Path: "/points/tasks"},
			{Method: "POST", Path: "/points/tasks/{code}/claim"},
			{Method: "POST", Path: "/points/shop/course/{courseId}/redeem"},
			{Method: "POST", Path: "/points/shop/{sku}/redeem"},
		},
	},

	{
		Name:  "featured",
		Title: "内容精选管理端（/api/admin/featured-content*：列表 / 详情 / 增删改 / 发布 / 图片上传）",
		// 公开精选端点（/featured-contents、/featured-content/{id}、/featured-content/{id}/view）
		// 的消费者是独立 Nuxt 门户仓（ADR-0001），不在本仓 frontend/src/api/** 消费面内，故不登记。
		Roots: []string{
			"service.FeaturedContentPageResult",
			"service.FeaturedContentAdminDetailDTO",
			"service.FeaturedDeleteResult",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/admin/featured-contents"},
			{Method: "GET", Path: "/admin/featured-content/{id}"},
			{Method: "POST", Path: "/admin/featured-content"},
			{Method: "PUT", Path: "/admin/featured-content/{id}"},
			{Method: "DELETE", Path: "/admin/featured-content/{id}"},
			{Method: "POST", Path: "/admin/featured-content/{id}/publish"},
			{Method: "POST", Path: "/admin/featured-content/upload-image", NoData: true},
		},
	},

	{
		// ADR-0048 决策 7 的第八片。SSE 流式对话（POST /ai-assistant/chat）与事件 payload
		// 按决策 6 明确排除：它不走统一信封、没有 data 类型可指认，故**不登记**在端点表里
		//（排除口径写在 Title，会渲染进生成物头部）。
		Name:  "aiAssistant",
		Title: "AI 助手信封面（/api/ai-assistant/*：模型 / 会话 / 消息 / 用户模型 / 诊断字典）；SSE 流式对话 POST /ai-assistant/chat 不走统一信封、事件 payload 不定型，不在本域生成面（ADR-0048 决策 6）",
		Roots: []string{
			"service.ModelOption",
			"service.AIAssistantModeModels",
			"service.UserModelDTO",
			"service.AIChatSessionDTO",
			"service.AIChatMessageDTO",
			"service.AISessionRenameResultDTO",
			"service.AIImageUploadResultDTO",
			"service.DiagnosisBrandOption",
			"service.DiagnosisFaultCodePage",
		},
		Endpoints: []Endpoint{
			{Method: "GET", Path: "/ai-assistant/models"},
			{Method: "GET", Path: "/ai-assistant/modes"},
			{Method: "GET", Path: "/ai-assistant/user-models"},
			{Method: "POST", Path: "/ai-assistant/user-models", NoData: true},
			{Method: "DELETE", Path: "/ai-assistant/user-models/{id}", NoData: true},
			{Method: "GET", Path: "/ai-assistant/sessions"},
			{Method: "POST", Path: "/ai-assistant/sessions"},
			{Method: "DELETE", Path: "/ai-assistant/sessions/{id}", NoData: true},
			{Method: "PATCH", Path: "/ai-assistant/sessions/{id}/title"},
			{Method: "GET", Path: "/ai-assistant/sessions/{id}/messages"},
			{Method: "POST", Path: "/ai-assistant/upload-image"},
			{Method: "GET", Path: "/ai-assistant/diagnosis/brands"},
			{Method: "GET", Path: "/ai-assistant/diagnosis/models"},
			{Method: "GET", Path: "/ai-assistant/diagnosis/fault-codes"},
			// 手册静态资源是原样字节流（非信封），无 data 指认 —— 显式登记为有意无载荷。
			{Method: "GET", Path: "/ai-assistant/diagnosis/manual/{filepath}", NoData: true},
		},
	},

	{
		Name:  "valuation",
		Title: "残值评估（/api/valuation/*：字典 / 评估 / 电池 RUL / 报告 / 平行 auth / 管理端 CRUD）",
		Roots: []string{
			"model.EvaluationResponse",
			"model.EvaluationDetail",
			"model.DimensionScore",
			"model.CreateBatteryResponse",
			"model.ListBatteryResponse",
			"model.BatteryEvaluation",
			"model.BatteryEvaluationSummary",
			"model.CycleFeature",
			"model.RawStats",
			"model.FeatureImportance",
			"repository.AlgorithmParameters",
			"repository.Brand",
			"repository.VehicleType",
			"repository.Series",
			"repository.Tonnage",
			"repository.MastType",
			"repository.MastHeight",
			"repository.BatteryTypeDict",
			"repository.TransmissionType",
			"repository.EngineType",
			"repository.SeriesConfigOptions",
			"repository.ConditionRating",
			"repository.RegionCoefficient",
			"repository.CoefficientConfig",
			"repository.OriginalPrice",
			"repository.ConfigOption",
		},
		// 域内完整（ADR-0048 决策 2）：估值 Web 消费面（frontend/src/api/valuation/*，
		// baseURL 前缀 /api/valuation）+ 该域全部已注册路由。
		// NoData：估值登出（refresh_token 自证身份吊销，响应 data 为 null 是有意的）。
		// 非统一信封端点（gin.H / 裸 c.JSON，7 处）**登记不改造**（issue #967 决策 2）：
		// 注解用 swag 内联 object{} 如实描述形状，故仍有 data 指认、不登记 NoData。
		Endpoints: []Endpoint{
			// 公开组：字典只读
			{Method: "GET", Path: "/valuation/dictionaries/brands"},
			{Method: "GET", Path: "/valuation/dictionaries/vehicle-types"},
			{Method: "GET", Path: "/valuation/dictionaries/series"},
			{Method: "GET", Path: "/valuation/dictionaries/tonnages"},
			{Method: "GET", Path: "/valuation/dictionaries/config-types"},
			{Method: "GET", Path: "/valuation/dictionaries/mast-types"},
			{Method: "GET", Path: "/valuation/dictionaries/mast-heights"},
			{Method: "GET", Path: "/valuation/dictionaries/battery-types"},
			{Method: "GET", Path: "/valuation/dictionaries/transmission-types"},
			{Method: "GET", Path: "/valuation/dictionaries/engine-types"},
			{Method: "GET", Path: "/valuation/dictionaries/series-config-options"},
			{Method: "GET", Path: "/valuation/dictionaries/condition-ratings"},
			{Method: "GET", Path: "/valuation/dictionaries/provinces"},
			{Method: "GET", Path: "/valuation/dictionaries/cities"},
			{Method: "GET", Path: "/valuation/dictionaries/coefficient-configs"},
			{Method: "GET", Path: "/valuation/dictionaries/earliest-factory-year"},
			// 公开组：管理端聚合与分页（admin.ts 消费）
			{Method: "GET", Path: "/valuation/dictionaries/algorithm-parameters"},
			{Method: "GET", Path: "/valuation/dictionaries/original-prices"},
			{Method: "GET", Path: "/valuation/dictionaries/region-coefficients"},
			// 公开组：评估统计与报告生成
			{Method: "GET", Path: "/valuation/evaluations/stats"},
			{Method: "POST", Path: "/valuation/evaluations/{id}/report"},
			{Method: "GET", Path: "/valuation/evaluations/{id}/report", NoData: true},
			{Method: "POST", Path: "/valuation/battery/evaluations/{id}/report"},
			{Method: "GET", Path: "/valuation/battery/evaluations/{id}/report", NoData: true},
			// 公开组：估值登出（有意无载荷）
			{Method: "POST", Path: "/valuation/auth/logout", NoData: true},
			// 可选认证组：匿名可提交（登录则记录 user_id）
			{Method: "POST", Path: "/valuation/evaluations"},
			{Method: "POST", Path: "/valuation/battery/evaluations"},
			// 估值鉴权组（JWTAuth + CapValuationUse）
			{Method: "GET", Path: "/valuation/evaluations"},
			{Method: "GET", Path: "/valuation/evaluations/{id}"},
			{Method: "GET", Path: "/valuation/battery/evaluations"},
			{Method: "GET", Path: "/valuation/battery/evaluations/{id}"},
			{Method: "GET", Path: "/valuation/auth/me"},
			// 管理端写面（JWTAuth + CapValuationConfig + AuditLog；描述符注册表驱动）
			{Method: "POST", Path: "/valuation/admin/original-prices"},
			{Method: "PUT", Path: "/valuation/admin/original-prices/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/original-prices/{id}"},
			{Method: "POST", Path: "/valuation/admin/region-coefficients"},
			{Method: "PUT", Path: "/valuation/admin/region-coefficients/{id}"},
			{Method: "PUT", Path: "/valuation/admin/coefficient-configs/{key}"},
			{Method: "POST", Path: "/valuation/admin/brands"},
			{Method: "PUT", Path: "/valuation/admin/brands/{id}"},
			{Method: "POST", Path: "/valuation/admin/vehicle-types"},
			{Method: "PUT", Path: "/valuation/admin/vehicle-types/{id}"},
			{Method: "POST", Path: "/valuation/admin/series"},
			{Method: "PUT", Path: "/valuation/admin/series/{id}"},
			{Method: "POST", Path: "/valuation/admin/tonnages"},
			{Method: "POST", Path: "/valuation/admin/mast-types"},
			{Method: "POST", Path: "/valuation/admin/mast-heights"},
			{Method: "POST", Path: "/valuation/admin/battery-types"},
			{Method: "POST", Path: "/valuation/admin/transmission-types"},
			{Method: "POST", Path: "/valuation/admin/engine-types"},
			{Method: "POST", Path: "/valuation/admin/condition-ratings"},
			{Method: "PUT", Path: "/valuation/admin/condition-ratings/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/brands/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/vehicle-types/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/series/{id}"},
			{Method: "PUT", Path: "/valuation/admin/tonnages/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/tonnages/{id}"},
			{Method: "PUT", Path: "/valuation/admin/mast-types/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/mast-types/{id}"},
			{Method: "PUT", Path: "/valuation/admin/mast-heights/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/mast-heights/{id}"},
			{Method: "PUT", Path: "/valuation/admin/battery-types/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/battery-types/{id}"},
			{Method: "PUT", Path: "/valuation/admin/transmission-types/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/transmission-types/{id}"},
			{Method: "PUT", Path: "/valuation/admin/engine-types/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/engine-types/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/condition-ratings/{id}"},
			{Method: "DELETE", Path: "/valuation/admin/region-coefficients/{id}"},
		},
	},
}
