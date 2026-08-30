# CONTEXT.md — 领域词汇表（Domain Glossary）

本文件为架构评审、domain-modeling 与 AI 导航提供共享领域语言。架构术语见 codebase-design vocabulary（module / interface / seam / adapter / leverage / locality）。

## 角色

- **学员（hrwai_user）**：统一账号角色，学员端 / 残值评估 / AI 助手共用一张用户表（hrwai_users）与一套 JWT。
- **讲师（tutor）**：独立账号表（tutor），管理章节内容、题库、阅卷；不建课（课程创建/编辑仅管理员，见 ADR-0006 后的领域约定）。
- **管理员（admin）**：独立账号表（admin），管理学员/讲师/课程/题库/残值配置/AI 配置。

## 账号与认证

- **统一账号**：hrwai_users 表 + 统一 JWT（角色 hrwai_user）；支持用户名或手机号登录。
- **验证码（code）**：邮箱/手机号注册、登录、绑定、改账号、找回/修改密码的 6 位数字验证码。用途六态：register / login / bind / account_change / reset_password / change_password。错误上限 5 次，发送节流 60 秒，TTL 5 分钟。
- **验证码通道（channel）**：邮箱（SMTP，开发降级日志）与短信（腾讯云 SMS SendSms，开发降级日志）是同一验证码状态机两侧的 adapter。
- **会话（session）**：签发（issue）/ 校验（verify）/ 吊销（revoke）JWT 的生命周期。双令牌（ADR-0016）：access 2h（中间件仅收 access）+ refresh 7 天轮换；黑名单（`jwt:blacklist:`）只管理 refresh——刷新轮换即作废旧 refresh（防重放），登出撤销 refresh；access 生命周期短，不入黑名单、自然过期。
- **登录态 Cookie**：父域名 httpOnly Cookie（hrwai_token），子域名间共享登录；Bearer 头优先于 Cookie。生产已启用 HTTPS（PR #254），Cookie 通道恢复、仅携带 access（不自动续期，见 ADR-0016）；HTTP 时期的历史约束见 ADR-0003（已解决）。
- **微信小程序登录（wx-login）**：小程序端 uni.login 临时 code 换 openid 登录（POST /api/auth/wx-login，code2session）；openid 已绑定直接登录，未注册自动建号绑定（account 取 `wx_`+openid 前 12 位，昵称「微信学员」+openid 后 6 位，账号前缀冲突时追加后段或序号重试，唯一约束冲突与其它错误分类处理），复用统一登录骨架签发双令牌。凭证经 `WECHAT_MINI_PROGRAM_APP_ID`/`WECHAT_MINI_PROGRAM_APP_SECRET` 配置（GitHub Secrets 同名；与网页端扫码登录的开放平台凭证 `WECHAT_OPEN_PLATFORM_*` 严格区分，两套 AppID/AppSecret 不可混用）。契约见 `docs/docs/reference/微信小程序登录-文档说明.md`。
- **认证页（auth page）**：登录/注册/找回密码三页共用认证页外壳（AuthPageShell，白底极简 + 主次分离——密码为主入口，邮箱/手机/微信收纳为「或使用以下方式登录」图标按钮；tutor/admin 仅密码入口）。三页提交流程共用 useAuthFlow 状态机；redirect 回跳白名单（isSafeRedirect）与「路径前缀→身份」表单点（authRedirect），见 ADR-0014。
- **资料审核（profile review）**：昵称/头像修改走提交→审核（通过/驳回）流程，审核结果以站内信通知。

## 通知与审计

