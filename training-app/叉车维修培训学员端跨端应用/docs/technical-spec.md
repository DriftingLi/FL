# 叉车维修培训学员端 - 技术方案

> 版本：v1.0 | 更新日期：2026-09-02

## 一、项目概述

### 1.1 产品定位

面向叉车维修人员的在线培训与考核平台，学员端支持 Android/iOS/H5/微信小程序四端访问。

### 1.2 核心功能

| 模块 | 功能 |
|---|---|
| 用户体系 | 登录/注册、个人信息、手机/邮箱绑定、微信一键登录 |
| 课程中心 | 课程列表、章节学习、学习进度上报、资料下载 |
| 考试系统 | 模拟考试、等级考试、成绩查询、错题本 |
| 练习题库 | 标签练习、随机练习、答题卡、数据报告 |
| AI 助手 | 智能问答、SSE 流式对话、多模型切换、会话管理 |
| 论坛社区 | 发帖/回帖、点赞、举报、签到打卡 |
| 积分体系 | 每日签到、浏览积分、积分明细、任务中心 |
| 求职模块 | 职位列表、在线简历、附件简历、职位投递 |

---

## 二、技术选型

### 2.1 技术栈

| 层级 | 选型 | 版本 | 选型理由 |
|---|---|---|---|
| 框架 | uni-app X | Latest | 一套代码编译四端，团队已有积累 |
| 语言 | UTS | Latest | uni-app X 原生语言，TypeScript 超集，类型安全 |
| 视图层 | Vue 3 SFC (.uvue) | 3.x | 组合式 API、响应式系统成熟 |
| 样式 | SCSS + Scoped CSS | - | uni-app X 标准方案，样式隔离 |
| 状态管理 | Vue 3 响应式单例 | - | 轻量级，无需引入 Pinia |
| HTTP | uni.request 封装 | - | 跨端统一请求层 |
| 后端 | Go + RESTful API | - | 高性能、易部署 |

### 2.2 目标平台适配

| 平台 | 最低版本 | 特殊处理 |
|---|---|---|
| Android | API 21 (5.0) | armeabi-v7a + arm64-v8a 双 ABI |
| iOS | iOS 12.0+ | 状态栏适配 |
| H5 | 现代浏览器 | History 路由 |
| 微信小程序 | 基础库 2.x | wx.login、分享、订阅消息 |

---

