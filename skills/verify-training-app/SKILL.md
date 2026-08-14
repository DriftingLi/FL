---
name: verify-training-app
description: 叉车维修培训学员端跨端应用（UniApp X）变更验证流程。完成代码改动后自动执行，覆盖文件结构完整性、UniApp 配置校验、静态代码规则、平台编译四类检查，全部通过才允许宣告完成。
---

# verify-training-app

适用于 `training-app/叉车维修培训学员端跨端应用/`（UniApp X，目标平台：微信小程序 / H5 / App）的变更后验证流程。

## 前置条件

- Node.js 可用
- `npx @dcloudio/uni-cli` 可用（用于微信小程序编译检查）
- 工作目录为项目根目录（包含 `manifest.json`、`pages.json` 的目录）

## 步骤 1：文件结构完整性检查

### 1.1 pages.json 页面路径校验

读取 `pages.json`，提取所有 `pages[*].path` 值，逐条确认对应目录下存在以下文件之一：

- `{path}/index.uvue`
- `{path}/index.vue`
- `{path}.uvue`

任何页面路径找不到对应文件即失败。

### 1.2 tabBar 图标校验

读取 `pages.json` 的 `tabBar.list`，对每个 `iconPath` 和 `selectedIconPath` 确认 `static/` 下文件存在。任何图标文件缺失即失败。

### 1.3 关键入口文件存在性

确认以下文件存在，缺失即失败：

- `main.uts`
- `App.uvue`
- `pages.json`
- `manifest.json`
- `index.html`
- `types/index.uts`

## 步骤 2：UniApp 配置合法性检查

### 2.1 JSON 语法校验

依次验证以下文件是合法 JSON：

- `pages.json`
- `manifest.json`
- `platformConfig.json`（如存在）

任何 JSON 解析失败即失败。

### 2.2 pages.json 关键字段校验

- `globalStyle.navigationBarTitleText`：非空字符串
- `tabBar.list`：长度 ≤ 5（微信小程序限制）
- 每个 tabBar 项必须包含 `pagePath`、`text`、`iconPath`、`selectedIconPath`
- 每个 `pagePath` 必须出现在 `pages` 数组中

任何一项违规即失败。

### 2.3 manifest.json 关键字段校验

- `appid`：非空字符串
- `versionName`：符合语义化版本格式（如 `1.0.0`）
- `versionCode`：正整数
- `vueVersion`：必须为 `"3"`

## 步骤 3：静态代码规则检查

在项目根目录执行以下规则检查，每项有违规即记录，超过阈值即失败：

### 3.1 禁止在 UTS 文件中使用 `Object.keys()`

项目代码在 `api/request.uts` 中有明确注释：Android 端不支持 `Object.keys()`，需用 `for...in` 遍历。

检查所有 `.uts` 文件，搜索 `Object.keys(` 和 `Object.values(`，出现即失败。

例外：`node_modules/`、`unpackage/`、`static/` 目录排除。

### 3.2 import 路径一致性

检查所有 `api/*.uts` 中的 `import` 语句，确认导入路径指向 `../config/env`、`../constants/app`、`../utils/storage`、`../types/index` 之一。任何指向不存在文件的导入即失败。

### 3.3 平台条件编译注释语法

检查所有 `.uts` 和 `.uvue` 文件中的条件编译注释，确认格式为：

- `// #ifdef <PLATFORM>`
- `// #ifndef <PLATFORM>`
- `// #endif`

合法平台值：`APP-ANDROID`、`APP-IOS`、`APP-HARMONY`、`H5`、`MP-WEIXIN`、`MP-ALIPAY`、`MP-BAIDU`、`MP-TOUTIAO`、`QUICKAPP`、`MP-JD`、`MP-QQ`。

格式错误即失败。

## 步骤 4：平台编译检查

### 4.1 微信小程序编译

```bash
npx @dcloudio/uni-cli build --platform mp-weixin --mode development
```

判断标准：

- 编译过程无 Error 级别错误
- `unpackage/dist/build/mp-weixin/` 目录生成
- 编译退出码为 0

注意：编译警告（Warning）记录但不阻塞，在报告中列出。

如果 `npx @dcloudio/uni-cli` 不可用（网络问题或包未安装），记录为环境问题，跳过此步骤，在报告中注明。

## 汇报格式

检查全部通过后：

```
✅ verify-training-app 通过
  文件结构: 通过（N 个页面、N 个图标均已确认）
  配置校验: pages.json / manifest.json / platformConfig.json 合法
  静态规则: Object.keys 禁用 / import 路径 / 条件编译语法 全部通过
  平台编译: mp-weixin 编译通过（unpackage/dist/build/mp-weixin/ 已生成）
```

任何步骤失败：

```
❌ verify-training-app 失败
  步骤: [步骤编号.名称]
  问题: [具体描述，包含违规文件路径和行号]
  修复建议: [修复方向]
```

## 注意事项

- 本 Skill 只覆盖能明确判断对错的自动化检查；UI 视觉、交互体验、业务流程逻辑不属于本 Skill 范围，需人工验收。
- 步骤 4 编译检查生成的文件在 `unpackage/` 目录下，属于构建产物，不需要提交到版本控制。
- 如果改动只涉及某一端（例如只改了 H5 相关逻辑），可跳过 `mp-weixin` 编译，但必须在报告中注明。
- `unpackage/` 和 `node_modules/` 目录在文件搜索中排除，避免噪音。
