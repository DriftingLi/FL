# API 接口清单

本文档为叉车维修培训系统 + 残值评估子系统 + AI 助手的 HTTP 接口总清单（路径 / 方法 / 鉴权 / 请求格式 / 返回格式）。

> **在线交互文档（gin-swagger，C 方案）**：开发/测试环境 `GET /swagger/index.html`（需 BasicAuth `SWAGGER_USER`/`SWAGGER_PASS`，见 `.secret/swagger-credentials.pem` 与 GitHub Secrets）；生产默认关闭（`SWAGGER_ENABLED=false`）。本清单保留作离线对照，以 Swagger UI 为准。

> **准确性基准**：本清单以 `backend/internal/api` 实际注册的路由与 `backend/internal/service` 的 typed DTO 契约为准（2026-09-04，#517 投稿域后）。与历史文档的差异（已下线端点等）见文末「变更记录」。

## 0. 通用约定

- 基础路径：`/api`；静态资源：`/static/*`
- 鉴权方式：`Authorization: Bearer <access JWT>`（access 2h 过期，用 `POST /api/auth/refresh` 以 refresh token 换新双令牌，见 ADR-0016）；部分接口同时写入登录 Cookie（HttpOnly，仅携带 access）
- 角色：`admin`（管理员）/ `tutor`（讲师）/ `hrwai_user`（学员/普通用户）/ `recruiter`（企业招聘者，独立表 `recruiter_users`，host-only `recruiter_token`）
- 响应统一包裹 `{ "code": number, "message": string, "data": any }`（AI 流式对话与文件下载除外，见各节）
- 响应码：`200` 成功 / `201` 创建成功 / `400` 参数或业务错误 / `401` 未认证 / `403` 无权限 / `404` 不存在 / `500` 服务器错误；错误时 `data` 为 `null`
- 分页约定：query 参数 `page`（默认 1）、`page_size`（默认见各端点）；返回体含 `total`，部分含 `pages`/`page`/`page_size`
- 时间格式：`YYYY-MM-DD HH:mm:ss`（ISO，北京时区）
- 限流：基于客户端 IP 的 token bucket；健康检查放行
- 上传限制：图片上传接口单文件大小受限（见各 handler 校验）

成功响应示例（分页列表）：

```json
{ "code": 200, "message": "success", "data": { "total": 10, "page": 1, "page_size": 12, ... } }
```

---

