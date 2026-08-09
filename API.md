# API 接口清单

本文档为叉车维修培训系统 + 残值评估子系统 + AI 助手的 HTTP 接口总清单（路径 / 方法 / 鉴权 / 说明）。

- 基础路径：`/api`
- 响应统一包裹：`{ "code": 200, "message": "...", "data": ... }`（`/api/valuation/*` 有独立响应格式）
- 鉴权方式：`Authorization: Bearer <JWT>`
- 角色：`admin`（管理员）/ `tutor`（讲师）/ `hrwai_user`（学员/普通用户）
- 上传限制：`/api/admin/featured-content/upload-image` 等图片上传接口，单文件大小受限，见各 handler 校验
- 限流：基于客户端 IP 的 token bucket，健康检查放行

---

## 1. 系统与健康检查

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api` | 无 | 服务信息（版本号） |
| GET | `/api/health` | 无 | 健康检查（探测 Redis，异常返回 503 degraded） |
| GET | `/api/health/live` | 无 | 存活探针（liveness，仅进程存活） |
| GET/HEAD | `/static/*filepath` | 无 | 静态资源与上传文件（uploads 前缀走上传目录） |

## 2. 账号认证 `/api/auth`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/login` | 无 | 账号密码登录 |
| POST | `/api/auth/register` | 无 | 注册 |
| POST | `/api/auth/admin-login` | 无 | 管理员登录 |
| POST | `/api/auth/tutor-login` | 无 | 讲师登录 |
| POST | `/api/auth/logout` | JWT | 登出 |
| GET | `/api/auth/me` | JWT | 当前用户信息 |
| PUT | `/api/auth/profile` | JWT | 更新个人资料（昵称/头像） |
| POST | `/api/auth/avatar` | JWT | 上传头像 |

### 邮箱验证码 `/api/auth/email`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/email/send-code` | 无 | 发送邮箱验证码 |
| POST | `/api/auth/email/register` | 无 | 邮箱验证码注册 |
| POST | `/api/auth/email/login` | 无 | 邮箱验证码登录 |

### 手机号 `/api/auth/phone`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/phone/send-code` | 无 | 发送手机验证码 |
| POST | `/api/auth/phone/register` | 无 | 手机验证码注册 |
| POST | `/api/auth/phone/login` | 无 | 手机验证码登录 |

### 微信 `/api/auth/wechat`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/wechat/qrcode` | 无 | 获取登录二维码（框架占位） |
| POST | `/api/auth/wechat/login` | 无 | 微信扫码登录（框架占位） |

### 账号绑定修改 `/api/auth/profile`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/profile/send-code` | JWT | 发送绑定验证码 |
| POST | `/api/auth/profile/email` | JWT | 绑定/修改邮箱 |
| POST | `/api/auth/profile/phone` | JWT | 绑定/修改手机号 |
| POST | `/api/auth/profile/password` | JWT | 修改密码 |

## 3. 课程学习 `/api`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/courses` | 无 | 课程列表（公开，支持 `specialty_id` / `level_id` 过滤；未挂方向/等级的课程不展示） |
| GET | `/api/catalog/tree` | 无 | 课程目录树：专业方向 → 课程等级 → 课程（含章节数） |
| GET | `/api/specialties` | 无 | 专业方向列表（仅启用项） |
| GET | `/api/levels` | 无 | 课程等级列表（仅启用项） |
| GET | `/api/tags` | 无 | 题库标签列表（仅启用项） |
| GET | `/api/chapter/:chapter_id/slides` | 无 | 章节课件列表（公开） |
| GET | `/api/course/:course_id` | JWT | 课程详情（含等级/学时/前置课程/证书模板） |
| GET | `/api/course/:course_id/chapter/:chapter_id` | JWT | 章节详情 |
| POST | `/api/chapter/:chapter_id/slides/regenerate` | JWT | 重新生成章节课件 |
| POST | `/api/course/:course_id/progress` | JWT | 记录学习进度 |

### 学员端 `/api/student`（role=hrwai_user）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/student/profile` | JWT+hrwai_user | 学员个人资料 |
| GET | `/api/student/records` | JWT+hrwai_user | 学习/考试记录 |
| GET | `/api/student/study-stats` | JWT+hrwai_user | 学习统计 |

## 4. 题库练习 `/api/question-bank`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/question-bank/questions` | JWT | 题库列表（支持 `tag_id` 标签过滤，题目附 `tags`） |
| POST | `/api/question-bank/questions` | JWT+tutor/admin | 新增题目 |
| POST | `/api/question-bank/questions/batch-publish` | JWT+admin | 批量发布 |
| POST | `/api/question-bank/questions/batch-reject` | JWT+admin | 批量驳回 |
| POST | `/api/question-bank/questions/batch-import` | JWT+tutor/admin | 批量导入 |
| GET | `/api/question-bank/questions/:question_id` | JWT | 题目详情 |
| PUT | `/api/question-bank/questions/:question_id` | JWT+tutor/admin | 更新题目 |
| DELETE | `/api/question-bank/questions/:question_id` | JWT+tutor/admin | 删除题目 |
| POST | `/api/question-bank/questions/:question_id/publish` | JWT+admin | 发布题目 |
| POST | `/api/question-bank/questions/:question_id/reject` | JWT+admin | 驳回题目 |
| GET | `/api/question-bank/stats` | JWT | 题库统计 |
| POST | `/api/question-bank/upload-image` | JWT+tutor/admin | 上传题图 |

## 5. 自由刷题模式 `/api/practice-mode`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/practice-mode/free` | 自由刷题 |
| GET | `/api/practice-mode/tag` | 标签练习开始/续练（`tag_id` 必填，`count` 控制题量，0=全部；返回题目+进度，mode=`tag:<tagID>`） |
| GET | `/api/practice-mode/sequential` | 顺序练习 |
| GET | `/api/practice-mode/sequential-progress` | 顺序练习进度 |
| POST | `/api/practice-mode/progress` | 保存练习进度 |
| GET | `/api/practice-mode/progress` | 查询练习进度 |
| POST | `/api/practice-mode/submit` | 提交练习 |
| GET | `/api/practice-mode/stats` | 练习统计 |
| GET | `/api/practice-mode/history` | 练习历史 |

## 6. 定级考试 `/api/level-exam`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/level-exam/sessions` | JWT | 场次列表 |
| POST | `/api/level-exam/sessions` | JWT+admin | 创建场次 |
| PUT | `/api/level-exam/sessions/:session_id/status` | JWT+admin | 更新场次状态 |
| GET | `/api/level-exam/sessions/:session_id` | JWT | 场次详情 |
| PUT | `/api/level-exam/sessions/:session_id` | JWT+admin | 更新场次 |
| DELETE | `/api/level-exam/sessions/:session_id` | JWT+admin | 删除场次 |
| GET | `/api/level-exam/available` | JWT+hrwai_user | 可参加的考试 |
| GET | `/api/level-exam/history` | JWT+hrwai_user | 考试历史 |
| POST | `/api/level-exam/sessions/:session_id/enter` | JWT+hrwai_user | 进入考试 |
| POST | `/api/level-exam/participants/:participant_id/save` | JWT+hrwai_user | 保存作答 |
| POST | `/api/level-exam/participants/:participant_id/submit` | JWT+hrwai_user | 交卷 |
| GET | `/api/level-exam/participants/:participant_id/result` | JWT+hrwai_user | 考试成绩 |

## 7. 模拟考试 `/api/mock-exam`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/mock-exam/start` | 开始模拟考 |
| POST | `/api/mock-exam/:mock_exam_id/save` | 保存进度 |
| GET | `/api/mock-exam/:mock_exam_id/resume` | 恢复考试 |
| POST | `/api/mock-exam/:mock_exam_id/submit` | 交卷 |
| GET | `/api/mock-exam/:mock_exam_id/result` | 考试结果 |
| GET | `/api/mock-exam/history` | 考试历史 |

## 8. 阅卷评分 `/api/grading`（role=tutor/admin）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/grading/participants` | 阅卷列表 |
| GET | `/api/grading/participants/:participant_id` | 阅卷详情 |
| POST | `/api/grading/participants/:participant_id/confirm-objective` | 确认客观题成绩 |
| POST | `/api/grading/:answer_id/grade` | 人工评分 |
| POST | `/api/grading/:answer_id/regrade` | 重新评分 |
| POST | `/api/grading/:answer_id/confirm-ai` | 确认 AI 评分 |
| POST | `/api/grading/:answer_id/ai-grade` | 触发 AI 评分 |
| GET | `/api/grading/stats` | 阅卷统计 |

## 9. 讲师端 `/api/tutor`（role=tutor）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tutor/courses` | 讲师课程列表 |
| GET | `/api/tutor/grading-stats` | 讲师阅卷统计 |
| GET | `/api/tutor/course/:course_id/chapters` | 课程章节列表 |
| GET | `/api/tutor/chapter/:chapter_id` | 章节详情 |
| POST | `/api/tutor/chapter/:chapter_id/upload` | 上传章节文件（课件/视频等） |
| POST | `/api/tutor/upload-image` | 上传图文 Markdown 图片（Vditor 格式，返回 succMap；`chapter_id` 可选，按章节分目录存储） |
| PUT | `/api/tutor/chapter/:chapter_id` | 更新章节 |
| DELETE | `/api/tutor/file/:file_id` | 删除文件 |
| POST | `/api/tutor/files/batch-delete` | 批量删除文件 |

## 10. 错题本 `/api/wrong-questions`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/wrong-questions` | 错题列表 |
| POST | `/api/wrong-questions/:question_id/redo` | 重做错题 |
| POST | `/api/wrong-questions/:question_id/remove` | 移除错题 |
| GET | `/api/wrong-questions/stats` | 错题统计 |
| GET | `/api/wrong-questions/export` | 导出错题 |

## 11. 管理端 `/api/admin`（role=admin）

### 课程管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/courses` | 课程列表 |
| POST | `/api/admin/course` | 创建课程 |
| GET | `/api/admin/course/:course_id` | 课程详情 |
| PUT | `/api/admin/course/:course_id` | 更新课程 |
| DELETE | `/api/admin/course/:course_id` | 删除课程 |
| POST | `/api/admin/course/:course_id/chapter` | 新增章节 |
| PUT | `/api/admin/chapter/:chapter_id` | 更新章节 |
| DELETE | `/api/admin/chapter/:chapter_id` | 删除章节 |
| POST | `/api/admin/course/generate-content` | 异步生成章节内容（async_task） |
| GET | `/api/admin/course/generate-content/:task_id` | 查询生成任务状态 |

### 培训目录管理（专业方向 / 等级 / 证书模板 / 题库标签）

课程表单新增字段：`specialty_id`、`level_id`（创建/编辑必填）、`theory_hours`（理论学时）、`practice_hours`（实操学时）、`certificate_template_id`、`prerequisite_course_ids`（前置课程 ID 数组，编辑回填避免清空）、`sort_order`（课程排序，所属方向+等级层级内生效）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/catalog/tree` | 管理端目录树：专业方向 → 等级 → 课程 → 章节（含停用项） |
| GET | `/api/admin/courses` | 课程列表（支持 `specialty_id` / `level_id` 过滤，按 `sort_order` 排序） |
| GET | `/api/admin/specialties` | 专业方向列表 |
| POST | `/api/admin/specialty` | 创建专业方向 |
| PUT | `/api/admin/specialty/:specialty_id` | 更新专业方向 |
| DELETE | `/api/admin/specialty/:specialty_id` | 删除专业方向 |
| GET | `/api/admin/levels` | 课程等级列表 |
| POST | `/api/admin/level` | 创建课程等级 |
| PUT | `/api/admin/level/:level_id` | 更新课程等级 |
| DELETE | `/api/admin/level/:level_id` | 删除课程等级 |
| GET | `/api/admin/certificate-templates` | 证书模板列表 |
| POST | `/api/admin/certificate-template` | 创建证书模板（`validity_days` 有效期，天） |
| PUT | `/api/admin/certificate-template/:id` | 更新证书模板 |
| DELETE | `/api/admin/certificate-template/:id` | 删除证书模板 |
| GET | `/api/admin/question-tags` | 题库标签列表（含 `question_count` 题目数；学员端 `/api/tags` 统计已发布题目数） |
| POST | `/api/admin/question-tag` | 创建题库标签 |
| PUT | `/api/admin/question-tag/:id` | 更新题库标签 |
| DELETE | `/api/admin/question-tag/:id` | 删除题库标签 |
| GET | `/api/admin/question/:question_id/tags` | 查询题目标签 |
| PUT | `/api/admin/question/:question_id/tags` | 全量替换题目标签（`tag_ids`） |
| PUT | `/api/admin/specialty/:specialty_id/sort` | 交换专业方向排序（body `swap_with`） |
| PUT | `/api/admin/level/:level_id/sort` | 交换课程等级排序（body `swap_with`） |
| PUT | `/api/admin/question-tag/:id/sort` | 交换题库标签排序（body `swap_with`） |
| PUT | `/api/admin/course/:course_id/sort` | 交换课程排序（同一方向+等级组内，body `swap_with`） |

### 用户管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/hrwai-users` | 用户列表 |
| POST | `/api/admin/hrwai-users` | 创建用户 |
| PUT | `/api/admin/hrwai-users/:id` | 更新用户 |
| PUT | `/api/admin/hrwai-users/:id/password` | 重置密码 |
| PUT | `/api/admin/hrwai-users/:id/status` | 启用/禁用 |
| DELETE | `/api/admin/hrwai-users/:id` | 删除用户 |
| GET | `/api/admin/tutors` | 讲师列表 |
| POST | `/api/admin/tutor` | 创建讲师 |
| DELETE | `/api/admin/tutor/:tutor_id` | 删除讲师 |
| PUT | `/api/admin/tutor/:tutor_id/password` | 重置讲师密码 |
| PUT | `/api/admin/tutor/:tutor_id/status` | 启用/禁用讲师 |

### 统计与审核

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/statistics` | 运营统计 |
| GET | `/api/admin/audit-logs` | 操作审计日志 |
| GET | `/api/admin/export/students` | 导出学员名单 CSV |
| GET | `/api/admin/export/exam-records` | 导出成绩单 CSV |
| GET | `/api/admin/export/questions` | 导出题库 CSV |
| GET | `/api/admin/export/evaluations` | 导出评估记录 CSV |
| GET | `/api/admin/profile-reviews` | 资料审核列表 |
| POST | `/api/admin/profile-reviews/:id/approve` | 通过审核 |
| POST | `/api/admin/profile-reviews/:id/reject` | 拒绝审核 |

### AI 配置管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/ai-configs` | AI 配置列表（Key 脱敏） |
| POST | `/api/admin/ai-configs` | 新增 AI 配置 |
| PUT | `/api/admin/ai-configs/:id` | 更新配置 |
| DELETE | `/api/admin/ai-configs/:id` | 删除配置 |
| POST | `/api/admin/ai-configs/:id/test` | 测试配置连通性 |
| GET | `/api/admin/ai-feature-bindings` | 功能绑定列表 |
| PUT | `/api/admin/ai-feature-bindings/:feature_key` | 设置绑定 |
| DELETE | `/api/admin/ai-feature-bindings/:feature_key/configs/:config_id` | 解除绑定 |

### 内容精选（公司动态/行业新闻）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/featured-contents` | 管理端列表（含草稿） |
| GET | `/api/admin/featured-content/:id` | 管理端详情 |
| POST | `/api/admin/featured-content` | 新增 |
| PUT | `/api/admin/featured-content/:id` | 更新 |
| DELETE | `/api/admin/featured-content/:id` | 删除 |
| POST | `/api/admin/featured-content/:id/publish` | 发布/下线 |
| POST | `/api/admin/featured-content/upload-image` | 上传封面图 |

### 论坛管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/forum/topics` | 帖子列表 |
| GET | `/api/admin/forum/topics/:id` | 帖子详情 |
| DELETE | `/api/admin/forum/topics/:id` | 删除帖子 |
| DELETE | `/api/admin/forum/replies/:id` | 删除回复 |

## 12. 内容精选（公开）`/api`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/featured-contents` | 无 | 内容精选列表（仅已发布） |
| GET | `/api/featured-content/:id` | 无 | 内容详情（含相关资讯/上下一篇） |

## 13. AI 助手 `/api/ai-assistant`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/ai-assistant/models` | 无 | 可用模型列表 |
| POST | `/api/ai-assistant/chat` | 可选 JWT | 流式对话（登录可保存会话） |
| GET | `/api/ai-assistant/sessions` | JWT+hrwai_user | 会话列表 |
| POST | `/api/ai-assistant/sessions` | JWT+hrwai_user | 创建会话 |
| DELETE | `/api/ai-assistant/sessions/:id` | JWT+hrwai_user | 删除会话 |
| PATCH | `/api/ai-assistant/sessions/:id/title` | JWT+hrwai_user | 重命名会话 |
| GET | `/api/ai-assistant/sessions/:id/messages` | JWT+hrwai_user | 会话消息 |
| GET | `/api/ai-assistant/user-models` | JWT+hrwai_user | 用户自定义模型列表 |
| POST | `/api/ai-assistant/user-models` | JWT+hrwai_user | 保存自定义模型 |
| DELETE | `/api/ai-assistant/user-models/:id` | JWT+hrwai_user | 删除自定义模型 |

## 14. 论坛 `/api/forum`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/forum/upload-image` | 上传论坛图片（multipart `file`，返回 `{url}`；先传图后随发帖/回复提交） |
| GET | `/api/forum/topics` | 帖子列表 |
| POST | `/api/forum/topics` | 发帖（可选 `images: [url...]`，最多 9 张） |
| GET | `/api/forum/topics/:id` | 帖子详情 |
| POST | `/api/forum/topics/:id/replies` | 回复（可选 `images: [url...]`，最多 3 张） |
| DELETE | `/api/forum/topics/:id` | 删帖（主题与全部回复图片一并清理） |
| DELETE | `/api/forum/replies/:id` | 删除回复（含下级回复图片一并清理） |

> 图片说明：图文分离，正文保持纯文本，图片以 `images` 数组独立存储/展示；仅接受本站 `images/forum/` 前缀 URL；上传后未发帖的悬空图片由后端定时任务（每 6 小时）回收超过 24h 未被引用的文件。

## 15. 通知 `/api/notifications`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/notifications` | JWT | 通知列表 |
| GET | `/api/notifications/unread-count` | JWT | 未读数量 |
| POST | `/api/notifications/:id/read` | JWT | 标记已读 |
| POST | `/api/notifications/read-all` | JWT | 全部已读 |

---

## 16. 残值评估子模块 `/api/valuation`

> 独立连接池（pgx）+ 独立响应格式；鉴权分三档：公开 / 可选认证 / hrwai_user JWT / admin JWT。

### 公开（无需登录）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/valuation/evaluations/stats` | 评估统计 |
| GET | `/api/valuation/health` | 子模块健康检查 |
| POST | `/api/valuation/auth/login` | 估值模块登录（兼容主体系） |
| POST | `/api/valuation/auth/register` | 估值模块注册 |
| POST | `/api/valuation/evaluations/:id/report` | 生成评估 PDF 报告 |
| GET | `/api/valuation/evaluations/:id/report` | 下载评估 PDF 报告 |
| POST | `/api/valuation/battery/evaluations/:id/report` | 生成电池评估 PDF |
| GET | `/api/valuation/battery/evaluations/:id/report` | 下载电池评估 PDF |
| GET | `/api/valuation/dictionaries/brands` | 品牌字典 |
| GET | `/api/valuation/dictionaries/vehicle-types` | 车型字典 |
| GET | `/api/valuation/dictionaries/series` | 车系字典 |
| GET | `/api/valuation/dictionaries/tonnages` | 吨位字典 |
| GET | `/api/valuation/dictionaries/config-types` | 配置类型字典 |
| GET | `/api/valuation/dictionaries/mast-types` | 门架类型字典 |
| GET | `/api/valuation/dictionaries/mast-heights` | 门架高度字典 |
| GET | `/api/valuation/dictionaries/battery-types` | 电池类型字典 |
| GET | `/api/valuation/dictionaries/transmission-types` | 传动类型字典 |
| GET | `/api/valuation/dictionaries/engine-types` | 发动机类型字典 |
| GET | `/api/valuation/dictionaries/series-config-options` | 车系配置项 |
| GET | `/api/valuation/dictionaries/condition-ratings` | 车况等级字典 |
| GET | `/api/valuation/dictionaries/region-coefficients` | 地区系数字典 |
| GET | `/api/valuation/dictionaries/provinces` | 省份列表 |
| GET | `/api/valuation/dictionaries/cities` | 城市列表 |
| GET | `/api/valuation/dictionaries/coefficient-configs` | 系数配置列表 |
| GET | `/api/valuation/dictionaries/original-prices` | 原厂价格字典 |
| GET | `/api/valuation/dictionaries/earliest-factory-year` | 最早出厂年份 |
| GET | `/api/valuation/dictionaries/algorithm-parameters` | 算法参数 |

### 可选认证（登录则记录 user_id）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/valuation/evaluations` | 提交评估（未登录匿名提交） |

### HRWAI 账号鉴权（JWT + role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/valuation/evaluations` | 我的评估历史 |
| GET | `/api/valuation/evaluations/:id` | 评估详情 |
| POST | `/api/valuation/battery/evaluations` | 创建电池 RUL 评估 |
| GET | `/api/valuation/battery/evaluations` | 电池评估列表 |
| GET | `/api/valuation/battery/evaluations/:id` | 电池评估详情 |
| GET | `/api/valuation/auth/me` | 当前估值用户 |
| POST | `/api/valuation/auth/logout` | 估值登出 |

### 管理员 CRUD（JWT + role=admin）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/valuation/admin/brands` | 新增品牌 |
| PUT | `/api/valuation/admin/brands/:id` | 更新品牌 |
| DELETE | `/api/valuation/admin/brands/:id` | 删除品牌 |
| POST | `/api/valuation/admin/vehicle-types` | 新增车型 |
| PUT | `/api/valuation/admin/vehicle-types/:id` | 更新车型 |
| DELETE | `/api/valuation/admin/vehicle-types/:id` | 删除车型 |
| POST | `/api/valuation/admin/series` | 新增车系 |
| PUT | `/api/valuation/admin/series/:id` | 更新车系 |
| DELETE | `/api/valuation/admin/series/:id` | 删除车系 |
| POST | `/api/valuation/admin/tonnages` | 新增吨位 |
| DELETE | `/api/valuation/admin/tonnages/:id` | 删除吨位 |
| POST | `/api/valuation/admin/mast-types` | 新增门架类型 |
| DELETE | `/api/valuation/admin/mast-types/:id` | 删除门架类型 |
| POST | `/api/valuation/admin/mast-heights` | 新增门架高度 |
| DELETE | `/api/valuation/admin/mast-heights/:id` | 删除门架高度 |
| POST | `/api/valuation/admin/battery-types` | 新增电池类型 |
| DELETE | `/api/valuation/admin/battery-types/:id` | 删除电池类型 |
| POST | `/api/valuation/admin/transmission-types` | 新增传动类型 |
| DELETE | `/api/valuation/admin/transmission-types/:id` | 删除传动类型 |
| POST | `/api/valuation/admin/engine-types` | 新增发动机类型 |
| DELETE | `/api/valuation/admin/engine-types/:id` | 删除发动机类型 |
| POST | `/api/valuation/admin/condition-ratings` | 新增车况等级 |
| PUT | `/api/valuation/admin/condition-ratings/:id` | 更新车况等级 |
| DELETE | `/api/valuation/admin/condition-ratings/:id` | 删除车况等级 |
| POST | `/api/valuation/admin/region-coefficients` | 新增地区系数 |
| PUT | `/api/valuation/admin/region-coefficients/:id` | 更新地区系数 |
| DELETE | `/api/valuation/admin/region-coefficients/:id` | 删除地区系数 |
| POST | `/api/valuation/admin/original-prices` | 新增原厂价格 |
| PUT | `/api/valuation/admin/original-prices/:id` | 更新原厂价格 |
| DELETE | `/api/valuation/admin/original-prices/:id` | 删除原厂价格 |
| PUT | `/api/valuation/admin/coefficient-configs/:key` | 更新系数配置（按 key） |

---

## 附件与静态资源

- `/static/uploads/<path>`：上传文件（章节课件、视频、图片、PDF 报告等）
- `/static/<path>`：其他静态资源
- 支持 GET 与 HEAD（前端文档/图片预览用 HEAD 探测存在性）
- 包含 `..` 的路径会被拒绝（防路径穿越）
