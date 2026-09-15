# 叉车维修培训学员端 — 移动端 UI 规范

> 基于 Figma 设计稿（2026-09-01 版本）
> 适用范围：`training-app/叉车维修培训学员端跨端应用` 全部 `.uvue` 页面

---

## 1. 色彩体系

> ⚠️ **本表的变量名以 `uni.scss` 实际定义为准**（2026-09-15 校正，#979 实测）：
> 此前表里写的 `$text-secondary` / `$text-placeholder` / `$text-inverse` / `$bg-page` **在 `uni.scss` 里都不存在**
> （真实名是 `$text-color-secondary` / `$text-color-placeholder` / `$text-color-inverse` / `$bg-color`）；
> 照旧表写会**静默失效**（`<style>` 未声明 `lang="scss"` 时变量不预处理、声明被直接丢弃，页面无报错、只是颜色没生效）。
> 机检：`utils/searchContract.test.js`「引用的 `$变量` 在 `uni.scss` 里都存在」。

### 1.1 主色

| Token | 色值 | 用途 |
|---|---|---|
| `$primary-color` | `#2979ff` | 按钮、选中态、Tab 高亮、链接、**主色 header / 状态栏** |
| `$primary-color-light` | `#5b9aff` | 渐变底部、hover 态 |
| `$primary-color-dark` | `#1c9eff` | 按钮 pressed 态 |
| `$primary-tint` | `#ebf5ff` | 主色浅底（分区标题带、选中标签底、历史 chip 底） |

### 1.2 页面背景

> ⚠️ **App 平台（uvue 原生端）的 `linear-gradient` 只接受「恰好 2 个颜色值 + 不带百分比停靠位 + `to` 关键字方向」**
> （2026-09-13 真机实测，见 **#937** / ADR-0010；守护 `utils/gradientSyntaxContract.test.js`）。
> 三色写法与 `deg` 角度都会被原生端**整条丢弃**（`deg` 的失效形态是**整幅退化成两端色的中点**，不是空白）。
> 另注：`.uvue` 不支持 CSS 变量 ⇒ 渐变只能逐处写字面色值。

| Token | 色值 | 用途 |
|---|---|---|
| `$gradient-start` | `#CFE9FB` | 渐变顶部（tabBar 页面专用） |
| `$bg-color` | `#F8F8F8` | 通用页面背景 |
| `$bg-tint` | `#F1F6FF` | 淡蓝画布（列表/结果页：白卡浮于其上，避免整页纯白） |

### 1.3 语义色

| Token | 色值 | 用途 |
|---|---|---|
| `$success-color` | `#4cd964` | 成功状态 |
| `$warning-color` | `#ff9900` | 警告状态 |
| `$danger-color` | `#ff3b30` | 危险操作（退出登录、删除） |

### 1.4 文字色

| Token | 色值 | 用途 |
|---|---|---|
| `$text-color` | `#333333` | 主文字（标题、正文） |
| `$text-color-secondary` | `#666666` | 次要文字（描述、副标题、结果片段） |
| `$text-color-placeholder` | `#999999` | 辅助文字（日期、placeholder） |
| `$text-disabled` | `#cccccc` | 禁用 / 弱化文字 |
| `$text-color-inverse` | `#ffffff` | 反色文字（深色背景上） |

### 1.5 表面与边框

| Token | 色值 | 用途 |
|---|---|---|
| `$card-bg` | `#ffffff` | 卡片背景 |
| `$border-color` | `#f0f0f0` | 分隔线、卡片边框 |
| `$border-color-light` | `#f0f0f0` | 更浅的分隔线（列表行内分隔） |
| `$input-border` | `#e5e5e5` | 输入框边框 |

---

## 2. 间距系统（8rpx 倍数）

| Token | 值 | 用途 |
|---|---|---|
| `$spacing-xs` | `8rpx` | 极小间距（图标与文字间隙） |
| `$spacing-sm` | `16rpx` | 小间距（标签间距、列表项间距） |
| `$spacing-md` | `24rpx` | 标准间距（页面边距、卡片内边距、卡片间距） |
| `$spacing-lg` | `32rpx` | 大间距（区块间距、列表水平内边距） |
| `$spacing-xl` | `48rpx` | 超大间距（底部留白） |

---

## 3. 圆角系统

| Token | 值 | 用途 |
|---|---|---|
| `$radius-sm` | `8rpx` | Tag 标签、小 badge |
| `$radius-md` | `16rpx` | 卡片、图标容器 |
| `$radius-lg` | `32rpx` | 次按钮描边、大卡片 |
| `$radius-pill` | `999rpx` | 胶囊按钮（主按钮、筛选按钮） |

---

## 4. 字号系统

| Token | 值 | 用途 |
|---|---|---|
| `$font-size-xs` | `20rpx` | 极小标注（角标、极小提示） |
| `$font-size-sm` | `24rpx` | 辅助文字（标签、日期、meta 信息） |
| `$font-size-md` | `28rpx` | 正文（列表项文字、输入框文字） |
| `$font-size-lg` | `32rpx` | 卡片标题、小标题 |
| `$font-size-xl` | `36rpx` | 页面标题（居中大标题） |

---

## 5. 组件规范

### 5.1 导航栏（appNavBar）