## 1. 系统与健康检查

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api` | 无 | 服务信息（版本号） |
| GET | `/api/health` | 无 | 健康检查（探测 Redis，异常返回 503 degraded） |
| GET | `/api/health/live` | 无 | 存活探针（liveness，仅进程存活） |
| GET | `/api/captcha` | 无 | 图形验证码（人机验证，发验证码前调用） |
| GET/HEAD | `/static/*filepath` | 无 | 静态资源与上传文件（uploads 前缀走上传目录） |

**GET /api/captcha**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "id": "c4d9...", "image": "data:image/png;base64,iVBOR..." } }
```

`image` 为 PNG base64 data URL，直接作为 `<img>` 展示；`id` 随 send-code 请求提交校验。图形验证码默认启用（`CAPTCHA_ENABLED`），未启用时 send-code 无需携带。

---

## 2. 账号认证 `/api/auth`

### 2.1 账号密码登录

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/login` | 无 | 学员/普通用户账号密码登录 |
| POST | `/api/auth/admin-login` | 无 | 管理员登录 |
| POST | `/api/auth/tutor-login` | 无 | 讲师登录 |
| POST | `/api/auth/refresh` | 无（refresh token 自身鉴权） | 刷新双令牌（轮换，旧 refresh 作废） |
| POST | `/api/auth/logout` | 无 | 登出（请求体带 refresh token 时将其撤销） |
| GET | `/api/auth/me` | JWT | 当前用户信息 |

**POST /api/auth/login**（admin-login / tutor-login 同格式）

请求体：

```json
{ "username": "13800000001", "password": "123456", "role": "hrwai_user" }
```

`role` 仅兼容历史，实际按登录端点区分角色。响应 200（同时写入登录 Cookie）：

```json
{ "code": 200, "message": "登录成功", "data": { "token": "eyJhbGciOi...", "refresh_token": "eyJhbGciOi...", "user_id": 1, "account": "13800000001", "username": "13800000001", "role": "hrwai_user" } }
```

双令牌说明：`token` 为 access（2h），`refresh_token` 为 refresh（7 天）；access 过期后以 refresh 调下述刷新端点换新对（轮换），refresh 仅存客户端本地、不写 Cookie。

**POST /api/auth/refresh**

请求体：`{ "refresh_token": "eyJhbGciOi..." }`。校验失败（类型不符/已撤销/过期）统一返回 401（防枚举）。响应 200：

```json
{ "code": 200, "message": "success", "data": { "token": "eyJhbGciOi...", "refresh_token": "eyJhbGciOi..." } }
```

**POST /api/auth/logout**

请求体：`{ "refresh_token": "eyJhbGciOi..." }`（可空；带 refresh token 时将其撤销入黑名单）。本端点不依赖 JWT——access 过期也能登出。响应 200：`{ "code": 200, "message": "已登出", "data": null }`

**GET /api/auth/me**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "user_id": 1, "uid": "10001", "account": "13800000001", "username": "13800000001", "role": "hrwai_user", "name": "张三", "avatar_url": "/static/uploads/avatars/xx.png", "phone": "13800000001", "email": "a@b.com", "company": "某公司", "has_password": true, "pending_profile_change": null } }
```

### 2.2 个人资料

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| PUT | `/api/auth/profile` | JWT | 修改昵称或单位（昵称走资料审核流，单位立即生效） |
| POST | `/api/auth/avatar` | JWT | 上传头像（multipart，字段名 `file`；同样走资料审核流） |
| DELETE | `/api/auth/account` | JWT | 注销当前账号（硬删除，论坛内容匿名化） |

**PUT /api/auth/profile**

请求体：`{ "nickname": "新昵称" }` 或 `{ "company": "新单位" }`

- `nickname`：走资料审核流（提交 → 审核 → 生效），响应为待审核请求
- `company`：立即生效，无需审核

响应 200（昵称走审核流，单位立即生效）：

```json
{ "code": 200, "message": "修改申请已提交，等待审核", "data": { "id": 1, "user_id": 1, "username": "13800000001", "avatar_url": "", "field_type": "nickname", "old_value": "旧昵称", "new_value": "新昵称", "status": "pending", "reject_reason": "", "created_at": "2026-08-16 10:00:00" } }
```

单位更新成功响应：`{ "code": 200, "message": "单位更新成功", "data": {} }`

**POST /api/auth/avatar**

multipart/form-data：`file`（图片）。响应 200：`data` 为头像修改审核请求（同上结构，`field_type: "avatar"`）。

**DELETE /api/auth/account**

注销当前学员账号，硬删除用户及关联数据（练习记录、错题、收藏、模拟考试、学习位置、打卡、站内信、评论与笔记），论坛帖子/回复匿名化为“已注销用户”，并清理登录态。需本地二次确认（输入账号），无需短信验证码。响应 200：`{ "code": 200, "message": "帐号已注销", "data": null }`

### 2.3 邮箱验证码 `/api/auth/email`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/email/send-code` | 无 | 发送邮箱验证码 |
| POST | `/api/auth/email/register` | 无 | 邮箱验证码注册 |
| POST | `/api/auth/email/login` | 无 | 邮箱验证码登录 |
| POST | `/api/auth/email/reset-password` | 无 | 忘记密码：验证码重置密码 |

### 2.4 手机号验证码 `/api/auth/phone`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/phone/send-code` | 无 | 发送手机验证码 |
| POST | `/api/auth/phone/register` | 无 | 手机验证码注册 |
| POST | `/api/auth/phone/login` | 无 | 手机验证码登录 |
| POST | `/api/auth/phone/reset-password` | 无 | 忘记密码：验证码重置密码 |

**POST /api/auth/{email|phone}/send-code**

请求体（`target` 为对应通道字段名；图形验证码启用时必填）：

```json
{ "phone": "13800000001", "purpose": "register", "captcha_id": "c4d9...", "captcha_value": "ab12" }
```

`purpose` 取值：`register` | `login` | `reset_password`。响应 200：`{ "code": 200, "message": "验证码已发送", "data": null }`

**POST /api/auth/{email|phone}/register**

请求体：`{ "phone": "13800000001", "code": "123456", "nickname": "张三", "company": "某公司", "password": "Test1234" }`（company 可选）

响应 201（自动登录，data 同 2.1 登录返回）：`{ "code": 201, "message": "注册成功", "data": { "token": "...", "refresh_token": "...", "user_id": 1, "account": "...", "username": "...", "role": "hrwai_user" } }`

**POST /api/auth/{email|phone}/login**

请求体：`{ "phone": "13800000001", "code": "123456" }`。响应 200，data 同登录返回。

**POST /api/auth/{email|phone}/reset-password**

请求体：`{ "phone": "13800000001", "code": "123456", "password": "NewPass123" }`。响应 200：`{ "code": 200, "message": "密码重置成功", "data": null }`

### 2.5 微信小程序登录 `/api/auth/wx-login`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/wx-login` | 无 | 微信小程序一键登录（code 换 openid，未注册自动建号） |

**POST /api/auth/wx-login**

请求体：`{ "code": "uni.login 临时凭证" }`。后端以 code 调微信 code2session 换 openid：已绑定用户直接登录；未注册自动建号（account 取 `wx_`+openid 前 12 位，昵称「微信学员」+openid 后 6 位）并绑定 openid。响应 200，data 为登录结果平铺结构（契约见 `docs/docs/reference/微信小程序登录-文档说明.md`，AppID 通过环境变量 `WECHAT_MINI_PROGRAM_APP_ID`（GitHub Secrets 同名）配置，勿硬编码；小程序凭证与开放平台扫码凭证（`WECHAT_OPEN_PLATFORM_*`）严格区分）：

```json
{ "code": 200, "message": "登录成功", "data": { "token": "jwt", "refresh_token": "jwt", "user_id": 1, "account": "wx_oABC_123456", "username": "微信学员123456", "name": "微信学员123456", "role": "hrwai_user", "avatar": "", "isNew": true } }
```

`isNew` 仅新用户为 true（前端据此提示已自动注册）。错误分支均 400：缺 code、未配置 AppID/Secret、code 失效（40029）、频率限制（45011）、高风险拦截（40226）。小程序凭证经环境变量 `WECHAT_MINI_PROGRAM_APP_ID` / `WECHAT_MINI_PROGRAM_APP_SECRET` 配置。

### 2.6 微信扫码登录 `/api/auth/wechat`（框架占位）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/wechat/qrcode` | 无 | 获取登录二维码 |
| POST | `/api/auth/wechat/login` | 无 | 微信扫码登录 |

### 2.7 账号绑定修改（短信/邮箱验证码确认）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/profile/send-code` | JWT | 发送绑定验证码 |
| POST | `/api/auth/profile/email` | JWT | 绑定/修改邮箱 |
| POST | `/api/auth/profile/phone` | JWT | 绑定/修改手机号 |
| POST | `/api/auth/profile/password/send-code` | JWT | 修改密码：发送短信验证码 |
| POST | `/api/auth/profile/password` | JWT | 修改密码（短信验证码确认） |
| POST | `/api/auth/account/send-code` | JWT | 修改账号：发送短信验证码 |
| PUT | `/api/auth/account` | JWT | 修改登录账号（响应携带新双令牌） |

**POST /api/auth/profile/send-code**（account/send-code、password/send-code 同格式）

请求体：`{ "channel": "phone", "target": "13800000001" }`

**POST /api/auth/profile/email** / **phone**

请求体：`{ "target": "new@example.com", "code": "123456" }`

**POST /api/auth/profile/password**

请求体：`{ "code": "123456", "password": "NewPass123" }`

**PUT /api/auth/account**

请求体：`{ "account": "13900000000", "code": "123456" }`

响应 200，data 为登录结果（含**新 token 与新 refresh_token**，客户端需更新存储）。

---

## 3. 课程学习 `/api`

### 3.1 公开接口

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/courses` | 无 | 课程列表（分页；可按专业方向/等级过滤） |
| GET | `/api/catalog/tree` | 无 | 课程目录树：专业方向 → 课程等级 → 课程（含章节数） |
| GET | `/api/levels` | 无 | 课程等级列表（仅启用项） |
| GET | `/api/tags` | 无 | 题库标签列表（仅启用项，含已发布题目数） |
| GET | `/api/chapter/:chapter_id/slides` | 无 | 章节课件列表（图片 URL 数组） |

**GET /api/courses**

Query：`page`（默认 1）、`page_size`（默认 12）、`specialty_id`、`level_id`

响应 200：

```json
{ "code": 200, "message": "success", "data": { "courses": [ { "course_id": 1, "name": "叉车基本构造", "description": "...", "cover_image": "/static/uploads/covers/xx.jpg", "duration": 120, "specialty_id": 1, "level_id": 2, "theory_hours": 20, "practice_hours": 10, "certificate_template_id": 1, "prerequisite_course_ids": null, "sort_order": 1, "status": 1, "created_at": "2026-08-01 10:00:00", "chapter_count": 3, "student_count": 12, "specialty": { "specialty_id": 1, "code": "xx", "name": "内燃叉车" }, "level": { "level_id": 2, "code": "advanced", "name": "进阶" }, "certificate_template": { "id": 1, "code": "CT01", "name": "结业证书", "description": "", "template_url": "", "validity_days": 365 } } ], "page": 1, "pages": 1, "total": 1 } }
```

**GET /api/catalog/tree**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "specialties": [ { "specialty_id": 1, "code": "xx", "name": "内燃叉车", "description": "", "sort_order": 1, "status": 1, "created_at": "...", "levels": [ { "level_id": 2, "code": "advanced", "name": "进阶", "description": "", "sort_order": 1, "status": 1, "created_at": "...", "courses": [ { "course_id": 1, "name": "叉车基本构造", "description": "", "cover_image": "", "duration": 120, "specialty_id": 1, "level_id": 2, "theory_hours": 20, "practice_hours": 10, "certificate_template_id": null, "sort_order": 1, "status": 1, "created_at": "..." } ] } ] } ] } }
```

**GET /api/levels**：同上，元素为 `{ level_id, code, name, description, sort_order, status, created_at }`

**GET /api/tags**：data 为数组，元素 `{ id, code, name, description, sort_order, status, created_at, updated_at, question_count }`

**GET /api/chapter/:chapter_id/slides**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "chapter_id": 1, "slides": ["/static/uploads/slides/1/1.png", "/static/uploads/slides/1/2.png"] } }
```

### 3.2 登录接口

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/course/:course_id` | JWT | 课程详情（含章节、学习进度与学习位置，ADR-0017） |
| GET | `/api/course/:course_id/chapter/:chapter_id` | JWT | 章节详情（含课件文件、上下章、学习状态） |
| POST | `/api/chapter/:chapter_id/slides/regenerate` | JWT | 重新生成章节课件（AI） |
| POST | `/api/course/:course_id/progress` | JWT | 上报学习进度（分钟/秒级时长、播放位置、显式完成，ADR-0017） |

**GET /api/course/:course_id**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "course_info": { "course_id": 1, "name": "叉车基本构造", "description": "...", "cover_image": "", "duration": 120, "specialty_id": 1, "level_id": 2, "theory_hours": 20, "practice_hours": 10, "certificate_template_id": null, "sort_order": 1, "status": 1, "created_at": "..." }, "chapters": [ { "chapter_id": 1, "course_id": 1, "title": "第一章 叉车分类与型号", "content": "markdown 图文内容", "content_type": "text", "description": "", "duration": 40, "file_url": "", "order_num": 1, "created_at": "..." } ], "progress": 33, "is_enrolled": true, "completed_chapters": 1, "last_chapter_id": 1, "last_position": 823, "last_studied_at": "2026-08-19T02:00:00.000000" } }
```

`progress` 为 0-100 的课程学习进度（浮点）。学习位置字段（ADR-0017）：`is_enrolled` 以「存在学习记录」代理报名语义；`last_chapter_id`/`last_position`（秒）/`last_studied_at` 为最后学习章节与位置，未学时为零值/null。

**GET /api/course/:course_id/chapter/:chapter_id**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "chapter_id": 1, "course_id": 1, "title": "第一章", "content": "...", "content_type": "text", "description": "", "duration": 40, "file_url": "", "order_num": 1, "created_at": "...", "files": [ { "file_id": 1, "chapter_id": 1, "file_name": "课件1.pdf", "file_url": "/static/uploads/chapters/xx.pdf", "content_type": "document", "file_size": 204800, "created_at": "..." } ], "previous_chapter_id": 0, "next_chapter_id": 2, "study_status": "completed" } }
```

`content_type`：`text` | `video` | `document` | `ppt` | `image`；`study_status`：`completed` / `studying` / 空。

**POST /api/course/:course_id/progress**

请求体：

```json
{ "chapter_id": 1, "duration": 2, "duration_seconds": 95, "video_position": 823, "completed": false }
```

`duration` 为本次学习增量（分钟，≥0）；新增字段全部可选（ADR-0017）：`duration_seconds`（秒，>0 时优先并按 ceil 换算分钟累加）、`video_position`（该章节最后播放位置，秒，≥0）、`completed`（显式完成该章节，直接置 progress=100）。带 `chapter_id` 的上报会刷新该课程最后学习位置。响应 200：

```json
{ "code": 200, "message": "学习进度更新成功", "data": { "progress": 33, "record_id": 12, "study_duration": 45, "completed_chapters": 1 } }
```

---

## 4. 学员端 `/api/student`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/student/profile` | 学员个人资料 + 学习统计 + 各课程进度 |
| GET | `/api/student/records` | 学习/考试记录（分页，可按日期过滤） |
| GET | `/api/student/study-stats` | 学习统计（按天聚合，days=7|30） |
| GET | `/api/student/courses` | 我的课程（含最后学习位置与 continue_learning，ADR-0017） |
| GET | `/api/student/courses/:course_id` | 单课程学习详情（每章进度/播放位置/完成状态） |

**GET /api/student/profile**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "student_info": { "student_id": 1, "uid": "10001", "account": "13800000001", "username": "13800000001", "avatar_url": "", "status": 1, "created_at": "..." }, "study_stats": { "total_courses": 6, "total_study_duration": 360, "completed_courses": 1, "learning_courses": 2, "latest_study_time": "2026-08-16 10:00:00" }, "course_progress": [ { "course_id": 1, "course_name": "叉车基本构造", "progress": 67, "study_duration": 120, "total_chapters": 3, "study_date": "2026-08-15" } ] } }
```

`total_study_duration` 单位为分钟。

**GET /api/student/records**

Query：`page`（默认 1）、`page_size`（默认 10）、`start_date`（YYYY-MM-DD）、`end_date`（YYYY-MM-DD）

响应 200：

```json
{ "code": 200, "message": "success", "data": { "page": 1, "pages": 1, "records": [ { "record_id": 1, "student_id": 1, "course_id": 1, "chapter_id": 1, "study_duration": 2, "progress": 33, "study_date": "2026-08-16 10:00:00", "course_name": "叉车基本构造", "chapter_title": "第一章" } ], "total": 1 } }
```

`chapter_id`/`chapter_title` 为 null 表示课程级记录。

**GET /api/student/study-stats?days=7**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "days": 7, "labels": ["08-10", "08-11", "08-12", "08-13", "08-14", "08-15", "08-16"], "data": [0, 0, 0, 30, 45, 0, 60], "total_minutes": 135, "active_days": 3 } }
```

**GET /api/student/courses**

我的课程（该学员已开始学习的课程，按 `last_studied_at` 倒序）；`continue_learning` 为最后学习时间最新的课程（无学习记录时为 null）。响应 200：

```json
{ "code": 200, "message": "success", "data": { "courses": [ { "course_id": 1, "course_name": "叉车基本构造", "cover": "/static/covers/c1.png", "specialty_id": 1, "level_id": 2, "progress": 68, "completed_chapters": 5, "total_chapters": 8, "study_duration": 120, "last_chapter_id": 1008, "last_chapter_title": "第三章 液压系统", "last_position": 823, "last_studied_at": "2026-08-19T02:00:00.000000" } ], "continue_learning": { "course_id": 1, "course_name": "叉车基本构造", "cover": "...", "specialty_id": 1, "level_id": 2, "progress": 68, "completed_chapters": 5, "total_chapters": 8, "study_duration": 120, "last_chapter_id": 1008, "last_chapter_title": "第三章 液压系统", "last_position": 823, "last_studied_at": "2026-08-19T02:00:00.000000" } } }
```

**GET /api/student/courses/:course_id**

单课程学习详情：我的课程条目字段 + 全部章节的学习状态（未学章节零值并入，按章节顺序）。响应 200：

```json
{ "code": 200, "message": "success", "data": { "course_id": 1, "course_name": "叉车基本构造", "cover": "...", "specialty_id": 1, "level_id": 2, "progress": 68, "completed_chapters": 5, "total_chapters": 8, "study_duration": 120, "last_chapter_id": 1008, "last_chapter_title": "第三章 液压系统", "last_position": 823, "last_studied_at": "...", "chapters": [ { "chapter_id": 1008, "title": "第三章 液压系统", "progress": 40, "video_position": 823, "completed": false } ] } }
```

---

## 5. 题库 `/api/question-bank`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/question-bank/questions` | JWT | 题库列表（分页；按题型/状态/关键词/标签过滤） |
| POST | `/api/question-bank/questions` | JWT+tutor/admin | 新增题目 |
| GET | `/api/question-bank/questions/:question_id` | JWT | 题目详情 |
| PUT | `/api/question-bank/questions/:question_id` | JWT+tutor/admin | 更新题目 |
| DELETE | `/api/question-bank/questions/:question_id` | JWT+tutor/admin | 删除题目 |
| POST | `/api/question-bank/questions/:question_id/publish` | JWT+admin | 发布题目 |
| POST | `/api/question-bank/questions/:question_id/reject` | JWT+admin | 驳回题目 |
| POST | `/api/question-bank/questions/batch-publish` | JWT+admin | 批量发布 |
| POST | `/api/question-bank/questions/batch-reject` | JWT+admin | 批量驳回 |
| POST | `/api/question-bank/questions/batch-import` | JWT+tutor/admin | 批量导入 |
| GET | `/api/question-bank/stats` | JWT | 题库统计 |
| POST | `/api/question-bank/upload-image` | JWT+tutor/admin | 上传题图 |

> 注：历史版本中的 `/api/question-bank/categories`、`/api/question-bank/knowledge-points` 及知识点 CRUD 已下线（404），当前按题型 `type` 与标签 `tag_id` 体系组织。

**GET /api/question-bank/questions**

Query：`page`（默认 1）、`page_size`（默认 20）、`type`（single_choice/multi_choice/true_false/short_answer/fault_image）、`status`（draft/published/rejected）、`keyword`（内容模糊匹配）、`tag_id`

响应 200（管理面含答案与解析；学员侧由练习/考试接口返回脱敏版）：

```json
{ "code": 200, "message": "success", "data": { "total": 1, "page": 1, "page_size": 20, "questions": [ { "id": 1, "content": "叉车的额定起重量指？", "options": [ { "key": "A", "text": "最大起重量" } ], "type": "single_choice", "score": 3, "status": "published", "image_url": "", "answer": "A", "explanation": "...", "reference_answer": null, "scoring_criteria": null, "created_by": 1, "created_by_type": "admin", "reject_reason": "", "created_at": "...", "updated_at": "...", "tags": [ { "id": 1, "code": "T01", "name": "基础" } ] } ] } }
```

**POST /api/question-bank/questions**

请求体（options 为 `[{key, text}]` 数组；简答题可带 `reference_answer`/`scoring_criteria`，识图题带 `image_url`）：

```json
{ "content": "叉车的额定起重量指？", "options": [ { "key": "A", "text": "最大起重量" }, { "key": "B", "text": "最小起重量" } ], "type": "single_choice", "score": 3, "answer": "A", "explanation": "解析", "image_url": "", "reference_answer": "", "scoring_criteria": "", "tag_ids": [1, 2] }
```

响应 200：data 为题目详情（同列表 questions 元素）。

**POST /api/question-bank/questions/batch-publish** / **batch-reject**

请求体：`{ "question_ids": [1, 2] }`；reject 额外 `{ "question_ids": [1], "reason": "重复" }`

**POST /api/question-bank/questions/batch-import**

请求体：`{ "questions": [ { "content": "...", "type": "single_choice", "options": [...], "answer": "A", "score": 3 } ] }`

**GET /api/question-bank/stats**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "total": 100, "by_type": { "single_choice": 60, "multi_choice": 20, "true_false": 10, "short_answer": 5, "fault_image": 5 }, "by_status": { "draft": 10, "published": 80, "rejected": 10 } } }
```

**POST /api/question-bank/upload-image**

multipart/form-data：`file`。响应 200：data 为 `{ "url": "..." }`。

---

## 6. 题库练习 `/api/practice-mode`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/practice-mode/free` | 自由刷题（count 题量，type 题型过滤） |
| GET | `/api/practice-mode/tag` | 标签练习抽题（tag_id 必填，count 控制题量，0=全部） |
| GET | `/api/practice-mode/sequential` | 顺序练习（全部题目顺序刷题） |
| GET | `/api/practice-mode/sequential-progress` | 顺序练习进度 |
| GET | `/api/practice-mode/progress` | 查询练习进度（?mode=sequential） |
| POST | `/api/practice-mode/progress` | 保存练习进度 |
| POST | `/api/practice-mode/submit` | 提交单题答案（自动判分，简答题走 AI 阅卷） |
| GET | `/api/practice-mode/stats` | 练习统计 |
| GET | `/api/practice-mode/history` | 练习历史（分页+过滤） |

> 注：历史版本中的 `/api/practice-mode/category`、`/api/practice-mode/knowledge-point`、`/api/practice-mode/knowledge-point-progress` 已下线（404）。

**GET /api/practice-mode/free?count=20&type=single_choice**

响应 200（tag/sequential 同结构；学员侧题目不含答案）：

```json
{ "code": 200, "message": "success", "data": { "questions": [ { "id": 1, "content": "叉车的额定起重量指？", "options": [ { "key": "A", "text": "最大起重量" } ], "type": "single_choice", "score": 3, "status": "published", "image_url": "", "created_by": 1, "created_by_type": "admin", "reject_reason": "", "created_at": "...", "updated_at": "..." } ], "current_index": 0, "total": 20, "completed": 0 } }
```

**GET /api/practice-mode/tag?tag_id=1&count=10**

Query：`tag_id`（必填）、`count`（0=全部）。

**GET /api/practice-mode/sequential**：无参数；**GET /api/practice-mode/sequential-progress**：无参数，返回 ProgressResultDTO（见下）。

**GET /api/practice-mode/progress?mode=sequential**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "completed": 5, "total": 20, "current_index": 5, "answers_state": { "1": "A", "2": ["A", "C"] } } }
```

**POST /api/practice-mode/progress**

请求体：

```json
{ "mode": "sequential", "index": 5, "total": 20, "answers_state": { "1": "A" } }
```

**POST /api/practice-mode/submit**

请求体：

```json
{ "question_id": 1, "user_answer": "A", "practice_type": "free" }
```

`user_answer`：客观题为字符串（多选为逗号拼接或数组），简答题为文本。响应 200：

```json
{ "code": 200, "message": "success", "data": { "is_correct": true, "correct_answer": "A", "explanation": "解析", "question_id": 1, "user_answer": "A", "accuracy_rate": 85.5, "common_wrong": "B", "total_attempts": 120, "ai_explanation": "本题考查..." } }
```

- `accuracy_rate`：全站正确率（基于 `question_practice_record` 聚合，样本 `<5` 时不返回，前端显示“—”）
- `common_wrong`：易错项（全体答错样本中最多的选项；多选按选项组合聚合；简答题不返回）
- `ai_explanation`：AI 解析（按需生成并缓存到题目，未配置 AI 时降级为静态 `explanation`）

简答题追加 `reference_answer`/`scoring_criteria`/`max_score`，AI 评分后追加 `ai_score`/`ai_comment`（降级时 `ai_fallback: true`）；未判定前 `is_correct` 为 null。

**GET /api/practice-mode/stats**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "total": 50, "correct": 40, "wrong": 10, "accuracy": 0.8, "by_type": { "single_choice": { "total": 30, "correct": 25, "accuracy": 0.833 } } } }
```

**GET /api/practice-mode/history?page=1&page_size=20&type=&start_date=&end_date=**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "total": 10, "page": 1, "page_size": 20, "records": [ { "id": 1, "student_id": 1, "question_id": 1, "is_correct": true, "practice_type": "free", "user_answer": "A", "created_at": "...", "question": { "id": 1, "content": "...", "type": "single_choice", "options": [...], "score": 3, "status": "published", "image_url": "", "created_by": 1, "created_by_type": "admin", "reject_reason": "", "created_at": "...", "updated_at": "..." } } ] } }
```

### 6a. 题目互动 `/api/questions`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/questions/:question_id/comments` | 题目评论列表（分页） |
| POST | `/api/questions/:question_id/comments` | 发表评论 |
| DELETE | `/api/questions/comments/:comment_id` | 删除本人评论 |
| GET | `/api/questions/:question_id/knowledge` | 题目考点（题库标签只读，无标签返回空数组） |
| GET | `/api/questions/:question_id/note` | 获取本人笔记 |
| PUT | `/api/questions/:question_id/note` | 保存笔记（每人每题一条，覆盖更新） |
| DELETE | `/api/questions/:question_id/note` | 删除笔记 |

**GET /api/questions/:question_id/comments?page=1&page_size=10**

响应 200：`{ "code": 200, "message": "success", "data": { "items": [ { "id": 1, "question_id": 1, "user_id": 2, "content": "这题易错", "created_at": "..." } ], "total": 1, "page": 1, "page_size": 10 } }`

前端展示最新 3 条 + 总数，“查看所有评论”弹窗分页；直发不预审，本人可删。

**GET /api/questions/:question_id/knowledge**

响应 200：`{ "code": 200, "message": "success", "data": [ { "id": 1, "code": "hydraulic", "name": "液压系统", "description": "...", "sort_order": 0, "status": 1 } ] }`（无标签返回 `[]`，前端显示“暂无”）

**PUT /api/questions/:question_id/note**

请求体：`{ "content": "我的笔记内容" }`。每人每题仅一条，重复保存即覆盖。响应 200：`{ "code": 200, "message": "success", "data": { "id": 1, "question_id": 1, "user_id": 2, "content": "...", "updated_at": "..." } }`

错题重做 `POST /api/wrong-questions/:question_id/redo` 同步返回上述 `accuracy_rate`/`common_wrong`/`ai_explanation` 等增强字段。

---

## 7. 模拟考试 `/api/mock-exam`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/mock-exam/start` | 开始模拟考（count 题量 + duration 时长分钟） |
| POST | `/api/mock-exam/:mock_exam_id/save` | 保存进度 |
| GET | `/api/mock-exam/:mock_exam_id/resume` | 恢复考试 |
| POST | `/api/mock-exam/:mock_exam_id/submit` | 交卷（客观题自动判分，简答题 AI 阅卷） |
| GET | `/api/mock-exam/:mock_exam_id/result` | 考试结果 |
| GET | `/api/mock-exam/history` | 考试历史（分页） |

**POST /api/mock-exam/start**

请求体：`{ "count": 20, "duration": 30 }`

响应 200：

```json
{ "code": 200, "message": "success", "data": { "mock_exam_id": 5, "duration": 30, "total_score": 100, "total_questions": 20, "remaining_time": 1800, "questions": [ { "id": 1, "content": "...", "options": [...], "type": "single_choice", "score": 3 } ] } }
```

**POST /api/mock-exam/:mock_exam_id/save**

请求体：`{ "answers": { "1": "A" }, "remaining_time": 1500 }`

**GET /api/mock-exam/:mock_exam_id/resume**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "mock_exam_id": 5, "duration": 30, "remaining_time": 1200, "questions": [...], "answers": { "1": "A" }, "start_time": "2026-08-16 10:00:00" } }
```

**POST /api/mock-exam/:mock_exam_id/submit**

请求体：`{}`。响应 200：

```json
{ "code": 200, "message": "success", "data": { "total_score": 80, "max_score": 100, "correct_count": 17, "total_questions": 20, "accuracy": 0.85, "details": [ { "question_id": 1, "type": "single_choice", "content": "...", "user_answer": "A", "correct_answer": "A", "score": 3, "max_score": 3, "explanation": "...", "options": [...], "is_correct": true } ] } }
```

**GET /api/mock-exam/:mock_exam_id/result**

响应 200：data 为提交结果 + `{ "mock_exam_id": 5, "submit_time": "..." }`

**GET /api/mock-exam/history?page=1&page_size=10**

响应 200：data 为 `{ total, page, page_size, exams: [ { id, student_id, question_ids, answers, start_time, submit_time, remaining_time, duration, status, result, created_at, score } ] }`（status：completed/in_progress；result 为提交结果对象）

---

## 8. 讲师端 `/api/tutor`（role=tutor）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tutor/courses` | 讲师课程列表（分页；可按方向/等级过滤） |
| GET | `/api/tutor/course/:course_id/chapters` | 课程章节列表（含文件） |
| GET | `/api/tutor/chapter/:chapter_id` | 章节详情（含上下章 ID + 文件列表） |
| POST | `/api/tutor/chapter/:chapter_id/upload` | 上传章节文件（课件/视频等） |
| POST | `/api/tutor/upload-image` | 上传图文 Markdown 图片（Vditor 格式） |
| PUT | `/api/tutor/chapter/:chapter_id` | 更新章节信息 |
| DELETE | `/api/tutor/file/:file_id` | 删除章节文件 |
| POST | `/api/tutor/files/batch-delete` | 批量删除文件 |

**GET /api/tutor/courses?page=1&page_size=10&specialty_id=&level_id=**

响应 200：data 为 `{ courses: [CourseDTO], page, pages, total }`（同 3.1）。

**GET /api/tutor/course/:course_id/chapters**

响应 200：data 为 `{ "course": { CourseDTO }, "chapters": [ChapterDTO 含 files] }`

**POST /api/tutor/chapter/:chapter_id/upload**

multipart/form-data：`file`（字段名 file）。响应 200：data 为 `{ "file_id": 1, "file_name": "...", "file_url": "...", "content_type": "...", "file_size": 204800 }`

**POST /api/tutor/upload-image**

multipart/form-data：`file`。响应 200（Vditor 约定结构）。

**PUT /api/tutor/chapter/:chapter_id**

请求体：`{ "title": "新标题", "content": "markdown", "duration": 40, "order_num": 1, "description": "" }`

**POST /api/tutor/files/batch-delete**

请求体：`{ "file_ids": [1, 2] }`

---

## 11. 错题本 `/api/wrong-questions`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/wrong-questions` | 错题列表（分页+过滤；`is_redone` 标记已重做） |
| POST | `/api/wrong-questions/:question_id/redo` | 重做错题（提交答案判分；做对标记 `is_redone` 而非移出） |
| POST | `/api/wrong-questions/:question_id/remove` | 移出错题本 |
| POST | `/api/wrong-questions/batch-remove` | 批量移出（`{question_ids:[]}`） |
| GET | `/api/wrong-questions/stats` | 错题统计 |
| GET | `/api/wrong-questions/export` | 导出错题本（纯文本附件；空列表不可导出，前端支持全选/单选导出） |

**GET /api/wrong-questions?page=1&page_size=20&type=&min_wrong_count=**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "total": 5, "page": 1, "page_size": 20, "items": [ { "question_id": 1, "content": "...", "type": "single_choice", "options": [...], "correct_answer": "A", "explanation": "...", "wrong_count": 2, "is_removed": false, "created_at": "..." } ] } }
```

**POST /api/wrong-questions/:question_id/redo**

请求体：`{ "user_answer": "B" }`。响应 200：data 为提交判定结果（同 practice submit）。

**POST /api/wrong-questions/:question_id/remove**：请求体 `{}`。响应 200：`{ "code": 200, "message": "已移出错题本", "data": { ... } }`

**GET /api/wrong-questions/stats**

响应 200：data 为 `{ "total": 5, "by_type": { "single_choice": 3 } }`

**GET /api/wrong-questions/export**：响应为 `text/plain` 附件下载（Content-Disposition attachment）。

---

## 12. 内容精选（公开）`/api`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/featured-contents` | 无 | 内容精选列表（仅已发布；分页+分类+排序过滤，支持最新/热点） |
| GET | `/api/featured-content/:id` | 无 | 内容详情（含相关资讯/上下一篇） |
| POST | `/api/featured-content/:id/view` | 无 | 浏览计数 +1 |

**GET /api/featured-contents?page=1&page_size=10&category=&sort=latest**

Query：`page`（默认 1）、`page_size`（默认 10）、`category` ∈ `company`（公司动态）/`industry`（行业新闻）/`product`（产品资讯）/`news`（政策法规）、`sort` ∈ `latest`（按发布时间，默认）/`hot`（按浏览量热点排序）。响应 200：

```json
{ "code": 200, "message": "success", "data": { "items": [ { "content_id": 1, "title": "叉车维保小知识", "category": "news", "category_label": "政策法规", "summary": "摘要", "cover_image": "/static/uploads/...", "source": "官网", "status": 1, "sort_order": 1, "view_count": 120, "published_at": "...", "created_at": "...", "updated_at": "..." } ], "page": 1, "pages": 1, "total": 1 } }
```

**GET /api/featured-content/:id**

响应 200：data 为列表项 + `content`（正文）、`related`（相关资讯数组）、`prev`/`next`（上/下一篇导航，null 表示无）。

**POST /api/featured-content/:id/view**：请求体 `{}`，响应 200 data 为 `{ content_id, view_count }`。

---

## 13. AI 助手 `/api/ai-assistant`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/ai-assistant/modes` | 无 | 双模式可用模型（普通/专家分别绑定，隐藏底层 model） |
| GET | `/api/ai-assistant/models` | 无 | 可用模型列表（兼容旧前端，聚合双模式） |
| POST | `/api/ai-assistant/chat` | 可选 JWT | 流式对话（SSE；登录可保存会话，支持 mode 普通/专家） |
| GET | `/api/ai-assistant/sessions` | JWT+hrwai_user | 会话列表 |
| POST | `/api/ai-assistant/sessions` | JWT+hrwai_user | 创建会话 |
| PATCH | `/api/ai-assistant/sessions/:id/title` | JWT+hrwai_user | 重命名会话 |
| DELETE | `/api/ai-assistant/sessions/:id` | JWT+hrwai_user | 删除会话 |
| GET | `/api/ai-assistant/sessions/:id/messages` | JWT+hrwai_user | 会话消息列表 |
| GET | `/api/ai-assistant/user-models` | JWT+hrwai_user | 用户自定义模型列表（api_key 脱敏，遗留兼容） |
| POST | `/api/ai-assistant/user-models` | JWT+hrwai_user | 保存自定义模型 |
| DELETE | `/api/ai-assistant/user-models/:id` | JWT+hrwai_user | 删除自定义模型 |

**GET /api/ai-assistant/modes**（新）

响应 200：

```json
{ "code": 200, "message": "success", "data": { "normal": { "id": 1, "name": "DeepSeek 普通", "model": "deepseek-chat", "base_url": "https://api.deepseek.com" }, "expert": { "id": 2, "name": "DeepSeek 专家", "model": "deepseek-reasoner", "base_url": "https://api.deepseek.com" } } }
```

`normal`/`expert` 为 null 表示该模式未绑定，前端对应禁用。管理端在 “AI 配置” 的 “AI助手模式绑定” 分别单绑定。

**GET /api/ai-assistant/models**（兼容旧）

响应 200：

```json
{ "code": 200, "message": "success", "data": [ { "id": 1, "name": "DeepSeek V3", "model": "deepseek-chat", "base_url": "https://api.deepseek.com" } ] }
```

**POST /api/ai-assistant/chat**（SSE 流式，非 JSON 信封）

请求体（新推荐 `mode`，隐藏底层模型）：

```json
{ "session_id": 1, "mode": "normal", "messages": [ { "role": "user", "content": "叉车液压系统常见故障？" } ] }
```

兼容旧：`{ "session_id": 1, "model_source": "admin", "config_id": 1, "messages": [...] }`

`mode`：`normal`（普通模式，绑定 `ai_assistant_normal`）| `expert`（专家模式，绑定 `ai_assistant_expert`）；`model_source` 仍支持 `admin|user|custom` 作回退。`session_id` 可选（登录用户指定会话；不传且已登录则新建）。

响应为 SSE 事件流（Content-Type: text/event-stream）：

```
event: message
data: {"content":"叉车液压系统常见故障包括…"}

event: message
data: {"content":"…（续）"}

event: done
data: null
```

出错时：`event: error` + `data: {"message":"错误信息"}`。客户端按 `event` 分派，逐段拼接 `message` 事件的内容。

**GET /api/ai-assistant/sessions**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "sessions": [ { "id": 1, "title": "新对话", "model_name": "deepseek-chat", "created_at": "...", "updated_at": "..." } ] } }
```

**POST /api/ai-assistant/sessions**

请求体：`{ "title": "新对话", "model_name": "deepseek-chat" }`。响应 200：data 为会话对象。

**PATCH /api/ai-assistant/sessions/:id/title**

请求体：`{ "title": "新标题" }`

**GET /api/ai-assistant/sessions/:id/messages**

响应 200（data 为数组）：`[ { "id": 1, "role": "user", "content": "...", "created_at": "..." } ]`

**GET /api/ai-assistant/user-models**

响应 200（data 为数组）：`[ { "id": 1, "name": "我的模型", "api_key": "sk-***", "base_url": "...", "model": "...", "created_at": "...", "updated_at": "..." } ]`

**POST /api/ai-assistant/user-models**

请求体：`{ "name": "我的模型", "api_key": "sk-xxx", "base_url": "https://...", "model": "gpt-4o-mini" }`（带 `id` 为更新）。响应 200：data 为模型对象。

---

## 14. 论坛 `/api/forum`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/forum/upload-image` | 上传论坛图片（图文分离，先传图后随发帖/回复提交 URL） |
| GET | `/api/forum/topics` | 帖子列表（scope=all|general|chapter；keyword 搜索；分页；`sort=latest|hot` `order=asc|desc`） |
| POST | `/api/forum/topics` | 发帖（images 最多 9 张） |
| GET | `/api/forum/topics/:id` | 帖子详情（含回复；`sort=latest|hot|time` `order=asc|desc`） |
| POST | `/api/forum/topics/:id/replies` | 回复（images 最多 3 张；支持回复楼层） |
| DELETE | `/api/forum/topics/:id` | 删除自己的帖子 |
| DELETE | `/api/forum/replies/:id` | 删除自己的回复 |
| POST | `/api/forum/topics/:id/like` | 点赞（幂等，ADR-0018） |
| DELETE | `/api/forum/topics/:id/like` | 取消点赞（幂等） |
| POST | `/api/forum/topics/:id/report` | 举报主题（reason 1-500 字） |
| POST | `/api/forum/replies/:id/report` | 举报回复 |
| GET | `/api/forum/my-topics` | 我的帖子（分页） |
| GET | `/api/forum/my-replies` | 我的回复（分页，含主题标题回填） |

**管理端 `/api/admin/forum`（role=admin）**

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/forum/topics` | 帖子列表 |
| GET | `/api/admin/forum/topics/:id` | 帖子详情 |
| DELETE | `/api/admin/forum/topics/:id` | 删除帖子 |
| DELETE | `/api/admin/forum/replies/:id` | 删除回复 |
| GET | `/api/admin/forum/reports?status=&page=&page_size=` | 举报列表（status 0 待处理/1 已处理，缺省全部） |
| PUT | `/api/admin/forum/reports/:id` | 处理举报（body: `{"status": 1}`） |

**POST /api/forum/upload-image**

multipart/form-data：`file`。响应 200：data 为 `{ "url": "/static/uploads/forum/xx.png" }`

**GET /api/forum/topics?scope=all&chapter_id=&page=1&page_size=10&keyword=**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "page": 1, "pages": 1, "topics": [ { "id": 1, "chapter_id": null, "chapter_title": null, "title": "求助：叉车启动困难", "content": "如题", "images": [], "view_count": 10, "reply_count": 2, "last_reply_at": "...", "created_at": "...", "author": { "user_id": 1, "username": "13800000001", "avatar_url": "" }, "can_delete": true } ], "total": 1 } }
```

**POST /api/forum/topics**

请求体：`{ "title": "求助：叉车启动困难", "content": "如题", "chapter_id": 0, "images": ["/static/uploads/forum/xx.png"] }`（chapter_id 为空/0 表示综合讨论区）

响应 200：data 为帖子对象（同列表 topics 元素）。

**GET /api/forum/topics/:id**

响应 200：data 为帖子对象 + `replies` 数组：

```json
{ "code": 200, "message": "success", "data": { "id": 1, "title": "...", "content": "...", "images": [], "view_count": 10, "reply_count": 2, "last_reply_at": "...", "created_at": "...", "author": { ... }, "can_delete": true, "likes_count": 3, "liked_by_me": true, "replies": [ { "id": 1, "topic_id": 1, "parent_id": null, "parent_name": null, "content": "回复内容", "images": [], "created_at": "...", "author": { ... }, "can_delete": true } ] } }
```

**POST /api/forum/topics/:id/replies**

请求体：`{ "content": "回复内容", "images": [], "parent_reply_id": 0 }`

**DELETE /api/forum/topics/:id** / **DELETE /api/forum/replies/:id**：请求体 `{}`，响应 200 `{ "code": 200, "message": "删除成功", "data": null }`。

---

## 14a. 通用收藏 `/api/favorites`（role=hrwai_user，ADR-0018）

多态收藏：`target_type` ∈ course / chapter / question / featured / topic；user+type+id 唯一（幂等）。收藏时校验目标存在且可见（课程=已发布+挂载、题目=published、精选=已发布）；列表实时回填目标快照，目标已删除的条目不出现。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/favorites?target_type=&page=&page_size=` | 我的收藏列表（快照回填） |
| POST | `/api/favorites` | 收藏（body: `{ "target_type": "course", "target_id": 1 }`，幂等） |
| DELETE | `/api/favorites/:id` | 取消收藏（仅本人） |
| GET | `/api/favorites/check?target_type=&target_id=` | 是否已收藏 |

**POST /api/favorites** 响应 201：

```json
{ "code": 201, "message": "收藏成功", "data": { "favorite_id": 1, "target_type": "course", "target_id": 1, "title": "叉车基本构造", "cover": "/static/covers/c1.png", "created_at": "..." } }
```

**GET /api/favorites/check** 响应 200：`{ "code": 200, "message": "success", "data": { "favorited": true, "favorite_id": 1 } }`

## 14b. 全局搜索 `/api/search`（公开，ADR-0018）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/search?keyword=&type=&page=&page_size=` | 全局搜索 |

`type` ∈ course / question / content / topic；缺省返回各分区 top 5 + 总数，指定类型时分页。可见性与业务口径一致：课程=已发布+挂载、题目=published、精选=已发布、帖子全量（物理删除即消失）。匹配为标题/内容 LIKE（不区分大小写）。

**GET /api/search?keyword=液压**（全部分区）响应 200：

```json
{ "code": 200, "message": "success", "data": { "keyword": "液压", "courses": { "items": [ { "type": "course", "id": 1, "title": "液压传动维修", "cover": "...", "summary": "液压系统原理与维修" } ], "total": 1 }, "questions": { "items": [ { "type": "question", "id": 5, "title": "液压油温过高的原因不包括…", "cover": "", "summary": "" } ], "total": 1 }, "contents": { "items": [], "total": 0 }, "topics": { "items": [], "total": 0 } } }
```

**GET /api/search?keyword=液压&type=course**（分页）响应 200：

```json
{ "code": 200, "message": "success", "data": { "keyword": "液压", "type": "course", "total": 1, "page": 1, "pages": 1, "items": [ { "type": "course", "id": 1, "title": "液压传动维修", "cover": "...", "summary": "..." } ] } }
```

## 14c. 学习资料 `/api/materials`（role=hrwai_user，ADR-0018）

资料 = 已发布课程下章节挂载的课件附件（chapter_file 聚合视图，不建独立资料库）；`file_url` 为静态直链可直接下载。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/materials?course_id=&page=&page_size=` | 资料列表（可按课程过滤） |
| GET | `/api/materials/:id` | 资料详情（含课程/章节回填） |
| GET | `/api/materials/:id/download` | 下载地址（`{ "file_url": "...", "file_name": "..." }`） |
| GET | `/api/student/materials` | 学员可访问资料（同列表，清单别名） |

**GET /api/materials** 响应 200：

```json
{ "code": 200, "message": "success", "data": { "page": 1, "pages": 1, "total": 1, "materials": [ { "file_id": 1, "chapter_id": 1, "chapter_title": "液压泵拆装", "course_id": 1, "course_name": "液压传动维修", "file_name": "液压手册.pdf", "file_url": "/static/uploads/chapters/h.pdf", "content_type": "document", "file_size": 1024, "created_at": "..." } ] } }
```

---

## 14d. 资料投稿 `/api/contributions`（role=hrwai_user，#517 / ADR-0026）

资料投稿（contribution）= 学员自主上传、经审核通过后对全学员公开免费下载的**非课程来源**资料，与 14c 学习资料（material，课程附件）严格区分。形态：一份投稿 1–5 个文件（白名单 pdf/doc/docx/ppt/pptx/xls/xlsx/zip/mp4；单文件 ≤20MB、合计 ≤50MB），必挂目标证件、跟随全局证件过滤器。生命周期 `pending → approved / rejected`；`pending → withdrawn`（作者撤回）；`approved → archived`（下架）。先审后发，**未过审不产生积分**。

积分：过审直记 +50（`contribution_approved`）；下载量每人每稿终身计一次为事实源，跨 10/50/200 档追加 +30/+80/+200（`contribution_tier`）；违规下架走 rollback 追回（封底 0）。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/contributions/upload-file` | 上传暂存文件（multipart `file`），返回 `{ file_url, file_name, file_size, content_type }` |
| POST | `/api/contributions` | 创建投稿（`credential_id/title/intro/is_anonymous/files[]`），返回 pending 条目 |
| GET | `/api/contributions?credential_id=&sort=latest\|hot&page=&page_size=` | 公开广场（仅 approved，按证件过滤） |
| GET | `/api/contributions/mine` | 我的投稿（全部状态，含驳回/下架原因） |
| GET | `/api/contributions/:id` | 详情（含文件清单；匿名投稿作者显示「匿名学员」） |
| POST | `/api/contributions/:id/download` | 下载计数（幂等：每人每稿终身一次，作者不计；跨档当场直记达阶奖励），返回 `{ is_new, tier_awarded }` |
| DELETE | `/api/contributions/:id` | 撤回 pending（→ withdrawn） |
| POST | `/api/contributions/:id/report` | 举报已上架投稿（`reason` ∈ piracy/content_error/violation/stale；同人同稿合并） |

管理端审核（`/api/admin/contributions`，role=tutor+admin；V1 仅管理端有 UI）：

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/contributions/pending` | 待审核队列（分页） |
| POST | `/api/admin/contributions/:id/approve` | 通过（直记 +50 + 站内信，同事务） |
| POST | `/api/admin/contributions/:id/reject` | 驳回（`reason` 必填） |
| POST | `/api/admin/contributions/:id/archive` | 下架（`reason` 必填；追回该稿累计投稿分） |
| GET | `/api/admin/contributions/reports?status=` | 举报队列（status 0 待处理 / 1 已处理） |
| POST | `/api/admin/contributions/reports/:id/handle` | 处置举报（`action` ∈ archive/dismiss） |

**POST /api/contributions** 请求体：

```json
{ "credential_id": 1, "title": "叉车液压故障排查手册", "intro": "整理自一线维修笔记", "is_anonymous": false, "files": [ { "file_url": "/static/uploads/contributions/a.pdf", "file_name": "a.pdf", "file_size": 10240, "content_type": "document" } ] }
```

**GET /api/contributions** 响应 200：

```json
{ "code": 200, "message": "success", "data": { "items": [ { "id": 1, "credential_id": 1, "title": "叉车液压故障排查手册", "intro": "...", "status": "approved", "is_anonymous": false, "downloads_count": 12, "files": [ { "file_id": 1, "file_name": "a.pdf", "file_url": "/static/uploads/contributions/a.pdf", "file_size": 10240, "content_type": "document" } ], "author": { "user_id": 7, "username": "小明", "anonymous": false }, "created_at": "2026-09-01 10:00:00" } ], "total": 1, "page": 1, "page_size": 20 } }
```

> 上传/文件访问注意：R2 模式下 `file_url` 为 `https://www.gccsmile.com/contributions/...` 公开直链，需 nginx 对象存储反代登记 `contributions`（与 `resumes`）顶层前缀（见 PR #530）；未登记会落进门户兜底被 Nuxt 报 404。

---

## 15. 通知 `/api/notifications`（JWT）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/notifications` | 通知列表（分页，含未读数） |
| GET | `/api/notifications/unread-count` | 未读数 |
| POST | `/api/notifications/:id/read` | 单条标记已读 |
| POST | `/api/notifications/read-all` | 全部标记已读 |

`type` 取值：`system`（系统）/ `profile_review`（资料审核，payload 含 review_status）/ `contribution_approved`（投稿过审）/ `contribution_rejected`（投稿驳回）/ `contribution_archived`（投稿下架，含追回分值）/ `contribution_tier`（投稿下载达阶）/ `forum_reply`（帖子被回复或楼中楼被回复）/ `forum_report`（举报处理结果）/ `forum_topic_deleted`（帖子被管理端删除）/ `forum_reply_deleted`（回复被管理端删除）。论坛类通知 `link` 为 `/training/forum/:topic_id`、payload 携带 `topic_id`（帖子已删除时无 link）。

**GET /api/notifications?page=1&page_size=10**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "items": [ { "id": 1, "type": "system", "title": "考试提醒", "content": "您有新的等级考试", "link": "/level-exam/1", "payload": {}, "is_read": false, "read_at": null, "created_at": "..." } ], "page": 1, "pages": 1, "total": 1, "unread_count": 3 } }
```

**GET /api/notifications/unread-count**：响应 200，data 为 `{ "count": 3 }`

**POST /api/notifications/:id/read**：请求体 `{}`，响应 200。**POST /api/notifications/read-all**：同。

---

## 16. 管理端 `/api/admin`（role=admin）

### 16.1 课程管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/courses` | 课程列表（分页；关键词/方向/等级过滤） |
| POST | `/api/admin/course` | 创建课程 |
| GET | `/api/admin/course/:course_id` | 课程详情（含章节） |
| PUT | `/api/admin/course/:course_id` | 更新课程 |
| PUT | `/api/admin/course/:course_id/sort` | 交换课程排序 |
| DELETE | `/api/admin/course/:course_id` | 删除课程 |
| POST | `/api/admin/course/:course_id/chapter` | 新增章节 |
| PUT | `/api/admin/chapter/:chapter_id` | 更新章节 |
| DELETE | `/api/admin/chapter/:chapter_id` | 删除章节 |
| POST | `/api/admin/course/generate-content` | 异步生成章节内容（AI） |
| GET | `/api/admin/course/generate-content/:task_id` | 查询生成任务状态 |

**GET /api/admin/courses?page=1&page_size=10&keyword=&specialty_id=&level_id=**

响应 200：data 为 `{ courses: [CourseDTO], page, pages, total }`

**POST /api/admin/course**

请求体：

```json
{ "name": "叉车液压系统", "description": "...", "cover_image": "/static/uploads/covers/xx.jpg", "duration": 120, "status": 1, "sort_order": 1, "specialty_id": 1, "level_id": 2, "certificate_template_id": 1, "theory_hours": 20, "practice_hours": 10, "prerequisite_course_ids": [] }
```

响应 200：data 为 CourseDTO。

**PUT /api/admin/course/:course_id**：同上请求体（全量或部分字段）。**PUT /api/admin/course/:course_id/sort**：请求体 `{ "swap_with": 2 }`。

**POST /api/admin/course/:course_id/chapter**

请求体：`{ "title": "第一章", "content": "markdown", "duration": 40, "order_num": 1, "description": "" }`

**POST /api/admin/course/generate-content**

请求体：`{ "course_id": 1, "chapter_ids": [1, 2] }`。响应 200：data 为 `{ "task_id": "uuid" }`

**GET /api/admin/course/generate-content/:task_id**

响应 200：data 为 `{ "task_id": "...", "status": "pending|running|completed|failed", "total": 2, "completed": 1, "results": [ { "chapter_id": 1, "title": "...", "status": "completed", "content": "...", "error": "" } ] }`

### 16.2 培训目录（专业方向 / 等级 / 证书模板 / 题库标签）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/catalog/tree` | 管理端目录树（含停用项） |
| GET | `/api/admin/specialties` | 专业方向列表 |
| POST | `/api/admin/specialty` | 创建专业方向 |
| PUT | `/api/admin/specialty/:specialty_id` | 更新专业方向 |
| PUT | `/api/admin/specialty/:specialty_id/sort` | 交换方向排序 |
| DELETE | `/api/admin/specialty/:specialty_id` | 删除专业方向 |
| GET | `/api/admin/levels` | 课程等级列表 |
| POST | `/api/admin/level` | 创建课程等级 |
| PUT | `/api/admin/level/:level_id` | 更新课程等级 |
| PUT | `/api/admin/level/:level_id/sort` | 交换等级排序 |
| DELETE | `/api/admin/level/:level_id` | 删除课程等级 |
| GET | `/api/admin/certificate-templates` | 证书模板列表 |
| POST | `/api/admin/certificate-template` | 创建证书模板 |
| PUT | `/api/admin/certificate-template/:id` | 更新证书模板 |
| DELETE | `/api/admin/certificate-template/:id` | 删除证书模板 |
| GET | `/api/admin/question-tags` | 题库标签列表（含题目数） |
| POST | `/api/admin/question-tag` | 创建题库标签 |
| PUT | `/api/admin/question-tag/:id` | 更新题库标签 |
| DELETE | `/api/admin/question-tag/:id` | 删除题库标签 |
| PUT | `/api/admin/question/:question_id/tags` | 全量替换题目标签 |

**POST /api/admin/specialty**：请求体 `{ "code": "xx", "name": "内燃叉车", "description": "", "sort_order": 1, "status": 1 }`

**POST /api/admin/level**：同上（level_id 由系统生成）。

**POST /api/admin/certificate-template**：请求体 `{ "code": "CT01", "name": "结业证书", "description": "", "template_url": "", "validity_days": 365, "status": 1 }`（validity_days 有效期天数）

**POST /api/admin/question-tag**：请求体 `{ "code": "T01", "name": "基础", "description": "", "sort_order": 1, "status": 1 }`

**PUT /api/admin/question/:question_id/tags**：请求体 `{ "tag_ids": [1, 2] }`

### 16.3 用户管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/hrwai-users` | 学员列表（分页+关键词） |
| POST | `/api/admin/hrwai-users` | 创建学员 |
| PUT | `/api/admin/hrwai-users/:id` | 更新学员 |
| PUT | `/api/admin/hrwai-users/:id/password` | 重置密码 |
| PUT | `/api/admin/hrwai-users/:id/status` | 启用/禁用 |
| DELETE | `/api/admin/hrwai-users/:id` | 删除学员 |
| GET | `/api/admin/tutors` | 讲师列表（分页+关键词） |
| POST | `/api/admin/tutor` | 创建讲师 |
| DELETE | `/api/admin/tutor/:tutor_id` | 删除讲师 |
| PUT | `/api/admin/tutor/:tutor_id/password` | 重置讲师密码 |
| PUT | `/api/admin/tutor/:tutor_id/status` | 启用/禁用讲师 |

**GET /api/admin/hrwai-users?page=1&page_size=10&keyword=**

响应 200：data 为 `{ list: [ { id, uid, account, username, phone, email, company, status, created_at } ], page, page_size, total }`

**POST /api/admin/hrwai-users**：请求体 `{ "phone": "13800000001", "password": "123456", "account": "", "username": "13800000001", "email": "", "company": "" }`

**POST /api/admin/tutor**：请求体 `{ "username": "tutor01", "password": "123456", "name": "李讲师" }`

### 16.4 统计与审核

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/statistics` | 运营统计（总览 + 课程学习排行） |
| GET | `/api/admin/profile-reviews` | 资料审核列表（昵称/头像变更待审） |
| POST | `/api/admin/profile-reviews/:id/approve` | 通过审核 |
| POST | `/api/admin/profile-reviews/:id/reject` | 拒绝审核 |
| GET | `/api/admin/audit-logs` | 操作审计日志（管理员/讲师写操作） |

**GET /api/admin/statistics**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "overview": { "total_students": 100, "active_today": 20, "total_courses": 6, "total_study_duration": 3600 }, "course_stats": [ { "course_id": 1, "name": "叉车基本构造", "study_count": 50, "total_duration": 3000, "avg_progress": 45 } ] } }
```

**GET /api/admin/profile-reviews?page=&page_size=&status=**

响应 200：data 为 `{ page, pages, requests: [ { id, user_id, username, avatar_url, field_type, old_value, new_value, status, reject_reason, reviewed_by, reviewed_at, created_at } ], total }`

**POST /api/admin/profile-reviews/:id/approve** / **reject**：reject 请求体 `{ "reason": "不通过原因" }`。

### 16.5 AI 配置管理

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

**GET /api/admin/ai-configs**

响应 200（data 为数组）：`[ { "id": 1, "name": "DeepSeek 主配置", "api_key": "sk-***", "base_url": "...", "model": "deepseek-chat", "description": "", "is_active": true, "created_at": "...", "updated_at": "..." } ]`

**POST /api/admin/ai-configs**：请求体 `{ "name": "DeepSeek 主配置", "api_key": "sk-xxx", "base_url": "https://api.deepseek.com", "model": "deepseek-chat", "description": "", "is_active": true }`

**GET /api/admin/ai-feature-bindings**

响应 200（data 为数组）：`[ { "feature_key": "ai_assistant", "feature_label": "AI 助手", "is_multi": false, "config_id": 1, "config_name": "DeepSeek 主配置", "bound_configs": [ { "config_id": 1, "config_name": "...", "model": "..." } ] } ]`

### 16.6 内容精选管理

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/featured-contents` | 管理端列表（含草稿；分页+分类+状态过滤） |
| POST | `/api/admin/featured-content` | 新增 |
| GET | `/api/admin/featured-content/:id` | 管理端详情 |
| PUT | `/api/admin/featured-content/:id` | 更新 |
| DELETE | `/api/admin/featured-content/:id` | 删除 |
| POST | `/api/admin/featured-content/:id/publish` | 发布/下线 |
| POST | `/api/admin/featured-content/upload-image` | 上传封面图 |

**POST /api/admin/featured-content**

请求体：`{ "title": "标题", "category": "news", "summary": "摘要", "cover_image": "/static/uploads/...", "content": "正文 HTML/MD", "source": "官网", "status": "draft", "sort_order": 1 }`

**POST /api/admin/featured-content/:id/publish**：请求体 `{}`，发布/下线切换。

### 16.7 数据导出

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/admin/export/students` | 导出学员名单 CSV |
| GET | `/api/admin/export/exam-records` | 导出成绩单 CSV |
| GET | `/api/admin/export/questions` | 导出题库 CSV |
| GET | `/api/admin/export/evaluations` | 导出评估记录 CSV |

响应：`text/csv` 附件下载。

---

## 16.8 问答与简历、招聘（#364-#375，ADR-0022）

### 16.8.1 论坛类别与问答筛选

论坛新增 `category`（`discussion` | `question`，空串归一 `discussion`）与问答筛选 `solved`（`all` | `solved` 已解决 | `unsolved` 求助，仅对 `question` 有意义）；`question` 帖一律 `chapter_id=NULL`（带 `chapter_id>0` 返回 400，`CHECK` 兜底）。

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/forum/topics?category=&solved=&scope=&chapter_id=&keyword=&sort=&order=&page=&page_size=` | JWT+hrwai_user | 帖子列表（`scope=all|general|chapter` 与 `category` 必须共存在同一条 WHERE，否则问答帖灌进讨论 Tab） |
| POST | `/api/forum/topics` | JWT+hrwai_user | 发帖（`{title, content, category, chapter_id?, images[]}`，`category=question` 时不得带 `chapter_id>0`） |
| GET | `/api/forum/topics/:id` | JWT+hrwai_user | 详情（含 `accepted_reply_id`/`solved_at`/`reward_issued` 与每条回复 `is_accepted`） |
| POST | `/api/forum/topics/:id/accept` | JWT+hrwai_user（仅楼主） | 采纳回答 `{reply_id}`（首次采纳才发分，见积分） |
| DELETE | `/api/forum/topics/:id/accept` | JWT+hrwai_user（仅楼主） | 取消采纳（已发分不回滚） |

`GET /api/forum/topics` 响应与既有同形，元素新增 `category`/`accepted_reply_id`/`solved_at`/`reward_issued`；`solved` 非法返回 400。

### 16.8.2 学员侧简历卡 `/api/resume`（role=hrwai_user）

常驻实体 `job_cards`（`user_id` 主键，`visibility` 默认 `hidden`）。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/resume` | 我的简历（未建时 404） |
| PUT | `/api/resume` | 整页保存（`real_name`/`contact_phone`/`wechat`/`region`/`expected_specialty_id`/`expected_regions[]`/`salary_min/max`/`salary_negotiable`/`available_in`/`job_nature`/`experience_years`/`self_intro`/`resume_experiences[]`/`resume_certifications[]`/`resume_file_url`/`photos[]`） |
| PUT | `/api/resume/visibility` | 切换公开 `{visibility: hidden|open}` |
| POST | `/api/resume/pdf` | 上传 PDF 简历（`file`，仅 PDF ≤50MB） |
| POST | `/api/resume/image` | 上传工作照（`file`，图片 ≤20MB，≤6 张） |
| GET | `/api/resume/view-stats` | 近 7 天查看过我的企业数 `{count}`（按企业去重，不含企业名） |

### 16.8.3 招聘端 `/api/recruit`（role=recruiter，`recruit.` 子域 + `recruiter_token` host-only）

三层漏斗：L1 未登录 401/403 无公开列表；L2 脱敏卡（唯一脱敏路径）；L3 交换后明文。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/auth/recruiter-login` | 企业招聘者登录（`{username, password}` → `role=recruiter`，写 `recruiter_token` host-only） |
| GET | `/api/recruit/me` | 当前招聘者信息 |
| GET | `/api/recruit/resumes?region=&specialty_id=&credential_id=&salary_min=&salary_max=&experience_years=&available_in=&page=&page_size=` | 脱敏简历列表（仅 `visibility=open`，默认 `updated_at DESC`，无缓存；读取后审计入 `recruit_resume_views`） |
| GET | `/api/recruit/resumes/:id` | 脱敏详情（与列表同一脱敏路径；`hidden` 时 404；同样审计） |
| POST | `/api/recruit/contact-requests` | 发起交换申请 `{student_user_id, message(1-200)}`（`pending` 唯一、30 天冷却、日限 20） |
| GET | `/api/recruit/contact-requests` | 我的申请列表（`pending/approved/rejected/expired/revoked`） |
| GET | `/api/recruit/resumes/:id/contact` | 明文联系方式与 PDF（仅 `approved` 且未 `revoked`/`expired` 时可用，实时校验，无缓存；L2 脱敏口径在授权后仍保持） |

脱敏边界（字段级负向断言）：响应体不含 `contact_phone`/`wechat`/`resume_file_url`/`image_urls`/`未打码 real_name`/`region 精确值`；`real_name` 返回打码值（`real_name`/`real_name_masked`），`resume_certifications` 已去 `image_urls`。

### 16.8.4 联系方式交换学员侧 `/api/resume/contact-requests`（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/resume/contact-requests` | 收到的申请列表（含 `company_name`/`contact_name`/`message`，不含企业电话） |
| POST | `/api/resume/contact-requests/:id/approve` | 同意（状态 `approved`，招聘方收邮件） |
| POST | `/api/resume/contact-requests/:id/reject` | 拒绝（`rejected`，30 天冷却） |
| POST | `/api/resume/contact-requests/:id/revoke` | 撤回已同意（`revoked`，实时生效） |

`contact_requests` 状态机：`pending` 14 天后 `expired`（`daemon.Runner` 托管，`interval` 1h，`jitter` 错峰，`panic` 恢复，`context` 贯穿）、`approved` 永久有效直至 `revoked`。学员注销时 `ON DELETE CASCADE` 一并失效。

### 16.8.5 管理端巡检（role=admin）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/admin/recruiters` | 创建企业招聘者（邀约制，企业信息全必填） |
| PUT | `/api/admin/recruiters/:id/status` | 切换启用/禁用（禁用后登录 400） |
| GET | `/api/admin/recruiters` | 列表（简易） |
| GET | `/api/admin/points/ledger?reason=&user_id=&page=&page_size=` | 全量流水按原因筛选（`accepted_bonus`/`accept_action`/`rollback` 等，可定位 `ref_id=topic_id`） |
| GET | `/api/admin/inspection/deleted-after-accepted` | 巡检计数 `{count}`（楼主删除已解决帖累加） |
| GET | `/api/admin/recruit/views?recruiter_id=&student_user_id=&page=&page_size=` | 查看招聘查看留痕 |
| GET | `/api/admin/recruit/requests?recruiter_id=&student_user_id=&status=&page=&page_size=` | 查看交换申请记录 |
| DELETE | `/api/admin/forum/topics/:id` | 管理员删帖（若曾发分则按 `rollback` 对冲扣减，封底 0，幂等） |
| DELETE | `/api/admin/forum/replies/:id` | 管理员删回复（若为被采纳回答则同上回收） |
| PUT | `/api/admin/forum/reports/:id` | 处理举报（`{status:1}` 标记已处理） |

积分流水 `reason` 取值：`accepted_bonus`（答主 +40）、`accept_action`（楼主 +5）、`rollback`（违规回收，对冲）、`admin_penalty` 等既有值；学员侧 `GET /api/points/ledger` 按人返回，管理端 `GET /api/admin/points/ledger` 可按 `reason` 全局筛选。

### 16.8.6 职位发布与职位广场（spec #449 T2 #451）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/recruit/jobs` | recruiter | 发布职位 `{title, specialty_id(必填), region?, salary_min?, salary_max?, salary_text?, experience_req?, description?}`（活跃职位上限 50） |
| PUT | `/api/recruit/jobs/:id` | recruiter | 编辑自己的职位（改别人的 403） |
| POST | `/api/recruit/jobs/:id/toggle-status` | recruiter | 上架/下架（`open`↔`closed`；被强制下架的职位不能自行重新上架） |
| GET | `/api/recruit/jobs?page=&page_size=&specialty_id=` | recruiter | 我的职位列表（含 closed/强制下架历史） |
| GET | `/api/recruit/jobs/:id` | recruiter | 我的职位详情 |
| GET | `/api/jobs?specialty_id=&region=&salary_min=&salary_max=&experience=&page=&page_size=` | hrwai_user | 职位广场（仅 `open` 且未强制下架，按 `published_at` 新鲜度排序） |
| GET | `/api/jobs/:id` | hrwai_user | 职位详情（含企业名/主营/联系人；**不含**电话/邮箱/信用代码） |

**L1 延伸**：无 token 访问职位列表/详情一律 401（职位不进入任何未登录面，无 SEO）。

### 16.8.7 投递即授权（spec #449 T3 #452）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/jobs/:id/apply` | hrwai_user | 投递职位（同事务写投递 + 写/复活 `approved` 授权 `source=application`；投递后该企业明文端点当场放行；`hidden` 简历可投递；缺真实姓名/电话 400；applied 唯一；30 天冷却；日限 10） |
| GET | `/api/resume/applications?page=&page_size=` | hrwai_user | 我的投递（含 `job_title`/`company_name`/`employer_viewed_at`） |
| POST | `/api/resume/applications/:id/withdraw` | hrwai_user | 撤回投递 `{revoke_contact?: boolean}`（默认 false 不连带收回授权；true 则授权置 `revoked`，明文端点 403） |

**投递状态轴**：`applied → rejected | withdrawn`。`rejected` 终态（企业标记不合适，30 天冷却）；`withdrawn` 后可立即重投同一职位。

**授权来源轴（`contact_requests.source`）**：`recruiter`（企业发起交换申请）/ `application`（投递产生）。明文载体唯一（GetContact 查 `approved` 状态，与来源无关）；投递产生的授权不计入企业日限、不受冷却限制；撤回投递默认不动授权。

### 16.8.8 企业处理投递与内容治理（spec #449 T4 #453 / T5 #454）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/recruit/jobs/:id/applications?page=&page_size=` | recruiter | 按职位查看投递（只能看自己的职位，越权 403；含 `unread_count` 未读投递数；候选人脱敏——姓名打码、无手机/微信/PDF/证书原图） |
| GET | `/api/recruit/applications/:id` | recruiter | 投递详情（打开即记录已读 `employer_viewed_at`；回显投递时刻简历更新时间，内容读最新不落快照） |
| POST | `/api/recruit/applications/:id/reject` | recruiter | 标记不合适 → `rejected`（仅 applied 可拒；30 天冷却） |
| POST | `/api/jobs/:id/report` | hrwai_user | 举报职位 `{reason}`（同一学员对同一职位唯一，重复举报合并） |
| GET | `/api/admin/jobs?recruiter_id=&page=&page_size=` | admin | 职位巡检（全量含 closed/强制下架，可按企业筛） |
| GET | `/api/admin/job-reports?page=&page_size=` | admin | 待处理举报队列 |
| POST | `/api/admin/job-reports/:id/handle` | admin | 标记举报已处理 |
| POST | `/api/admin/jobs/:id/force-offline` | admin | 带原因强制下架 `{reason}`（学员侧立即不可见；企业不能自行重新上架；处置入审计日志、邮件通知企业） |

---

## 17. 残值评估子模块 `/api/valuation`

> 独立连接池（pgx）+ 独立响应格式（同样 `{code, message, data}` 信封）；鉴权分三档：公开 / 可选认证（登录则记录 user_id）/ hrwai_user JWT / admin JWT。

### 17.1 公开（无需登录）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/valuation/evaluations/stats` | 评估统计 |
| GET | `/api/valuation/health` | 子模块健康检查 |
| POST | `/api/valuation/auth/login` | 估值模块登录（兼容主体系） |
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

字典接口响应 200：data 为数组或对象（各字典结构以具体实现为准，字段见管理端 CRUD 对应字典描述）。

### 17.2 可选认证（登录则记录 user_id）

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/valuation/evaluations` | 提交评估（未登录匿名提交） |
| POST | `/api/valuation/battery/evaluations` | 创建电池 RUL 评估（未登录匿名提交） |

**POST /api/valuation/evaluations**

请求体：

```json
{ "brand": "合力", "vehicle_type": "平衡重式", "series": "H 系列", "tonnage": 3.0, "config_type": "标准", "mast_type": "二级门架", "mast_height_mm": 3000, "factory_year": 2018, "sale_year": 2024, "usage_hours": 4200, "original_paint": true, "province": "广东省", "city": "广州市", "has_license_plate": true, "has_registration_certificate": true, "has_maintenance_records": false, "condition_rating": "B" }
```

响应 200：data 为评估结果（ID + 输入参数 + 全部 K 系数 + 残值 + 置信区间 + 维度评分 + 建议）。

**POST /api/valuation/battery/evaluations**

请求体：

```json
{ "battery_type": "lfp", "battery_model": "48V300Ah", "cycles": [ { "cycle_index": 1, "voltage_series": [3.2, 3.3], "current_series": [0.5, 0.5], "capacity": 295.5 } ] }
```

`cycles` 至少 10 条；`voltage_series` 与 `current_series` 长度一致。

### 17.3 HRWAI 账号鉴权（JWT + role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/valuation/evaluations` | 我的评估历史 |
| GET | `/api/valuation/evaluations/:id` | 评估详情（仅本人） |
| GET | `/api/valuation/battery/evaluations` | 电池评估列表 |
| GET | `/api/valuation/battery/evaluations/:id` | 电池评估详情 |
| GET | `/api/valuation/auth/me` | 当前估值用户 |
| POST | `/api/valuation/auth/logout` | 估值登出 |

### 17.4 管理员 CRUD（JWT + role=admin）

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

## 18. 静态资源

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/HEAD | `/static/uploads/<path>` | 上传文件（章节课件、视频、图片、PDF 报告等） |
| GET/HEAD | `/static/<path>` | 其他静态资源 |

- 支持 GET 与 HEAD（前端文档/图片预览用 HEAD 探测存在性）
- 包含 `..` 的路径会被拒绝（防路径穿越）

---

## 19. 变更记录（相对历史版本）

- **移除**：`/api/exam/*`（课程考核，路由从未注册；课程考核由等级考试/模拟考试体系替代）
- **移除**：`/api/question-bank/categories`、`/api/question-bank/knowledge-points` 及知识点 CRUD（已下线，契约测试锁定 404）
- **移除**：`/api/practice-mode/category`、`/api/practice-mode/knowledge-point`、`/api/practice-mode/knowledge-point-progress`（已下线）
- **新增补录**：`/api/captcha`、`/api/auth/{email|phone}/reset-password`、`/api/featured-content/:id/view`、`/api/forum/upload-image`、`PUT /api/admin/course/:course_id/sort`、`PUT /api/admin/specialty/:specialty_id/sort`、`PUT /api/admin/level/:level_id/sort`、`/api/admin/profile-reviews`、`/api/admin/audit-logs`
- **修正**：`/api/valuation/battery/evaluations` 为可选认证（非强制 JWT）；AI 会话列表返回 `{sessions: []}`；论坛列表返回 `{topics: []}`
- 全部端点补充请求格式（path/query/body）与返回格式（JSON 示例）
- **新增（ADR-0018）**：论坛互动（like/report/my-topics/my-replies + 管理端举报）、通用收藏 `/api/favorites`、全局搜索 `/api/search`、学习资料 `/api/materials`
- **新增（ADR-0022，#364-#376）**：论坛类别 `category` + 问答筛选 `solved` + 采纳 `accept`、学员侧简历 `/api/resume`（含 `view-stats`）、招聘端 `/api/recruit/*`（脱敏列表/详情、`/contact-requests`、`/resumes/:id/contact` 明文）、学员侧交换 `/api/resume/contact-requests/*`、管理端 `recruiters`/`points/ledger`/`inspection`/`recruit/views|requests` 巡检；积分流水 `reason` 新增 `accepted_bonus`/`accept_action`/`rollback`；`recruit.` 子域复用 catch-all（DNS + SAN 为主要运维工作，无新增 server block）
- **新增（#517，ADR-0026）**：资料投稿域 `/api/contributions`（学员：upload-file/create/list/mine/detail/download/withdraw/report）+ 管理端 `/api/admin/contributions/*`（pending/approve/reject/archive/reports/handle）；积分流水 `reason` 新增 `contribution_approved`/`contribution_tier`，站内信 `type` 新增投稿四种；material（课程附件）与 contribution（学员投稿）两词分域
- **未覆盖**：uni-app `training-app` 端契约漂移（ADR-0019）本次未覆盖，继续挂账
