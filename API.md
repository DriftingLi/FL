# API 接口清单

本文档为叉车维修培训系统 + 残值评估子系统 + AI 助手的 HTTP 接口总清单（路径 / 方法 / 鉴权 / 请求格式 / 返回格式）。

> **准确性基准**：本清单以 `backend/internal/api` 实际注册的路由与 `backend/internal/service` 的 typed DTO 契约为准（2026-08-19，6c0dfec）。与历史文档的差异（已下线端点等）见文末「变更记录」。

## 0. 通用约定

- 基础路径：`/api`；静态资源：`/static/*`
- 鉴权方式：`Authorization: Bearer <access JWT>`（access 2h 过期，用 `POST /api/auth/refresh` 以 refresh token 换新双令牌，见 ADR-0016）；部分接口同时写入登录 Cookie（HttpOnly，仅携带 access）
- 角色：`admin`（管理员）/ `tutor`（讲师）/ `hrwai_user`（学员/普通用户）
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
| PUT | `/api/auth/profile` | JWT | 修改昵称（走资料审核流：提交 → 审核 → 生效） |
| POST | `/api/auth/avatar` | JWT | 上传头像（multipart，字段名 `file`；同样走资料审核流） |

**PUT /api/auth/profile**

请求体：`{ "nickname": "新昵称" }`

响应 200（返回待审核的资料修改请求）：

```json
{ "code": 200, "message": "修改申请已提交，等待审核", "data": { "id": 1, "user_id": 1, "username": "13800000001", "avatar_url": "", "field_type": "nickname", "old_value": "旧昵称", "new_value": "新昵称", "status": "pending", "reject_reason": "", "created_at": "2026-08-16 10:00:00" } }
```

**POST /api/auth/avatar**

multipart/form-data：`file`（图片）。响应 200：`data` 为头像修改审核请求（同上结构，`field_type: "avatar"`）。

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

### 2.5 微信扫码登录 `/api/auth/wechat`（框架占位）

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| POST | `/api/auth/wechat/qrcode` | 无 | 获取登录二维码 |
| POST | `/api/auth/wechat/login` | 无 | 微信扫码登录 |

### 2.6 账号绑定修改（短信/邮箱验证码确认）

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
| GET | `/api/specialties` | 无 | 专业方向列表（仅启用项） |
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

**GET /api/specialties**

响应 200（data 为数组）：`{ "code": 200, "message": "success", "data": [ { "specialty_id": 1, "code": "xx", "name": "内燃叉车", "description": "", "sort_order": 1, "status": 1, "created_at": "..." } ] }`

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
{ "code": 200, "message": "success", "data": { "is_correct": true, "correct_answer": "A", "explanation": "解析", "question_id": 1, "user_answer": "A" } }
```

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

---

## 7. 等级考试 `/api/level-exam`

### 7.1 场次管理

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/level-exam/sessions` | JWT | 场次列表（分页；可按状态过滤、选择是否带学员参与信息） |
| POST | `/api/level-exam/sessions` | JWT+admin | 创建场次 |
| GET | `/api/level-exam/sessions/:session_id` | JWT | 场次详情 |
| PUT | `/api/level-exam/sessions/:session_id` | JWT+admin | 更新场次 |
| PUT | `/api/level-exam/sessions/:session_id/status` | JWT+admin | 更新场次状态（upcoming/ongoing/finished） |
| DELETE | `/api/level-exam/sessions/:session_id` | JWT+admin | 删除场次 |

**GET /api/level-exam/sessions?page=1&page_size=10&status=&include_participants=**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "total": 1, "page": 1, "page_size": 10, "sessions": [ { "id": 1, "name": "2026年8月定级考试", "start_time": "2026-08-20 09:00:00", "end_time": "2026-08-20 11:00:00", "duration": 90, "status": "upcoming", "created_by": 1, "question_config": { "single_choice": 20, "multi_choice": 10, "true_false": 10, "short_answer": 2, "fault_image": 2 }, "total_score": 100, "pass_score": 60, "created_at": "...", "updated_at": "..." } ] } }
```

**POST /api/level-exam/sessions**

请求体：`{ "name": "2026年8月定级考试", "start_time": "2026-08-20 09:00:00", "end_time": "2026-08-20 11:00:00" }`

响应 200：data 为场次详情（同 sessions 元素）。

### 7.2 学员考试流程（role=hrwai_user）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/level-exam/available` | 可参加的考试（含个人参与状态） |
| GET | `/api/level-exam/history` | 考试历史（分页） |
| POST | `/api/level-exam/sessions/:session_id/enter` | 进入考试（抽题，返回题目与剩余时间） |
| POST | `/api/level-exam/participants/:participant_id/save` | 保存作答 |
| POST | `/api/level-exam/participants/:participant_id/submit` | 交卷（支持超时交卷） |
| GET | `/api/level-exam/participants/:participant_id/result` | 考试成绩（含逐题明细） |