- **站内信（notification）**：站内信通知基础设施，当前唯一渠道；资料审核、论坛互动（帖子被回复/楼中楼被回复/举报处理结果/管理端删帖删回复，link 指向 `/training/forum/:id`、payload 携带 topic_id）等业务事件通过站内信模块发出。
- **审计日志（audit log）**：管理员/讲师写操作由中间件统一记录，落库留痕（合规用途，与系统运行日志区分）。
- **系统运行日志（app log）**：zap 统一日志栈（`internal/logger`），排查用——级别过滤、敏感字段脱敏、访问日志（request_id/user_id/role）、生产文件轮转持久化（`/data/logs`）。与「审计日志」的边界：前者是运行期诊断输出（console/文件），后者是业务写操作的持久化记录（DB 表），两者互不替代。

## 培训领域

- **目标证件（target credential）**：学员报考的外部持证目标（`credential`，`code` 唯一），与"证书模板（培训合格证书）"严格区分。两类：特种作业上岗证（`special_operation`：叉车司机N1/低压电工/焊工等）与职业技能等级（`skill_level`：工程机械维修工·叉车维修方向 L5-L1，每级为独立证件）；每证件拥有独立的课程库与题库（`course.credential_id` / `question.credential_id` 单归属，V1 1:N，预留 M:N 扩展）。
  _Avoid_: 证书、证件（泛称）、考证目标
- **当前证件（current credential）**：学员在 `hrwai_users.current_credential_id` 上的单选上下文，侧栏顶部 `CredentialSwitcher` 展示与切换；切换即全局过滤器（课程/题库/练习/模考/错题/搜索/收藏的 course/question 分区按当前证件过滤，论坛/AI 不过滤）。
- **预筛选（prescreening）**：首次注册/登录时 `current_credential_id IS NULL` 的强制拦截流，需在 onboarding 选定目标证件后方可进入 training 工作区；存量用户下一跳同样拦截一次。
  _Avoid_: 首选证件、初始化选择
