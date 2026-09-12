# 叉车维修培训学员端 - 领域词汇表

面向叉车维修人员的在线培训与考核平台，学员端支持 Android/iOS/H5/微信小程序四端访问。

## 认证与安全

**生物识别认证（Biometric Authentication）**：
利用设备硬件能力（指纹、面容）验证用户身份的认证方式。Android/iOS/微信小程序走 SOTER 协议，H5 用浏览器弹窗模拟。

_Avoid_: 指纹认证、面容认证、人脸认证（这些是生物识别的子集，不是完整概念）

**SOTER**：
微信主导的生物认证协议，覆盖 Android/iOS/微信小程序，提供标准化的指纹/面容认证能力。

**凭据门控回填（Credential-Gated Autofill）**：
记住密码的核心策略——凭据持久化存储后，不在启动时自动回填，而是要求用户通过生物识别验证后才回填。区别于传统的"自动填充"模式。

_Atry_: 自动填充、自动回填（暗示无验证，安全模型不同）

**快捷登录（Quick Login）**：
生物识别验证通过后，不回填表单，而是用本地加密保存的 refresh_token 静默换取新会话、直接进入应用。与凭据门控回填共享"生物验证通过才解锁本地数据"的门控模型，区别在于解锁的是登录凭证而非表单内容；验证对象是机主本人（本地生物识别），服务端不做生物特征比对。

**凭据降级（Credential Degradation）**：
当生物识别不可用时（平台不支持 SOTER，或设备未启用任何生物识别方式），回退到较低安全级别的存储策略。当前实现：降级为"仅记住账号"（不保存密码）。

**孤儿凭据（Orphaned Credentials）**：
已保存的完整凭据（含密码密文）因生物识别在保存之后变得不可用（用户在系统设置中删除指纹/面容、硬件失效）而永远无法解锁的数据。属于应自愈清理的残留状态。

**secureStorage**：
凭据存储抽象层（`utils/secureStorage.uts`），封装 `hasStoredCredentials`、`saveSecureCredentials`、`loadSecureCredentials`、`clearSecureCredentials` 等接口。App 端经 uni-secure-storage 插件加密落盘（Android Keystore / iOS Keychain），非 App 端明文存储（H5 生产策略另行评估）。

**useBiometric**：
生物识别封装 composable（`composables/useBiometric.uts`），提供 `checkSupport()`（检测设备支持的生物识别方式）与 `authenticate()`（触发一次生物验证）方法，内部处理平台检测、SOTER 调用、降级逻辑。

## 培训体系

**课程中心**：
提供叉车维修相关的在线课程，包含章节学习、学习进度上报、资料下载。

**考试系统**：
模拟考试与等级考试，支持成绩查询和错题本功能。

**练习题库**：
标签练习、随机练习、答题卡、数据报告。

## 资源与投稿（移动端映射）

域定义见根仓库 CONTEXT.md「资料投稿（contribution）」词条族；本节只记移动端映射（#710）。

**资源 tab（forum-resource-panel）**：
论坛页资源分区的 2x2 入口：课程商城 / 我的已购 / 我的上传 / 上传资源。「上传资源」格子与资源区的创建入口（header「发帖」/搜索栏）一律跳 `forum-create?scope=resource`——统一发布页（#760），资源 tab 承载投稿表单，独立页 upload-resource 已退役。

**统一发布页（forum-create）**：
分类行三 tab（ADR-0040 后）：广场/资源/知识问答——**「备考经验」不再是发布选项**（学员不能自称考经，它是管理端认定）。资源=投稿表单（文件方式三行〔微信聊天文档/本地文件上传/选择压缩文件，白名单 pdf/doc/docx/ppt/pptx/xls/xlsx/zip/mp4、单 ≤20MB、合计 ≤50MB、≤5 个前置校验〕+ 标题 ≤120 + 简介 + 目标证件取当前证件〔无证件阻断〕；链路先传后交：逐个 POST `/contributions/upload-file` 暂存 → POST `/contributions` 建稿，API 封装 `api/contribution.uts`）；其余三 tab=发帖表单（标题 ≤100 + 正文 + 图片 ≤9，`createForumTopicApi`，category 即 tab scope）。resource 永不发进发帖接口（isResourceMode 分流 + 编辑回填 normalizeCategory 双守卫）。

**我的已购（my-purchases）**：
已购课程 = GET `/student/courses`（我的课程，根仓库 ADR-0017）口径——后端**无购买概念**（课程全开放，`points_price` 积分兑换无已兑清单 endpoint），真「已购清单」如需支持另立后端票；不要再用 profile.course_progress 假充已购（契约测试已钉）。

## 用户体系

**学员**：
平台的主要用户角色，通过登录注册后使用培训、考试、求职等功能。

**学员端**：
面向学员的跨端应用（Android/iOS/H5/微信小程序），即本项目。