**GET /api/level-exam/available**

响应 200（data 为数组）：

```json
{ "code": 200, "message": "success", "data": [ { "id": 1, "name": "2026年8月定级考试", "start_time": "...", "end_time": "...", "duration": 90, "status": "ongoing", "created_by": 1, "question_config": { ... }, "total_score": 100, "pass_score": 60, "created_at": "...", "updated_at": "...", "has_participated": true, "participant_status": "in_progress", "participant_id": 3, "can_enter": true } ] }
```

**GET /api/level-exam/history?page=1&page_size=10**

响应 200：data 为 `{ total, page, page_size, records: [ { id, exam_session_id, session_name, student_id, status, start_time, submit_time, remaining_time, answers_snapshot, question_ids, created_at, score, objective_score, subjective_score, is_passed } ] }`

**POST /api/level-exam/sessions/:session_id/enter**

请求体：`{}`。响应 200：

```json
{ "code": 200, "message": "success", "data": { "participant_id": 3, "session": { "id": 1, "name": "..." }, "questions": [ { "id": 1, "content": "...", "options": [ { "key": "A", "text": "..." } ], "type": "single_choice", "score": 3 } ], "answers": { "1": "A" }, "remaining_time": 5400, "start_time": "2026-08-16 10:00:00" } }
```

**POST /api/level-exam/participants/:participant_id/save**

请求体：`{ "answers": { "1": "A" }, "remaining_time": 5000 }`

**POST /api/level-exam/participants/:participant_id/submit**

请求体：`{ "answers": { "1": "A" }, "is_timeout": false, "remaining_time": 4500 }`（超时自动交卷时 `is_timeout: true`）

**GET /api/level-exam/participants/:participant_id/result**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "participant": { "id": 3, "exam_session_id": 1, "student_id": 1, "status": "submitted", "start_time": "...", "submit_time": "...", "remaining_time": 0, "answers_snapshot": { "1": "A" }, "question_ids": [1, 2], "created_at": "...", "score": 85, "objective_score": 60, "subjective_score": 25, "is_passed": true }, "answers": [ { "id": 10, "exam_participant_id": 3, "question_id": 1, "user_answer": "A", "score": 3, "grading_comment": "", "ai_comment": "", "is_correct": true, "grader_id": null, "graded_at": null, "ai_score": null, "ai_graded_at": null, "question": { "id": 1, "content": "...", "options": [...], "type": "single_choice", "score": 3 } } ] } }
```

---

## 8. 模拟考试 `/api/mock-exam`（role=hrwai_user）

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

## 9. 阅卷评分 `/api/grading`（role=tutor/admin）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/grading/participants` | 阅卷列表（可 ?session_id= 过滤） |
| GET | `/api/grading/participants/:participant_id` | 阅卷详情 |
| POST | `/api/grading/participants/:participant_id/confirm-objective` | 确认客观题成绩 |
| POST | `/api/grading/:answer_id/grade` | 人工评分（简答题） |
| POST | `/api/grading/:answer_id/regrade` | 重新评分 |
| POST | `/api/grading/:answer_id/confirm-ai` | 确认 AI 评分 |
| POST | `/api/grading/:answer_id/ai-grade` | 触发 AI 评分 |
| GET | `/api/grading/stats` | 阅卷统计（按天，?session_id= 可选） |

**GET /api/grading/participants**

响应 200（data 为数组）：

```json
{ "code": 200, "message": "success", "data": [ { "id": 3, "exam_session_id": 1, "student_id": 1, "status": "submitted", "start_time": "...", "submit_time": "...", "remaining_time": 0, "answers_snapshot": {}, "question_ids": [1, 2], "created_at": "...", "score": 0, "objective_score": 60, "subjective_score": 0, "is_passed": false, "session_name": "2026年8月定级考试", "student_name": "张三", "pass_score": 60, "ungraded_count": 2, "objective_ungraded": 0, "subjective_ungraded": 2, "total_answers": 32, "grading_status": "pending" } ] }
```

**GET /api/grading/participants/:participant_id**

响应 200：data 为阅卷详情（participant 字段 + `answers`: [LevelExamAnswerDTO]）。

**POST /api/grading/participants/:participant_id/confirm-objective**：请求体 `{}`

**POST /api/grading/:answer_id/grade**：请求体 `{ "score": 4, "comment": "要点齐全" }`