- **占位证件（placeholder credential）**：已建档但课程/题库为 0 的目标证件，可被选为当前证件，视图呈空状态"内容建设中"。
- **课程目录（course catalog）**：目标证件内的 `专业方向 → 课程等级 → 课程` 三层组织视图（虚拟树，实时由 credential/specialty/course_level/course 计算，无物理 catalog 表）。未挂方向/等级的课程不出现在学员端目录与列表（口径统一）；课程必归属一个目标证件（`credential_id` 必填）。
- **专业方向（specialty）**：课程目录二级维度（证件内），全局共享（操作/维修/安全/电池等），管理员维护。
- **课程等级（course level）**：课程目录三级维度，**全局共享**（不归属方向，入门/进阶/专项/认证），任意方向的课程可挂任意等级。
- **课程（course）/ 章节（chapter）**：PPT/视频/图文混排内容；PPT 经 LibreOffice sidecar 转 WebP。课程必挂目标证件 + 专业方向 + 课程等级（创建/编辑必填），可关联证书模板与前置课程。
- **收藏（favorite）**：多态收藏（target_type+target_id：course/chapter/question/featured/topic；user+type+id 唯一幂等），列表实时回填目标快照、目标删除即条目消失，见 ADR-0018；其中 course/question 分区按当前证件过滤。
- **全局搜索（search）**：course/question/content/topic 四类 LOWER LIKE 聚合（type 缺省各分区 top5），可见性与业务口径一致（挂载不变式/published/已发布），见 ADR-0018；course/question 分区按当前证件过滤。
- **学习资料（material）**：已发布课程下章节附件（chapter_file）的聚合视图，不建独立资料库；file_url 为静态直链，见 ADR-0018。
- **学习位置（learning position）**：学员在某课程的最后学习状态——最后章节（last_chapter_id）、章节播放位置（video_position，秒）、最后学习时间戳（last_studied_at），挂在 study_record 双轨记录上（课程级承载 last_*、章节级承载位置）；章节完成以 progress≥100 为单一事实源（时长自动完成与显式 completed 收敛于此），见 ADR-0017。
- **证书模板（certificate template）**：课程可选关联的培训合格证书（结业证），含有效期（天）；课程挂靠后学员完成学习可获证。与"目标证件"语义严格区分。
- **前置课程（course prerequisite）**：课程间的依赖关系（A 完成才能学 B），防自指防成环；编辑回填 prerequisite_course_ids 避免误清空。
- **模拟考试（mock exam）**：自动判分 + AI 评分的自主模拟测验；题目类型满分规则由判分规则表定义（识图 4 分、简答 10 分）。定级考试（考试中心）已下线，模拟考试为唯一考试形态；按当前证件的题库抽题。
- **练习（practice）/ 错题本（wrong question book）**：顺序/随机/专项/标签练习；错题按题收录。刷题解析包含结果卡（正确/用时/正确率/易错项）、AI 解析（按需生成并缓存，未配置时降级静态解析）、评论、考点（题库标签）与笔记（每人每题一条私有备忘）五模块，练习与错题重做共用同一提交管线与装配；重做结果同口径落 question_practice_record（正确率/易错项统计含重做），错题重做为单题即时形态、无会话生命周期；均按当前证件的题库过滤。
- **答题会话（answering session）**：学员在一次练习/模拟考试/错题重做中逐题作答的状态与推进节奏（选项选择、对/错模板、倒计时、自动交卷、断点续传）；练习/模拟考试/错题重做共享同一交互形态。会话 module 为守卫（本人+进行中）、题目顺序重建、答案三态初始化（null/[]/absent）的唯一实现（ADR-0010）。
- **判分（grading）**：客观题判题唯一实现（gradeQuestion，含多选部分给分）；简答题及格线 = 满分 × 0.6。分值表两行单点定义：practice（练习/错题重做共用，客观题满分来源；原「level_exam」行已正名——定级考试下线后该行作为练习满分事实源存活）与 mock_exam（模拟考试）；简答题满分取题目自定义分，缺省 10。
- **题库标签（question tag）**：题目分类维度（法规/结构/液压/电气/制动/故障诊断/应急等），创建需唯一编码；题目可多标签，标签练习按标签抽题。
- **标签练习（tag practice）**：按题库标签抽题的练习模式（原「章节练习」已退役并入）。
- **AI 助手**：大模型流式对话（DeepSeek 默认，可配置 OpenAI 兼容模型）。归属 training 子域名（学员工作区功能），由主域名迁入。
- **论坛（forum）**：含类别 `category`（`discussion` | `question`）与采纳状态（`accepted_reply_id`/`solved_at`，问答帖仅楼主可采纳一条回答，被采纳回复恒置顶）的图文分离论坛。**图文分离**——主题与回复可携带 `images` 图片 URL 数组（JSONB），正文保持纯文本，不做 markdown 渲染。坐标与意图双维度：`discussion`+NULL=综合讨论区、`discussion`+N=章节讨论区（均为现状）、`question`+NULL=全局问答（新增）、`question`+N=非法（`CHECK (category <> 'question' OR chapter_id IS NULL)` 兜底 + service 400）。**判类别看 `category`，判区域看 `chapter_id`，两者不可互相替代**——`scope=general` 的定义是 `chapter_id IS NULL`，而问答帖的 `chapter_id` 同样为 NULL，故列表查询必须让 `category` 与 `scope` 共存在同一条 WHERE 里，否则问答帖会整片灌进讨论 Tab（管理端综合区同理）。互动（ADR-0018）：主题点赞（forum_topic_like，幂等，计数经 `likes_count` 反范式列维护，热度排序走索引）、举报（forum_report，待处理/已处理二态，管理端处置沿用删帖/删回复流）、我的帖子/我的回复；问答筛选 `solved`（all/solved/unsolved，仅对 question 有意义）与列表 `reward_issued` 标记。问答帖一律 `chapter_id=NULL`，提问不选章节，章节页不产生问答。
- **综合讨论区（general forum）**：**非章节讨论帖**（`category='discussion' AND chapter_id IS NULL`）。注意与旧定义区分：旧定义为"所有非章节帖"（`chapter_id IS NULL`），本次收窄后问答帖虽 `chapter_id` 同为 NULL 但**不属于**综合讨论区。
- **论坛图片（forum image）**：先经 `POST /api/forum/upload-image` 上传到 `images/forum/` 子目录拿 URL，随发帖/回复提交；删除主题/回复时图片存储一并清理；上传后未发帖的**悬空图片**由进程内定时任务（每 6 小时，通用守护 runner 托管，panic 恢复 + jitter 错峰 + 注入式 ticker + context 取消贯穿存储）扫描差集、回收超过 24h 未被引用的文件。
- **每日打卡（check-in）**：学员按 `Asia/Shanghai` 自然日签到，独立 `CheckInService` 模块承载（与论坛帖子/回复解耦，共享 `ForumAuthor` 展示名 seam，路由 `/api/forum/check-in/*` 契约不变）。能力四件套：签到（幂等 `forum_checkin` PK(user_id, check_date)）、日历（按年月查 `check_date BETWEEN`，`Asia/Shanghai` 口径）、连击（streak，从今日/昨日连续往前计数）、排行榜（累计总榜 `total DESC, last_date ASC`，`streak/todayChecked` 经批量聚合回填，`Me` 名次合并查询）。日历与连击跨时区一致性由 `Asia/Shanghai` 统一承载。
- **问答采纳奖励（Q&A accept reward）**：问答帖采纳触发的积分流水直记（非任务领取制）：答主被采纳 +40（`accepted_bonus`）、楼主采纳动作 +5（`accept_action`），即时入账、站内信到达；每帖只发一次分（幂等键 `ref_type='forum_topic', ref_id=topic_id, reason='accepted_bonus'`，取消/更换/并发均只发一次），自答零分，防刷乙档（答主日 3 次、楼主日 5 次、同一楼主↔答主配对 1-3 次满分、4-5 次减半、6 次起零分），删帖不回滚、违规回收走 `rollback` 对冲（封底 0）。
- **简历卡（resume card / job card）**：1:1 挂在学员账号上的常驻实体（`job_cards`，`user_id` 主键，`ON DELETE CASCADE`），资料直接长在卡上，无发布/快照/有效期；含身份与联系（`real_name`/`contact_phone`/`wechat`/`region`，后者与登录手机号分离）、求职意向（`expected_specialty_id`→`specialty`、`expected_regions` JSONB、`salary_min/max`/`salary_negotiable`、`available_in`/`job_nature`）、资历（`experience_years`/`self_intro`≤1000、`resume_experiences` JSONB、`resume_certifications` JSONB 含 `credential_id`/`cert_no`/`expire_date`/`image_urls`）、附件（`resume_file_url` 单 PDF ≤50MB、`photos` ≤6）、状态（`visibility` 默认 `hidden`，公开后招聘端可见）。改一次即最新，无缓存快照。
- **企业招聘者（recruiter）**：第四角色（`recruiter_users` 独立表，不进 `hrwai_users`；`status` 禁用位），邀约制（仅管理员创建，企业信息必填：`company_name`/`credit_code`/`business_scope`/`contact_name`/`contact_phone`/`contact_email`），独立子域 `recruit.` + 独立布局 `RecruitLayout` + `role=recruiter` 鉴权，登录态 `recruiter_token` host-only（与学员侧 `hrwai_token` 父域共享隔离，防静默恢复串角色），会话仍归 `security.Session` 单例。三层漏斗：L1 未登录不可见（无公开列表/详情/SEO）、L2 脱敏卡（岗位/地区/薪资/年限/经历/自评/持证标签可见，姓名打码，无手机/微信/精确现居地/PDF/证书原图）、L3 交换后明文。
- **联系方式交换（contact exchange）**：L3 闭环，招聘方带附言（1-200 字）发起 `contact_requests`（`pending`→`approved`/`rejected`/`expired`/`revoked`，`pending` 14 天过期、`rejected`/`revoked` 后 30 天冷却、同一企业对同一学员 `pending` 唯一、单企业日限 20），学员站内信收到申请（企业名/联系人/附言，不含企业电话，`link=/training/resume`）后可同意/拒绝/撤回（永久授权、撤回实时生效、明文不缓存现查 `approved` 状态），招聘方同意后邮件通知并可在「我的申请」列表查看状态，学员注销时申请与授权一并失效。
- **简历查看留痕（resume view trail）**：招聘方每次查看含个人信息的脱敏卡即写入 `recruit_resume_views`（`recruiter_id`/`resume_user_id`/`viewed_at`，粒度同一招聘方对同一学员每日一次，健康检查与自身访问不写），学员侧仅见聚合数「近 7 天 N 家企业查看过你的简历」（按企业去重，`WHERE resume_user_id=? AND viewed_at>=now-7d` 走 `(resume_user_id, viewed_at)` 索引，不返回企业名），招聘方无法反查。

