# CONTEXT.md — 领域词汇表（Domain Glossary）

本文件为架构评审、domain-modeling 与 AI 导航提供共享领域语言。架构术语见 codebase-design vocabulary（module / interface / seam / adapter / leverage / locality）。

## 角色

- **学员（hrwai_user）**：统一账号角色，学员端 / 残值评估 / AI 助手共用一张用户表（hrwai_users）与一套 JWT。
- **讲师（tutor）**：独立账号表（tutor），管理章节内容、题库、阅卷；不建课（课程创建/编辑仅管理员，见 ADR-0006 后的领域约定）。
- **管理员（admin）**：独立账号表（admin），管理学员/讲师/课程/题库/残值配置/AI 配置。

## 账号与认证

- **统一账号**：hrwai_users 表 + 统一 JWT（角色 hrwai_user）；支持用户名或手机号登录。
- **验证码（code）**：邮箱/手机号注册、登录、绑定的 6 位数字验证码。用途三态：register / login / bind。错误上限 5 次，发送节流 60 秒，TTL 5 分钟。
- **验证码通道（channel）**：邮箱（SMTP，开发降级日志）与短信（生产未接入，开发降级日志）是同一验证码状态机两侧的 adapter。
- **会话（session）**：签发（issue）/ 校验（verify）/ 吊销（revoke）JWT 的生命周期；登出即把 token hash 写入黑名单（`jwt:blacklist:`），TTL = token 剩余有效期。
- **登录态 Cookie**：父域名 httpOnly Cookie（hrwai_token），子域名间共享登录；Bearer 头优先于 Cookie。注：生产为 HTTP（443 不可用）时 Secure cookie 被浏览器拒绝，Cookie 通道失效、登录态跨子域不共享，见 ADR-0003。
- **资料审核（profile review）**：昵称/头像修改走提交→审核（通过/驳回）流程，审核结果以站内信通知。

## 通知与审计

- **站内信（notification）**：站内信通知基础设施，当前唯一渠道；资料审核等业务事件通过站内信模块发出。
- **审计日志（audit log）**：管理员/讲师写操作由中间件统一记录。

## 培训领域

- **课程目录（course catalog）**：专业方向 → 课程等级 → 课程 的三层组织视图（虚拟树，实时由 specialty/course_level/course 计算，无物理 catalog 表）。未挂方向/等级的课程不出现在学员端目录与列表（口径统一）。
- **专业方向（specialty）**：课程目录一级维度，全局共享（操作/维修/安全/电池等），管理员维护。
- **课程等级（course level）**：课程目录二级维度，**全局共享**（不归属方向，入门/进阶/专项/认证），任意方向的课程可挂任意等级。
- **课程（course）/ 章节（chapter）**：PPT/视频/图文混排内容；PPT 经 LibreOffice sidecar 转 WebP。课程挂专业方向 + 课程等级（创建/编辑必填），可关联证书模板与前置课程。
- **证书模板（certificate template）**：课程可选关联的培训合格证书，含有效期（天）；课程挂靠后学员完成学习可获证。
- **前置课程（course prerequisite）**：课程间的依赖关系（A 完成才能学 B），防自指防成环；编辑回填 prerequisite_course_ids 避免误清空。
- **考试（exam）/ 模拟考试（mock exam）/ 定级考试（原等级考试，level exam）**：自动判分 + AI 评分；题目类型满分规则由判分规则表定义。定级考试为考试中心功能名，与目录维度「课程等级」无关。
- **练习（practice）/ 错题本（wrong question book）**：顺序/随机/专项/标签练习；错题按题收录。
- **题库标签（question tag）**：题目分类维度（法规/结构/液压/电气/制动/故障诊断/应急等），创建需唯一编码；题目可多标签，标签练习按标签抽题。
- **标签练习（tag practice）**：按题库标签抽题的练习模式（原「章节练习」已退役并入）。
- **AI 助手**：大模型流式对话（DeepSeek 默认，可配置 OpenAI 兼容模型）。
- **论坛（forum）**：综合讨论区 + 章节讨论区；发帖/回复（可回复别人的回复）。**图文分离**——主题与回复可携带 `images` 图片 URL 数组（JSONB），正文保持纯文本，不做 markdown 渲染。
- **论坛图片（forum image）**：先经 `POST /api/forum/upload-image` 上传到 `images/forum/` 子目录拿 URL，随发帖/回复提交；删除主题/回复时图片存储一并清理；上传后未发帖的**悬空图片**由进程内定时任务（每 6 小时）扫描差集、回收超过 24h 未被引用的文件。

## 残值评估（valuation）

- **残值评估**：输入品牌、车型、系列、吨位、配置、出厂年份、工时、车况、区域，输出残值估算、置信区间与多维系数雷达图。
- **评估记录（evaluation record）**：不可变事实——K 系数、残值、置信区间、建议与 λ 值均在评估时点锁定（ADR-0004）；管理员修改系数配置不改变历史评估结论。
- **五维系数**：出厂时间 Kt、使用强度 Kh、品牌价值 Kb、车况 Kc、市场需求 Km；公式 `残值 = 原价 × Kt_adj × Kc × Km`，`Kt_adj = Kt^(Kh/Kb)`。
- **系数表（dictionary）**：品牌/车型/车况/规格/级联等字典数据，管理员 CRUD，学生端只读；读路径走缓存（`dict:*`）。
- **建议（suggestions）**：评估结果生成的文本建议（车况/证件/原厂漆/品牌强度/市场/残值率 7 个 section），百分比从 coefficient_configs 动态读取；建议文字在**评估时点持久化锁定**，不随系数配置漂移（ADR-0004）。
- **报告（report）**：评估/电池 RUL 的 PDF 报告；生成、下载与再生成收敛为单一报告协调器（加载/生成/回写三 adapter 槽位），同 ID 并发下载只产生一份 PDF。
- **电池 RUL**：锂电池剩余寿命预测（SOH、置信区间、EOL 阈值 60%、LFP/NCM 区分）。
