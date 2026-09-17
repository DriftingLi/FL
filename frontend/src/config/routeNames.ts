/**
 * 路由名单一事实源（ADR-0027 C6）：
 * router/index.ts 的 route.name 与 config/navigation.ts 的 NavItem.routeName /
 * activeRouteNames 都引用本表，`RouteName` 为其常量值 union。
 * 改错路由名 / 改漏一处同步时，编译期即刻报错，导航断链不可能静默上线。
 *
 * 新增页面 = 在此表加一行 + router 引用 + navigation 引用。
 */
export const routeNames = {
  // 登录 / 注册 / 找回密码
  Login: 'Login',
  Register: 'Register',
  ForgotPassword: 'ForgotPassword',

  // 培训模块 - 学员子区
  StudentDashboard: 'StudentDashboard',
  CourseList: 'CourseList',
  StudentSearch: 'StudentSearch',
  StudentMaterials: 'StudentMaterials',
  StudentFavorites: 'StudentFavorites',
  StudentNotebook: 'StudentNotebook',
  StudentHelpCenter: 'StudentHelpCenter',
  ForumPage: 'ForumPage',
  ForumAsk: 'ForumAsk',
  ForumDetail: 'ForumDetail',
  /** 「即将离开本站」中转页（#881 / ADR-0044） */
  LinkOut: 'LinkOut',
  ChapterView: 'ChapterView',
  /** 搜索结果落点：内容精选详情（ADR-0049 决策 4） */
  StudentFeaturedDetail: 'StudentFeaturedDetail',
  /** 搜索结果落点：题目承载页（ADR-0049 决策 4） */
  StudentQuestionDetail: 'StudentQuestionDetail',
  QuestionBank: 'QuestionBank',
  MockExam: 'MockExam',
  WrongQuestions: 'WrongQuestions',
  RealExamPapers: 'RealExamPapers',
  RealExamPractice: 'RealExamPractice',
  CheckIn: 'CheckIn',
  TaskCenter: 'TaskCenter',
  PointsLedger: 'PointsLedger',
  StudentProfile: 'StudentProfile',
  StudentResume: 'StudentResume',
  StudentResumeEdit: 'StudentResumeEdit',
  JobPlaza: 'JobPlaza',
  JobDetail: 'JobDetail',
  MyApplications: 'MyApplications',
  CredentialOnboarding: 'CredentialOnboarding',

  // 培训模块 - 导师子区
  TutorDashboard: 'TutorDashboard',
  TutorCourses: 'TutorCourses',
  TutorChapterManage: 'TutorChapterManage',
  TutorChapterEdit: 'TutorChapterEdit',
  TutorQuestionManage: 'TutorQuestionManage',
  TutorQuestionCreate: 'TutorQuestionCreate',
  TutorQuestionTags: 'TutorQuestionTags',

  // 残值评估
  ValuationHome: 'ValuationHome',
  ValuationResult: 'ValuationResult',
  ValuationReport: 'ValuationReport',
  ValuationBatteryInput: 'ValuationBatteryInput',
  ValuationBatteryResult: 'ValuationBatteryResult',
  ValuationHistory: 'ValuationHistory',

  // 残值评估独立登录 / 注册
  ValuationLogin: 'ValuationLogin',
  ValuationRegister: 'ValuationRegister',
  ValuationForgotPassword: 'ValuationForgotPassword',

  // AI 助手
  AIAssistant: 'AIAssistant',
  AIAssistantFeature: 'AIAssistantFeature',

  // 管理员后台
  AdminDashboard: 'AdminDashboard',
  HrwaiUserManage: 'HrwaiUserManage',
  ProfileReview: 'ProfileReview',
  ForumManage: 'ForumManage',
  ContributionManage: 'ContributionManage',
  CourseCatalog: 'CourseCatalog',
  CredentialManage: 'CredentialManage',
  PositionManage: 'PositionManage',
  QuestionReview: 'QuestionReview',
  Statistics: 'Statistics',
  AdminFaqManage: 'AdminFaqManage',
  AuditLogs: 'AuditLogs',
  AdminInspection: 'AdminInspection',
  ContentGenerate: 'ContentGenerate',
  AdminFeaturedContentList: 'AdminFeaturedContentList',
  AdminFeaturedContentEdit: 'AdminFeaturedContentEdit',
  TutorManage: 'TutorManage',
  RecruiterManage: 'RecruiterManage',
  ValuationConfigManage: 'ValuationConfigManage',
  AISettings: 'AISettings',

  // 企业招聘端
  RecruitDashboard: 'RecruitDashboard',
  RecruitResumes: 'RecruitResumes',
  RecruitResumeDetail: 'RecruitResumeDetail',
  RecruitRequests: 'RecruitRequests',
  RecruitJobManage: 'RecruitJobManage',
  RecruitApplicationList: 'RecruitApplicationList'
} as const

/** 全部合法路由名的常量值 union（ADR-0027 C6 类型收紧用）。 */
export type RouteName = (typeof routeNames)[keyof typeof routeNames]