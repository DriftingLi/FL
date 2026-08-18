# 叉车维修培训系统 · 学员端 API 文档

> 基础 URL：`https://www.gccsmile.com`
> 认证方式：`Authorization: Bearer <token>`（登录后获取）
> 响应格式：`{"code":200,"message":"success","data":...}`

---

## 1. 认证模块

### 1.1 学员注册 `POST /api/auth/register`

> 公开接口，不需要 Token

**请求体**
```json
{
  "phone": "13800000001",
  "password": "Test1234",
  "name": "测试学员",
  "email": "test@example.com",
  "company": "测试公司"
}
```

**响应** `201`
```json
{
  "code": 201,
  "message": "注册成功",
  "data": {
    "user_id": 1,
    "username": "13800000001",
    "token": "eyJhbG..."
  }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| phone | ✅ | 手机号，同时作为登录用户名 |
| password | ✅ | 密码 |
| name | ✅ | 姓名 |
| email | ❌ | 邮箱 |
| company | ❌ | 单位名称 |

---

### 1.2 学员登录 `POST /api/auth/login`

> 公开接口

**请求体**
```json
{
  "username": "13800000001",
  "password": "Test1234",
  "role": "student"
}
```

**响应** `200`
```json
{
  "code": 200,
  "message": "登录成功",
  "data": {
    "user_id": 1,
    "username": "13800000001",
    "token": "eyJhbG..."
  }
}
```

> 🔑 **token** 需要存入环境变量，后续所有需要鉴权的接口都带上 `Authorization: Bearer {{token}}`

---

### 1.3 获取当前用户信息 `GET /api/auth/me`

> 需要 Token

**响应** `200`
```json
{
  "code": 200,
  "data": {
    "user_id": 1,
    "username": "13800000001",
    "role": "student",
    "name": "测试学员",
    "phone": "13800000001",
    "email": "test@example.com",
    "company": "测试公司"
  }
}
```

---

### 1.4 登出 `POST /api/auth/logout`

> 需要 Token

**响应** `200`
```json
{ "code": 200, "message": "已登出" }
```

---

## 2. 学员信息模块

### 2.1 学员主页 `GET /api/student/profile`

> 需要 Token + `student` 角色

**响应** `200`
```json
{
  "code": 200,
  "data": {
    "name": "测试学员",
    "phone": "13800000001",
    "total_study_time": 3600,
    "completed_courses": 2,
    "total_courses": 6,
    "course_progress": [
      { "course_id": 1, "title": "叉车基本构造", "progress_pct": 67 }
    ]
  }
}
```

---

### 2.2 学习记录 `GET /api/student/records`

> 需要 Token + `student` 角色

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 10 | 每页条数 |
| start_date | string | — | 起始日期 `YYYY-MM-DD` |
| end_date | string | — | 截止日期 `YYYY-MM-DD` |

**示例**：`GET /api/student/records?page=1&page_size=10&start_date=2026-07-01&end_date=2026-07-25`

---

### 2.3 学习统计 `GET /api/student/study-stats`

> 需要 Token + `student` 角色

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| days | int | 7 | 统计天数（7 或 30） |

**示例**：`GET /api/student/study-stats?days=7`

---

## 3. 课程模块

### 3.1 课程列表 `GET /api/courses`

> 公开接口

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 12 | 每页条数 |
| category | string | — | 分类过滤（如 `safety`） |

**响应** `200`
```json
{
  "code": 200,
  "data": {
    "items": [
      {
        "course_id": 1,
        "title": "叉车基本构造",
        "category": "structure",
        "description": "...",
        "cover_url": "/static/uploads/covers/xxx.jpg",
        "duration": 120,
        "chapter_count": 3
      }
    ],
    "total": 6,
    "page": 1,
    "page_size": 12
  }
}
```

---

### 3.2 课程详情 `GET /api/course/:course_id`

> 需要 Token

| 路径参数 | 说明 |
|----------|------|
| course_id | 课程 ID（从 3.1 获取） |

**示例**：`GET /api/course/1`

**响应** `200`
```json
{
  "code": 200,
  "data": {
    "course_id": 1,
    "title": "叉车基本构造",
    "chapters": [
      { "chapter_id": 1, "title": "第一章 叉车分类与型号", "order_num": 1, "duration": 40 },
      { "chapter_id": 2, "title": "第二章 叉车基本结构组成", "order_num": 2, "duration": 40 },
      { "chapter_id": 3, "title": "第三章 叉车主要技术参数", "order_num": 3, "duration": 40 }
    ],
    "progress_pct": 33
  }
}
```

---

### 3.3 章节幻灯片 `GET /api/chapter/:chapter_id/slides`

> 公开接口

| 路径参数 | 说明 |
|----------|------|
| chapter_id | 章节 ID（全局唯一，从 3.2 的 `chapters[].chapter_id` 获取） |

**示例**：`GET /api/chapter/1/slides`

> ⚠️ `chapter_id` 是数据库全局自增主键，不同课程的章节 ID 不重复，可放心使用。

---

### 3.4 更新学习进度 `POST /api/course/:course_id/progress`

> 需要 Token

**请求体**
```json
{
  "chapter_id": 1,
  "duration": 120
}
```

| 路径参数 | 说明 |
|----------|------|
| course_id | 课程 ID |

| 请求字段 | 必填 | 说明 |
|----------|------|------|
| chapter_id | ❌ | 当前学习的章节 ID |
| duration | ✅ | 本次学习时长（秒），不能为负数 |

---

## 4. 题库练习模块

> 全部需要 Token + `student` 角色

### 4.1 课程分类 `GET /api/question-bank/categories`

**示例**：`GET /api/question-bank/categories`

---

### 4.2 知识点列表 `GET /api/question-bank/knowledge-points`

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| category | string | — | 课程分类过滤 |
| parent_id | int | — | 父知识点 ID |

**示例**：`GET /api/question-bank/knowledge-points?category=safety`

---

### 4.3 随机练习 `GET /api/practice-mode/free`

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| type | string | — | 题型过滤（`single`/`multi`/`judge`） |
| knowledge_point_id | int | — | 知识点 ID |
| count | int | 20 | 抽题数量 |

**示例**：`GET /api/practice-mode/free?count=10&type=single`

---

### 4.4 顺序练习（开始/续练） `GET /api/practice-mode/sequential`

**示例**：`GET /api/practice-mode/sequential`

---

### 4.5 顺序练习进度 `GET /api/practice-mode/sequential-progress`

**示例**：`GET /api/practice-mode/sequential-progress`

---

### 4.6 章节练习 `GET /api/practice-mode/category`

> 按课程分类抽题

| Query | 类型 | 必填 | 说明 |
|-------|------|------|------|
| category | string | ✅ | 课程分类 |
| count | int | ❌ | 题量（0=全部） |

**示例**：`GET /api/practice-mode/category?category=safety&count=20`

---

### 4.7 知识点专项练习 `GET /api/practice-mode/knowledge-point`

| Query | 类型 | 必填 | 说明 |
|-------|------|------|------|
| knowledge_point_id | int | ✅ | 知识点 ID |
| count | int | ❌ | 题量（0=全部） |
| random | bool | ❌ | 是否随机排序 |

**示例**：`GET /api/practice-mode/knowledge-point?knowledge_point_id=5&count=10&random=true`

---

### 4.8 提交答案 `POST /api/practice-mode/submit`

**请求体**
```json
{
  "question_id": 1,
  "user_answer": "A",
  "practice_type": "free"
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| question_id | ✅ | 题目 ID |
| user_answer | ✅ | 用户答案（单选填字母，多选填数组 `["A","B"]`，判断填 `"true"/"false"`） |
| practice_type | ❌ | 练习类型（`free`/`sequential`/`category`/`knowledge_point`），���认 `free` |

**响应** `200`
```json
{
  "code": 200,
  "data": {
    "correct": true,
    "correct_answer": "A",
    "explanation": "叉车属于场（厂）内专用机动车辆..."
  }
}
```

---

### 4.9 保存练习进度 `POST /api/practice-mode/progress`

> 支持顺序练习、专项练习、章节练习的断点续练

**请求体**
```json
{
  "index": 5,
  "practice_mode": "sequential",
  "total": 20,
  "answers_state": { "1": "A", "2": "B" }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| index | ✅ | 当前做题进度序号 |
| practice_mode | ✅ | `sequential` / `category` / `knowledge_point` |
| total | ✅ | 总题量 |
| answers_state | ✅ | 已答题状态 JSON |

---

### 4.10 查询练习进度 `GET /api/practice-mode/progress`

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| mode | string | sequential | 练习模式 |

**示例**：`GET /api/practice-mode/progress?mode=sequential`

---

### 4.11 练习统计 `GET /api/practice-mode/stats`

**示例**：`GET /api/practice-mode/stats`

---

### 4.12 练习历史 `GET /api/practice-mode/history`

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| type | string | — | 题型过滤 |
| start_date | string | — | 起始日期 |
| end_date | string | — | 截止日期 |

---

### 4.13 知识点进度 `GET /api/practice-mode/knowledge-point-progress`

| Query | 类型 | 说明 |
|-------|------|------|
| knowledge_point_id | int | 指定关联知识点（可选，不传返回全部） |

---

## 5. 课程考核模块

> 全部需要 Token

### 5.1 获取考核题目 `GET /api/exam/:course_id`

| 路径参数 | 说明 |
|----------|------|
| course_id | 课程 ID |

**示例**：`GET /api/exam/1`

---

### 5.2 提交考核 `POST /api/exam/:course_id/submit`

**请求体**
```json
{
  "answers": { "1": "A", "2": "B", "3": ["A", "C"] }
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| answers | ✅ | 答案映射，key=题目 ID，value=答案 |

---

### 5.3 最近考核结果 `GET /api/exam/:course_id/result`

**示例**：`GET /api/exam/1/result`

---

### 5.4 考核历史 `GET /api/exam/history`

**示例**：`GET /api/exam/history`

---

## 6. 模拟考试模块

> 全部需要 Token + `student` 角色

### 6.1 开始考试 `POST /api/mock-exam/start`

**请求体**
```json
{
  "count": 20,
  "duration": 30
}
```

| 字段 | 必填 | 说明 |
|------|------|------|
| count | ❌ | 题目数量（默认全部） |
| duration | ❌ | 考试时长（分钟），默认 90 |

**响应** `200`
```json
{
  "code": 200,
  "message": "模拟考试开始",
  "data": {
    "mock_exam_id": 1,
    "questions": [...],
    "duration": 30,
    "remaining_time": 1800
  }
}
```

> 🔑 记下 `mock_exam_id`，后续接口用到

---

### 6.2 保存进度 `POST /api/mock-exam/:mock_exam_id/save`

**请求体**
```json
{
  "answers": { "1": "A", "2": "B" },
  "remaining_time": 1500
}
```

| 路径参数 | 说明 |
|----------|------|
| mock_exam_id | 考试 ID（从 6.1 获取） |

---

### 6.3 恢复考试 `GET /api/mock-exam/:mock_exam_id/resume`

**示例**：`GET /api/mock-exam/1/resume`

---

### 6.4 交卷 `POST /api/mock-exam/:mock_exam_id/submit`

**示例**：`POST /api/mock-exam/1/submit`（无需请求体，后端取已保存答案）

---

### 6.5 考试成绩 `GET /api/mock-exam/:mock_exam_id/result`

**示例**：`GET /api/mock-exam/1/result`

---

### 6.6 考试历史 `GET /api/mock-exam/history`

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 10 | 每页条数 |

**示例**：`GET /api/mock-exam/history?page=1&page_size=10`

---

## 7. 错题本模块

> 全部需要 Token + `student` 角色

### 7.1 错题列表 `GET /api/wrong-questions`

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| type | string | — | 题型过滤 |
| knowledge_point_id | int | — | 知识点过滤 |
| min_wrong_count | int | — | 最少错误次数 |

**示例**：`GET /api/wrong-questions?page=1&page_size=20&type=single`

---

### 7.2 重做错题 `POST /api/wrong-questions/:question_id/redo`

**请求体**
```json
{
  "user_answer": "B"
}
```

**示例**：`POST /api/wrong-questions/42/redo`

---

### 7.3 移出错题本 `POST /api/wrong-questions/:question_id/remove`

**示例**：`POST /api/wrong-questions/42/remove`

---

### 7.4 错题统计 `GET /api/wrong-questions/stats`

**示例**：`GET /api/wrong-questions/stats`

---

### 7.5 导出错题本 `GET /api/wrong-questions/export`

**示例**：`GET /api/wrong-questions/export`

> 返回 `.txt` 纯文本附件

---

## 8. 题库与题目查询（学生只读）

> 需要 Token

### 8.1 题目列表 `GET /api/question-bank/questions`

| Query | 类型 | 默认 | 说明 |
|-------|------|------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页条数 |
| type | string | — | 题型 |
| status | string | — | 状态（`published` 为已发布） |
| keyword | string | — | 关键词搜索 |
| knowledge_point_id | int | — | 知识点过滤 |

### 8.2 题目详情 `GET /api/question-bank/questions/:question_id`

**示例**：`GET /api/question-bank/questions/1`

### 8.3 题库统计 `GET /api/question-bank/stats`

### 8.4 课程分类 `GET /api/question-bank/categories`

### 8.5 知识点列表 `GET /api/question-bank/knowledge-points`

| Query | 类型 | 说明 |
|-------|------|------|
| category | string | 课程分类过滤 |
| parent_id | int | 父知识点 ID |

---

## 附录 A：通用响应码

| code | 说明 |
|------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 400 | 请求参数错误 |
| 401 | 未登录或 Token 过期 |
| 403 | 无权限（角色不匹配） |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

---

## 附录 B：题型枚举

| 值 | 题型 |
|----|------|
| `single` | 单选题 |
| `multi` | 多选题 |
| `judge` | 判断题 |

---

## 附录 C：课程分类枚举（种子数据）

| 值 | 中文名称 | course_id |
|----|----------|-----------|
| `structure` | 叉车基本构造 | 1 |
| `hydraulic` | 液压传动系统 | 2 |
| `driving` | 叉车驾驶操作 | 3 |
| `maintenance` | 日常维护保养 | 4 |
| `cargo` | 货物装卸作业 | 5 |
| `troubleshooting` | 故障排查与应急 | 6 |

---

## 附录 D：测试流程

### Apifox 设置

1. 创建环境，变量 `base_url`=`https://www.gccsmile.com`，`token`=留空
2. 接口根目录 → Auth → Bearer Token → `{{token}}` → 应用到子接口
3. 公开接口（注册/登录/课程列表/幻灯片）单独设 `No Auth`

### 后置脚本（登录接口）

```javascript
var jsonData = pm.response.json();
if (jsonData.code === 200 && jsonData.data && jsonData.data.token) {
    pm.environment.set("token", jsonData.data.token);
}
```

### 推荐执行顺序

```
1.1 注册 → 1.2 登录 → 1.3 /me → 2.1 profile
→ 3.1 课程列表 → 3.2 课程详情 → 3.3 幻灯片 → 3.4 更新进度
→ 4.3 随机练习 → 4.8 提交答案 → 4.11 练习统计
→ 6.1 模拟考试 → 6.2 保存进度 → 6.4 交卷 → 6.5 成绩
→ 7.1 错题列表 → 7.2 重做错题
```