## 三、架构设计

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                      表现层 (UI)                         │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │  页面     │  │  组件     │  │ Composable│  │ 样式    │ │
│  │ pages/*.uvue│ │components/*.uvue│ │ composables/*.uts│ │ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
├─────────────────────────────────────────────────────────┤
│                      业务层 (Business)                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │  API 模块 │  │  Store   │  │  工具函数  │  │ 常量    │ │
│  │ api/*.uts │  │stores/*.uts│ │utils/*.uts│ │constants│ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
├─────────────────────────────────────────────────────────┤
│                      基础层 (Foundation)                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ 请求封装  │  │  存储封装  │  │  导航封装  │  │ 类型定义 │ │
│  │request.uts│ │storage.uts│ │navigation│  │types/*.uts│ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
├─────────────────────────────────────────────────────────┤
│                      平台层 (Platform)                    │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────┐ │
│  │ Android  │  │   iOS    │  │   H5     │  │ 小程序   │ │
│  └──────────┘  └──────────┘  └──────────┘  └─────────┘ │
└─────────────────────────────────────────────────────────┘
```

### 3.2 数据流

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  页面组件     │────>│  API 模块     │────>│  后端 API     │
│  *.uvue      │<────│  *.uts       │<────│  gccsmile.com │
└──────┬───────┘     └──────────────┘     └──────────────┘
       │
       v
┌──────────────┐     ┌──────────────┐
│  Store       │────>│ uni.Storage  │
│  响应式状态   │<────│  持久化       │
└──────────────┘     └──────────────┘
```

---

## 四、目录结构

```
├── api/                    # API 请求模块
│   ├── request.uts         # HTTP 请求封装（核心）
│   ├── auth.uts            # 认证相关 API
│   ├── course.uts          # 课程 API
│   ├── exam.uts            # 考试 API
│   ├── practice.uts        # 练习 API
│   ├── forum.uts           # 论坛 API
│   ├── aiAssistant.uts     # AI 助手 API
│   ├── student.uts         # 学员档案 API
│   ├── notification.uts    # 通知 API
│   ├── favorite.uts        # 收藏 API
│   ├── featured.uts        # 精选内容 API
│   ├── search.uts          # 搜索 API
│   ├── material.uts        # 学习资料 API
│   ├── job.uts             # 职位 API
│   ├── wrongQuestion.uts   # 错题本 API
│   ├── questionInteraction.uts # 题目互动 API
│   ├── credential.uts      # 证件切换 API
│   ├── mockExam.uts        # 模拟考试 API
│   ├── levelExam.uts       # 等级考试 API
│   └── helpers.uts         # 共享辅助函数
├── components/             # 通用组件
│   ├── app-badge/          # 徽章/角标
│   ├── app-button/         # 按钮
│   ├── app-card/           # 卡片容器
│   ├── app-chip/           # 标签/芯片
│   ├── app-empty-state/    # 空状态占位
│   ├── app-list-item/      # 列表项
│   ├── app-nav-bar/        # 自定义导航栏
│   ├── app-tabs/           # Tab 切换
│   └── tab-bar/            # 自定义 TabBar
├── composables/            # 组合式函数
│   └── useAiChat.uts       # AI 聊天核心逻辑
├── config/                 # 配置文件
│   └── env.uts             # 环境配置（API 地址、超时等）
├── constants/              # 全局常量
│   └── app.uts             # Storage Key、占位文案等
├── pages/                  # 页面模块
│   ├── index/              # 启动/引导页
│   ├── login/              # 登录
│   ├── register/           # 注册
│   ├── forgot-password/    # 找回密码
│   ├── profile-setup/      # 完善资料
│   ├── guide/              # 选择学习阶段
│   ├── dashboard/          # 首页（TabBar）
│   ├── courses/            # 课程模块
│   ├── exam/               # 考试模块
│   ├── practice/           # 练习模块
│   ├── profile/            # 个人中心（TabBar）
│   ├── forum/              # 论坛模块（TabBar）
│   ├── ai-assistant/       # AI 助手模块（TabBar）
│   ├── notifications/      # 通知中心
│   ├── points/             # 积分模块
│   ├── resources/          # 资源模块
│   ├── exam-info/          # 考情资讯
│   ├── resume/             # 简历模块
│   ├── jobs/               # 求职模块
│   ├── mall/               # 课程商城
│   ├── search/             # 搜索
│   └── featured/           # 精选内容
├── stores/                 # 状态管理
│   ├── index.uts           # 导出入口
│   └── auth.uts            # 认证状态管理
├── types/                  # 类型定义
│   └── index.uts           # 全局类型（60+ 类型）
├── utils/                  # 工具函数
│   ├── storage.uts         # 本地存储封装
│   ├── navigation.uts      # 页面导航封装
│   ├── format.uts          # 格式化工具
│   ├── aiPay.uts           # AI 助手版本管理
│   ├── markdown.uts        # Markdown 解析器
│   └── system.uts          # 系统信息
├── static/                 # 静态资源
│   ├── icons/              # 图标
│   └── tabbar/             # TabBar 图标
├── App.uvue                # 根组件
├── main.uts                # 入口文件
├── manifest.json           # 应用配置
├── pages.json              # 路由配置
├── uni.scss                # 全局样式变量
└── package.json            # 项目配置
```

---

## 五、核心模块设计

### 5.1 HTTP 请求层 (api/request.uts)

**设计要点**：

```uts
// 统一响应格式
type ApiResponse<T> = {
  code : number,
  message : string,
  data : T
}

// 请求封装核心能力
- 自动注入 Authorization: Bearer <token>
- 统一解包 ApiResponse<T>
- 401 自动刷新 Token（refresh_token）
- 刷新失败自动跳转登录页
- X-Silent: 1 头关闭错误 Toast
- 防重入：多个并发 401 只触发一次跳转
```

**请求流程**：

```
发起请求
  │
  ├─ 注入 Token
  │
  ├─ 发送 HTTP 请求
  │
  ├─ 200 + code=0 → 返回 data
  │
  ├─ 200 + code≠0 → 显示错误 Toast
  │
  ├─ 401 → 尝试刷新 Token
  │         │
  │         ├─ 成功 → 重试原请求
  │         │
  │         └─ 失败 → 清空登录态 → 跳转登录页
  │
  └─ 其他 HTTP 错误 → 显示网络错误
```

### 5.2 认证状态管理 (stores/auth.uts)

**设计要点**：

```uts
// 响应式状态
const token = ref('')
const user = ref<UserInfo | null>(null)
const isLoggedIn = computed(() => !!token.value)

// 持久化策略
- 登录成功 → token/user/loginProvider 写入 Storage
- 启动时 → 从 Storage 恢复 → 调 /auth/me 校验
- 登出 → 清空所有状态 + Storage

// 登录方式
- 密码登录
- 验证码登录（手机号/邮箱）
- 微信一键登录（小程序端）

// Token 刷新机制
registerRefreshTokenHandler(handler) // 注册到 request 层，避免循环依赖
```

### 5.3 AI 助手模块

**核心架构**：

```
┌─────────────────────────────────────────────┐
│              useAiChat (Composable)          │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│  │ 会话状态 │  │ 消息状态 │  │ 输入状态 │     │
│  └─────────┘  └─────────┘  └─────────┘     │
├─────────────────────────────────────────────┤
│              SSE 流式对话                    │
│  ┌─────────────────────────────────────┐    │
│  │ uni.request → SSE 流 → 实时渲染     │    │
│  └─────────────────────────────────────┘    │
├─────────────────────────────────────────────┤
│              页面层                          │
│  ┌──────────────┐  ┌──────────────┐         │
│  │ ai-basic.uvue│  │ ai-assistant │         │
│  │ (基础版)      │  │ (专业版)     │         │
│  └──────────────┘  └──────────────┘         │
└─────────────────────────────────────────────┘
```

**SSE 流式对话实现**：

```uts
// 核心流程
1. 建立 SSE 连接（uni.request）
2. 监听 onData 事件，逐块解析
3. 实时更新消息内容（ref 响应式）
4. 完成后关闭连接

// 错误处理
- 连接超时 → 提示用户重试
- 流中断 → 保留已接收内容
- 网络异常 → 自动重连或提示
```

### 5.4 考试系统

**考试流程**：

```
开始考试
  │
  ├─ 获取试题列表
  │
  ├─ 进入答题页面
  │   ├─ 显示题目
  │   ├─ 选择答案
  │   ├─ 答题卡导航
  │   └─ 自动保存（每题）
  │
  ├─ 提交考试
  │   ├─ 确认提交
  │   └─ 调用交卷 API
  │
  └─ 查看成绩
      ├─ 得分
      ├─ 用时
      ├─ 正确率
      └─ 答案解析
```

---

## 六、跨端适配策略

### 6.1 平台差异处理

| 差异点 | 处理方案 |
|---|---|
| 导航栏高度 | `useStatusBarHeight()` 动态获取，自定义导航栏 |
| TabBar | 自定义 `tab-bar` 组件，统一视觉 |
| 微信登录 | `uni.login()` + 后端换 Token |
| 分享 | `onShareAppMessage` 钩子 + 条件编译 |
| 推送通知 | Android/iOS 原生推送 + 小程序订阅消息 |
| 文件下载 | `uni.downloadFile` + `uni.openDocument` |

### 6.2 条件编译

```uts
// 平台判断
// #ifdef MP-WEIXIN
// 微信小程序专属逻辑
// #endif

// #ifdef APP-PLUS
// App 端专属逻辑
// #endif

// #ifdef H5
// H5 端专属逻辑
// #endif
```

### 6.3 样式适配

```scss
// rpx 单位 - 自适应宽度
.container {
  padding: 24rpx;
}

// 安全区域适配
.safe-area-bottom {
  padding-bottom: constant(safe-area-inset-bottom);
  padding-bottom: env(safe-area-inset-bottom);
}
```

---

## 七、性能优化

### 7.1 首屏优化

| 策略 | 实现 |
|---|---|
| 路由懒加载 | pages.json 中页面按需加载 |
| 图片懒加载 | 列表图片进入视口才加载 |
| 骨架屏 | 关键页面添加骨架屏占位 |
| 接口缓存 | 课程列表等静态数据本地缓存 |

### 7.2 列表优化

| 策略 | 实现 |
|---|---|
| 虚拟列表 | 长列表使用分页加载 |
| 图片压缩 | 服务端返回缩略图 URL |
| 复用机制 | 组件 key 优化列表渲染 |

### 7.3 网络优化

| 策略 | 实现 |
|---|---|
| 请求合并 | 批量接口合并请求 |
| 失败重试 | 网络异常自动重试 1 次 |
| 离线缓存 | 课程章节内容离线可用 |

---

## 八、安全设计

### 8.1 认证安全

- Token 存储：`uni.setStorageSync`（客户端安全存储）
- Token 刷新：过期前自动刷新，避免登录中断
- 401 处理：统一拦截，防止 Token 泄露

### 8.2 数据安全

- 敏感信息不存储在本地
- API 请求使用 HTTPS
- 用户密码传输加密

### 8.3 代码安全

- UTS 类型安全，编译时类型检查
- 无 eval/Function 等动态执行
- 防 XSS：用户输入统一转义

---

## 九、部署方案

### 9.1 构建产物

| 平台 | 产物格式 | 说明 |
|---|---|---|
| Android | APK/AAB | HBuilderX 云打包或本地打包 |
| iOS | IPA | 需 Mac + 证书 |
| H5 | HTML/CSS/JS | 部署到 Nginx |
| 微信小程序 | wxapkg | 上传微信后台 |

### 9.2 环境配置

```uts
// config/env.uts
const ENV_MODE = 'development' // development / production

const API_BASE_URL = ENV_MODE === 'development'
  ? 'https://dev-api.gccsmile.com/api'
  : 'https://www.gccsmile.com/api'
```

### 9.3 版本管理

- 版本号：`manifest.json` 中 `versionName` + `versionCode`
- 强更新：后端返回版本号对比，提示强制更新
- 静默更新：H5 端自动获取最新版本

---

## 十、风险与应对

| 风险 | 影响 | 应对措施 |
|---|---|---|
| uni-app X 生态不成熟 | 部分功能需自行封装 | 核心组件提前验证，预留原生开发时间 |
| UTS 调试困难 | 问题定位耗时 | 善用 console.log + HBuilderX 调试器 |
| 小程序端兼容性 | 样式/API 差异 | 逐端测试，条件编译处理差异 |
| AI 流式对话稳定性 | SSE 连接中断 | 自动重连 + 断点续传设计 |
| 首屏加载慢 | 用户体验差 | 骨架屏 + 接口缓存 + 懒加载 |

---

## 十一、开发规范

### 11.1 命名规范

| 类型 | 规范 | 示例 |
|---|---|---|
| 页面文件 | kebab-case.uvue | `course-detail.uvue` |
| 组件文件 | kebab-case.uvue | `app-button.uvue` |
| API 文件 | camelCase.uts | `course.uts` |
| 类型定义 | PascalCase | `CourseItem` |
| 常量 | UPPER_SNAKE_CASE | `STORAGE_KEY_TOKEN` |

### 11.2 组件规范

```uts
// 组件 props 定义
const props = defineProps<{
  title?: string,
  loading?: boolean,
  disabled?: boolean
}>()

// 组件 emits 定义
const emit = defineEmits<{
  (e: 'click', value: string): void
}>()
```

### 11.3 样式规范

```scss
// 页面级 scoped 样式
<style lang="scss" scoped>
.page-container {
  // 页面根容器
}

.section-title {
  // 模块标题
}
</style>
```

---

## 附录

### A. 页面清单（47 个路由）

| 模块 | 页面 | 路径 |
|---|---|---|
| 启动 | 启动页 | pages/index/index |
| 认证 | 登录 | pages/login/login |
| 认证 | 注册 | pages/register/register |
| 认证 | 找回密码 | pages/forgot-password/forgot-password |
| 认证 | 完善资料 | pages/profile-setup/profile-setup |
| 认证 | 选择学习阶段 | pages/guide/choose-cert |
| 首页 | 首页 | pages/dashboard/dashboard |
| 课程 | 课程列表 | pages/courses/courses |
| 课程 | 课程详情 | pages/courses/course-detail |
| 课程 | 章节学习 | pages/courses/chapter-view |
| 考试 | 考试入口 | pages/exam/exam |
| 考试 | 模拟考试 | pages/exam/mock-exam |
| 考试 | 模考成绩 | pages/exam/mock-exam-result |
| 考试 | 等级考试 | pages/exam/level-exam-do |
| 考试 | 等级考试成绩 | pages/exam/level-exam-result |
| 练习 | 题库练习 | pages/practice/practice |
| 练习 | 答题 | pages/practice/practice-do |
| 练习 | 数据报告 | pages/practice/practice-report |
| 论坛 | 学员论坛 | pages/forum/forum |
| 论坛 | 帖子详情 | pages/forum/forum-detail |
| 论坛 | 发布新帖 | pages/forum/forum-create |
| 论坛 | 我的论坛 | pages/forum/my-forum |
| 论坛 | 签到 | pages/forum/check-in |
| AI | AI 助手 | pages/ai-assistant/ai-assistant |
| AI | 对话设置 | pages/ai-assistant/ai-settings |
| AI | 选择版本 | pages/ai-assistant/version-select |
| AI | 基础版 | pages/ai-assistant/ai-basic |
| AI | 自定义模型 | pages/ai-assistant/custom-models |
| 我的 | 个人中心 | pages/profile/profile |
| 我的 | 设置 | pages/profile/settings |
| 我的 | 个人信息 | pages/profile/personal-info |
| 我的 | 学习记录 | pages/profile/records |
| 我的 | 练习记录 | pages/profile/practice-records |
| 我的 | 个人动态 | pages/profile/personal-activity |
| 我的 | 模考历史 | pages/profile/mock-exam-records |
| 我的 | 错题本 | pages/profile/wrong-questions |
| 我的 | 我的收藏 | pages/profile/favorites |
| 积分 | 积分明细 | pages/points/points-detail |
| 积分 | 任务中心 | pages/points/task-center |
| 通知 | 消息通知 | pages/notifications/notifications |
| 资源 | 我的已购 | pages/resources/my-purchases |
| 资源 | 我的上传 | pages/resources/my-uploads |
| 资源 | 上传资源 | pages/resources/upload-resource |
| 资讯 | 考情资讯 | pages/exam-info/exam-info |
| 求职 | 简历 | pages/resume/resume |
| 求职 | 在线简历 | pages/resume/resume-edit |
| 求职 | 附件简历 | pages/resume/resume-attach |
| 求职 | 职位广场 | pages/jobs/job-list |
| 求职 | 职位详情 | pages/jobs/job-detail |
| 商城 | 课程商城 | pages/mall/mall |
| 搜索 | 搜索 | pages/search/search |
| 精选 | 精选列表 | pages/featured/featured-list |
| 精选 | 文章详情 | pages/featured/featured-detail |

### B. API 模块清单（19 个模块）

| 模块 | 文件 | 主要接口 |
|---|---|---|
| HTTP 封装 | request.uts | get/post/put/del/patch/uploadFile |
| 认证 | auth.uts | login/register/sendCode/wechatLogin |
| 课程 | course.uts | list/detail/chapters/progress/tags |
| 模拟考试 | mockExam.uts | start/save/restore/submit/result |
| 等级考试 | levelExam.uts | list/start/save/submit/result |
| 练习 | practice.uts | random/sequential/tags/submit/stats |
| 学员 | student.uts | profile/stats/records/courses |
| 论坛 | forum.uts | topics CRUD/replies/like/report/checkIn |
| AI 助手 | aiAssistant.uts | models/sessions/chat(SSE) |
| 通知 | notification.uts | list/unread/read |
| 收藏 | favorite.uts | list/add/remove/check |
| 精选 | featured.uts | list/detail/viewCount |
| 搜索 | search.uts | search/categories |
| 资料 | material.uts | list/detail/download |
| 职位 | job.uts | list/detail/report/apply |
| 错题 | wrongQuestion.uts | list/remove/redo/stats |
| 题目互动 | questionInteraction.uts | comments/notes/knowledge |
| 证件 | credential.uts | current/list/switch |

### C. 通用组件清单（9 个组件）

| 组件 | 用途 | Props |
|---|---|---|
| app-badge | 徽章/角标 | text, type, size |
| app-button | 按钮 | type, loading, disabled, size |
| app-card | 卡片容器 | padding, shadow |
| app-chip | 标签/芯片 | text, active, color |
| app-empty-state | 空状态占位 | icon, text, actionText |
| app-list-item | 列表项 | title, subtitle, icon, arrow |
| app-nav-bar | 自定义导航栏 | title, leftIcon, rightIcon |
| app-tabs | Tab 切换 | tabs, activeIndex |
| tab-bar | 自定义 TabBar | tabs, activeIndex |