- 高度：`88rpx`（不含状态栏）
- 标题：居中，`$font-size-xl`（36rpx），font-weight bold
- 左侧返回按钮：`‹` 字符，`44rpx`
- 右侧图标：`44rpx`
- 背景：透明（渐变页面）或白色（普通页面）

### 5.2 卡片（appCard）

- 背景：`$card-bg`（#ffffff）
- 圆角：`$radius-md`（16rpx）
- 内边距：`$spacing-md`（24rpx）
- 外边距：`$spacing-md`（24rpx）
- 无阴影（设计稿中卡片无明显阴影）

### 5.3 主按钮（appButton type=primary）

- 背景：`linear-gradient(to bottom right, #2979ff, #5b9aff)` 或纯色 `#2979ff`（**方向只能写 `to` 关键字，禁 `deg`** —— ADR-0010）
- 文字：`$text-color-inverse`，`$font-size-lg`，font-weight bold
- 圆角：`$radius-pill`（999rpx 胶囊形）
- 高度：`80rpx`
- 禁用态：opacity 0.5

### 5.4 次按钮（appButton type=secondary）

- 背景：透明
- 边框：2rpx solid `$primary-color`
- 文字：`$primary-color`，`$font-size-md`
- 圆角：`$radius-lg`（32rpx）

### 5.5 筛选标签（appChip）

- 圆角：`$radius-pill`（胶囊）
- 未选中：`$bg-color` 浅灰底 + `$text-color-secondary` 文字
- 选中：`$primary-color` **实底** + `$text-color-inverse` 反白文字 + font-weight 600
- 内边距：`16rpx 32rpx`
- 字号：`$font-size-sm`（24rpx）

> 2026-09-15（#979）改版：原「白底描边 + 浅蓝底描边」两态在真机上就是一片白，选中态几乎看不出；
> 改为「浅灰底 / 主色实底」后，色彩只落在**当前档位**上。

### 5.6 Tab 栏（appTabs）

- underline 风格：文字下方 4rpx 蓝色指示条
- pill 风格：胶囊标签（同 appChip 选中态）
- 字号：`$font-size-md`（28rpx）
- 未选中：`$text-color-secondary`
- 选中：`$primary-color` + font-weight 600

### 5.7 列表项（appListItem）

- 高度：`~96rpx`
- 左侧：标签文字 `$font-size-md` `$text-color`
- 右侧：值文字 `$font-size-sm` `$text-placeholder` + 箭头 `›`
- 分隔线：底部 1rpx `$border-color`
- 危险项：文字 `$danger-color`

### 5.8 空状态（appEmptyState）

- 居中显示
- 图标：emoji `80rpx`
- 文字：`$font-size-md` `$text-placeholder`
- 操作按钮：可选，样式同 appButton

### 5.9 角标（appBadge）

- 背景：`$danger-color`
- 文字：`$text-inverse`，`$font-size-xs`
- 最小宽度：`32rpx`，高度 `32rpx`
- 圆角：`$radius-pill`

---

## 6. 页面结构模板

### 6.1 tabBar 页面（带渐变背景）

```html
<template>
  <view class="container" :style="{ height: windowHeight + 'px' }">
    <view class="status-bar" :style="{ height: statusBarHeight + 'px' }"></view>
    <appNavBar title="页面标题" />
    <scroll-view class="content-scroll" scroll-y>
      <!-- 内容 -->
    </scroll-view>
  </view>
</template>

<style>
.container {
  flex: 1;
  /* App 平台只支持 2 个颜色值、无百分比停靠（见 §1.2 注）；.uvue 不支持 CSS 变量，故写字面色值 */
  background: linear-gradient(180deg, #CFE9FB, #D0EBFD);
  background-color: #D0EBFD;
  flex-direction: column;
}
.status-bar { background-color: transparent; }
.content-scroll { flex: 1; }
</style>
```

### 6.2 普通页面（白色/灰色背景）

```html
<template>
  <view class="container" :style="{ height: windowHeight + 'px' }">
    <view class="status-bar" :style="{ height: statusBarHeight + 'px' }"></view>
    <appNavBar title="页面标题" />
    <scroll-view class="content-scroll" scroll-y>
      <!-- 内容 -->
    </scroll-view>
  </view>
</template>

<style>
.container {
  flex: 1;
  background-color: $bg-page;
  flex-direction: column;
}
.status-bar { background-color: transparent; }
.content-scroll { flex: 1; }
</style>
```

---

## 7. 禁止事项

1. **禁止硬编码色值**：所有颜色必须使用 `uni.scss` 中的 token 变量
2. **禁止硬编码间距**：间距必须使用 `$spacing-*` 系列变量
3. **禁止随意圆角**：圆角必须使用 `$radius-*` 系列变量
4. **禁止重复实现**：导航栏、卡片、按钮等公共组件必须使用共享组件
5. **禁止 px 单位**：除状态栏高度外，所有尺寸使用 rpx

---

## 8. 文件约定

- UI 规范文档：`docs/ui-spec.md`（本文件）
- Token 定义：`uni.scss`
- 全局样式：`App.uvue` 中的 `<style>` 块
- 共享组件：`components/app-*/` 目录
- 实施计划：`docs/ui-spec-implementation-plan.md`