## 残值评估（valuation）

- **残值评估**：输入品牌、车型、系列、吨位、配置、出厂年份、工时、车况、区域，输出残值估算、置信区间与多维系数雷达图。结果携带**未来价值曲线锚点（decay_anchor）**——后端唯一实现衰减公式，前端图表只做 d^n 乘法渲染（ADR-0012）。
- **评估记录（evaluation record）**：不可变事实——K 系数、残值、置信区间、建议与 λ 值均在评估时点锁定（ADR-0004）；管理员修改系数配置不改变历史评估结论。
- **五维系数**：出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km；公式 `残值 = 原价 × Kt_adj × Kc × Km`，`Kt_adj = Kt^(Kh/Kb)`。
- **系数表（dictionary）**：品牌/车型/车况/规格/级联等字典数据，管理员 CRUD，学生端只读；读路径走缓存（`dict:*`）。
- **建议（suggestions）**：评估结果生成的文本建议（车况/证件/原厂漆/品牌强度/市场/残值率 7 个 section），百分比从 coefficient_configs 动态读取；建议文字在**评估时点持久化锁定**，不随系数配置漂移（ADR-0004）。
- **报告（report）**：评估/电池 RUL 的 PDF 报告；生成、下载与再生成收敛为单一报告协调器（加载/生成/回写三 adapter 槽位），同 ID 并发下载只产生一份 PDF。
- **电池 RUL**：锂电池剩余寿命预测（SOH、置信区间、EOL 阈值 60%、LFP/NCM 区分）。

