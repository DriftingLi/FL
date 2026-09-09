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

**凭据降级（Credential Degradation）**：
当平台不支持生物识别时，回退到较低安全级别的存储策略。当前实现：支付宝等平台降级为"仅记住账号"（不保存密码）。

**secureStorage**：
凭据存储抽象层（`utils/secureStorage.uts`），封装 `hasStoredCredentials`、`saveSecureCredentials`、`loadSecureCredentials`、`clearSecureCredentials` 四个接口。demo 阶段明文存储，生产可替换为加密实现。

**useBiometric**：
生物识别封装 composable（`composables/useBiometric.uts`），提供 `requestBiometric()` 方法，内部处理平台检测、SOTER 调用、降级逻辑。

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
论坛页资源分区的 2x2 入口：课程商城 / 我的已购 / 我的上传 / 上传资源。「上传资源」格子与资源区的创建入口（header「发帖」/搜索栏）一律跳 `pages/resources/upload-resource`——资源不是论坛帖，`forum-create?scope=resource` 的 category 非法必 400（已由 forum-create 的 normalizeCategory 与契约测试双重钉死）。

**上传资源页（upload-resource）**：
投稿表单：标题 ≤120 + 简介 ≤2000 + 文件 1–5 个（扩展名白名单、单 ≤20MB、合计 ≤50MB 前置校验）+ 目标证件取当前证件（无证件阻断）。链路先传后交：逐个文件 POST `/contributions/upload-file` 暂存 → POST `/contributions` 建稿。API 封装 `api/contribution.uts`，类型内聚 api 模块（先例 `api/aiAssistant.uts`，不进 types/index.uts）。

**我的已购（my-purchases）**：
已购课程 = GET `/student/courses`（我的课程，根仓库 ADR-0017）口径——后端**无购买概念**（课程全开放，`points_price` 积分兑换无已兑清单 endpoint），真「已购清单」如需支持另立后端票；不要再用 profile.course_progress 假充已购（契约测试已钉）。

## 用户体系

**学员**：
平台的主要用户角色，通过登录注册后使用培训、考试、求职等功能。

**学员端**：
面向学员的跨端应用（Android/iOS/H5/微信小程序），即本项目。