**POST /api/grading/:answer_id/ai-grade**：请求体 `{}`。响应 200：data 为 `{ "score": 4, "comment": "...", "fallback": false }`（AI 不可用时 fallback: true）

---

## 10. 讲师端 `/api/tutor`（role=tutor）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/tutor/courses` | 讲师课程列表（分页；可按方向/等级过滤） |
| GET | `/api/tutor/grading-stats` | 讲师阅卷统计（按天分组，days=7|30） |
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
| GET | `/api/wrong-questions` | 错题列表（分页+过滤） |
| POST | `/api/wrong-questions/:question_id/redo` | 重做错题（提交答案判分） |
| POST | `/api/wrong-questions/:question_id/remove` | 移出错题本 |
| GET | `/api/wrong-questions/stats` | 错题统计 |
| GET | `/api/wrong-questions/export` | 导出错题本（纯文本附件） |

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
| GET | `/api/featured-contents` | 无 | 内容精选列表（仅已发布；分页+分类过滤） |
| GET | `/api/featured-content/:id` | 无 | 内容详情（含相关资讯/上下一篇） |
| POST | `/api/featured-content/:id/view` | 无 | 浏览计数 +1 |

**GET /api/featured-contents?page=1&page_size=10&category=**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "items": [ { "content_id": 1, "title": "叉车维保小知识", "category": "news", "category_label": "行业资讯", "summary": "摘要", "cover_image": "/static/uploads/...", "source": "官网", "status": "published", "sort_order": 1, "view_count": 120, "published_at": "...", "created_at": "...", "updated_at": "..." } ], "page": 1, "pages": 1, "total": 1 } }
```

**GET /api/featured-content/:id**

响应 200：data 为列表项 + `content`（正文）、`related`（相关资讯数组）、`prev`/`next`（上/下一篇导航，null 表示无）。

**POST /api/featured-content/:id/view**：请求体 `{}`，响应 200 data 为 `{ content_id, view_count }`。

---

## 13. AI 助手 `/api/ai-assistant`

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/api/ai-assistant/models` | 无 | 可用模型列表（管理员绑定 AI 助手功能的配置） |
| POST | `/api/ai-assistant/chat` | 可选 JWT | 流式对话（SSE；登录可保存会话） |
| GET | `/api/ai-assistant/sessions` | JWT+hrwai_user | 会话列表 |
| POST | `/api/ai-assistant/sessions` | JWT+hrwai_user | 创建会话 |
| PATCH | `/api/ai-assistant/sessions/:id/title` | JWT+hrwai_user | 重命名会话 |
| DELETE | `/api/ai-assistant/sessions/:id` | JWT+hrwai_user | 删除会话 |
| GET | `/api/ai-assistant/sessions/:id/messages` | JWT+hrwai_user | 会话消息列表 |
| GET | `/api/ai-assistant/user-models` | JWT+hrwai_user | 用户自定义模型列表（api_key 脱敏） |
| POST | `/api/ai-assistant/user-models` | JWT+hrwai_user | 保存自定义模型 |
| DELETE | `/api/ai-assistant/user-models/:id` | JWT+hrwai_user | 删除自定义模型 |

**GET /api/ai-assistant/models**

响应 200：

```json
{ "code": 200, "message": "success", "data": { "models": [ { "id": 1, "name": "DeepSeek V3", "model": "deepseek-chat", "base_url": "https://api.deepseek.com" } ] } }
```

**POST /api/ai-assistant/chat**（SSE 流式，非 JSON 信封）

请求体：

```json
{ "session_id": 1, "model_source": "admin", "config_id": 1, "user_model_id": 0, "custom_api_key": "", "custom_base_url": "", "custom_model": "", "messages": [ { "role": "user", "content": "叉车液压系统常见故障？" } ] }
```

`model_source`：`admin`（用 `config_id` 指定管理员配置）| `user`（用 `user_model_id`）| `custom`（临时传 `custom_*`）。`session_id` 可选（登录用户指定会话；不传且已登录则新建）。

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
| GET | `/api/forum/topics` | 帖子列表（scope=all|general|chapter；keyword 搜索；分页） |
| POST | `/api/forum/topics` | 发帖（images 最多 9 张） |
| GET | `/api/forum/topics/:id` | 帖子详情（含回复） |
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

## 15. 通知 `/api/notifications`（JWT）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/notifications` | 通知列表（分页，含未读数） |
| GET | `/api/notifications/unread-count` | 未读数 |
| POST | `/api/notifications/:id/read` | 单条标记已读 |
| POST | `/api/notifications/read-all` | 全部标记已读 |

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