## 门户与内容

- **官网门户（portal）**：独立 Nuxt 4 仓库承载的 www 子域名站点，SSR 混合渲染（首页预渲染、内容页 SWR/ISR），面向访客与搜索引擎爬虫；只读消费后端公开 API，与培训/评估等 Vue SPA 工作区解耦。
- **官网首页（homepage）**：门户 `/` 页面，固定区块结构（Hero/公司介绍/创始人/核心服务/合作模式/服务保障/内容精选轮播/CTA），构建时预渲染。
- **精选内容（featured content）**：官网内容板块，管理端（Vue SPA manage 子域名）维护，门户只读消费公开 API；分类四态：公司动态/行业新闻/产品资讯/政策法规。
- **精选内容归档页（news archive）**：`/news`（全部）+ `/news/[category]`（四类）分页列表页，是每条已发布内容的可发现入口（首页轮播只展示前 6 条）；支持排序：最新资讯（按 `published_at`）/ 热点资讯（按 `view_count`）。
- **阅读量（view count）**：文章被真实浏览器访问的次数。门户侧详情接口带 `no_view=1` 时不计入（SSR/爬虫路径），由 hydration 后客户端计数端点累加——与「详情请求次数」语义区分；论坛侧当前为详情请求即计数（`GET /api/forum/topics/:id` 每次 +1，未做防刷），与门户侧口径差异已在 spec #279 中追溯，未来若将浏览量提为热度主序需评估防刷复用。
