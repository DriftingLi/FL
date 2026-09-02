# 叉车维修培训学员端 - 移动端开发约定

> 本文件为移动端（uni-app-x 跨端应用）的 AI agent 工作约定，覆盖开发、测试、构建、发布全流程。
> 根目录 `E:\FL\AGENTS.md` 提供系统级全局约定，本文件补充移动端特有规范。

## 项目概述

- **技术栈**：uni-app-x + Vue 3 + TypeScript
- **目标平台**：Android、iOS、微信小程序、支付宝小程序
- **代码风格**：Composition API + `<script setup>` 语法

## 开发约定

### 文件结构
```
src/
├── pages/           # 页面组件（按功能模块分组）
├── components/      # 公共组件
├── composables/     # 组合式函数
├── stores/          # 状态管理（Pinia）
├── api/             # API 接口封装
├── utils/           # 工具函数
├── types/           # TypeScript 类型定义
├── constants/       # 常量定义
├── static/          # 静态资源
└── assets/          # 样式资源
```

### 代码规范
- **命名**：文件名使用 kebab-case，组件名使用 PascalCase
- **导入顺序**：@vue/* → uni-app API → 第三方库 → 本地模块
- **类型安全**：所有 props、emit、状态必须有 TypeScript 类型
- **组件通信**：优先使用 props/emit，跨层用 provide/inject 或 Pinia

### 平台兼容性
- **条件编译**：使用 `#ifdef` / `#ifndef` 处理平台差异
- **API 选择**：优先使用 uni-app API，原生 API 用条件编译包装
- **样式适配**：使用 rpx 单位，避免固定像素值
- **触摸交互**：确保触摸区域不小于 44px × 44px

## 测试约定

### 自动化测试（E2E）
- 使用 uni-app 官方测试框架 `@dcloudio/uni-automator`
- 测试文件命名：`*.test.js`，放在被测试文件同级目录
- 测试脚本：`npm run test`（默认 H5）、`npm run test:mp-weixin`（微信小程序）

### 测试配置
- **jest.config.js**：Jest 配置文件，定义测试环境和平台参数
- **env.js**：测试设备配置（H5/Android/iOS/微信小程序）
- **示例测试**：`pages/index/index.test.js`

### 测试覆盖要求
- 工具函数：100% 覆盖
- 组件：核心交互逻辑覆盖
- API 模块：mock 测试覆盖

### 运行测试
```bash
# 安装依赖
npm install

# 运行 H5 测试
npm run test:h5

# 运行微信小程序测试
npm run test:mp-weixin

# 运行 Android 测试
npm run test:android
```

## 构建与发布

### 开发环境（使用 HBuilderX）
```bash
# 启动 H5 开发服务器
# 在 HBuilderX 中运行：运行 → 运行到浏览器 → Chrome

# 启动微信小程序开发
# 在 HBuilderX 中运行：运行 → 运行到小程序模拟器 → 微信开发者工具

# 启动 Android 开发
# 在 HBuilderX 中运行：运行 → 运行到手机或模拟器 → Android
```

### 生产构建（使用 HBuilderX）
```bash
# 构建 H5
# 在 HBuilderX 中发行：发行 → H5-手机版

# 构建微信小程序
# 在 HBuilderX 中发行：发行 → 小程序-微信

# 构建 Android APK
# 在 HBuilderX 中发行：发行 → 原生App-云打包
```

### 发布流程
1. **代码审查**：所有改动必须通过 PR 审查
2. **测试验证**：单元测试 + E2E 测试全绿
3. **构建验证**：生产构建无错误
4. **平台审核**：微信小程序提交审核，Android/iOS 打包测试
5. **灰度发布**：先小范围验证，再全量发布

## 常见问题

### 跨端兼容性问题
- **问题**：某些 API 在特定平台不可用
- **解决**：使用条件编译 + 平台检测 + 降级方案

### 性能优化
- **列表渲染**：使用 `v-for` 加 `key`，避免 `index` 作为 key
- **图片优化**：使用懒加载，压缩图片大小
- **内存管理**：及时销毁定时器、事件监听

### 调试技巧
- **H5**：使用浏览器开发者工具
- **小程序**：使用微信开发者工具
- **Android**：使用 Chrome DevTools 远程调试

## 注意事项

1. **每次改动完成后，都必须创建一个对应的 git commit，以便后续追踪和回滚。**
2. **每次改动后，都必须编写或更新相关测试，并在交互给用户前，确保所有测试和验证全部通过。**
3. **提交前必须运行**：`npm run type-check`（类型检查）和 `npm test`（单元测试）
4. **避免使用**：`setTimeout`/`setInterval` 等可能造成内存泄漏的 API，优先使用 uni-app 生命周期管理

## 相关文档

- **Git 工作流**：`docs/GIT_WORKFLOW.md`
- **UI 规范**：`docs/ui-spec.md`
- **技术规范**：`docs/technical-spec.md`
- **产品设计**：`docs/product-design.md`