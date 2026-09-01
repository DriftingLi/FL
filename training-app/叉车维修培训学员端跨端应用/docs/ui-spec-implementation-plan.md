# 移动端 UI 规范建立与页面适配 — 实施计划

> 基于 Figma 设计稿（5 个 tabBar 页面）+ 现有 52 个页面代码分析

---

## 现状总结

| 维度 | 现状 |
|---|---|
| 框架 | uni-app X（Vue 3 + UTS），`.uvue` 组件 |
| 样式 | 纯 CSS，无预处理器，无 Tailwind |
| Token | `uni.scss` 定义了 token 但**几乎未使用**（页面全部硬编码色值） |
| 组件 | 仅 `components/tab-bar/` 一个共享组件，其余全部重复手写 |
| 页面数 | 52 个，tabBar 一级页面 5 个 |
| 适配 | rpx 单位 + 自定义状态栏 + safeArea 处理 |

---

## Phase 1：UI 规范文档 + Token 清理

### 1.1 编写 `docs/ui-spec.md`

基于 Figma 设计稿提取的规范：

**色彩体系**
| Token | 色值 | 用途 |
|---|---|---|
| `$primary-color` | `#2979ff` | 主色（按钮、选中态、Tab 高亮） |
| `$primary-gradient-start` | `#CFE9FB` | 页面渐变顶部 |
| `$primary-gradient-mid` | `#D0EBFD` | 页面渐变中部 |
| `$bg-page` | `#F5F5F5` | 页面渐变底部 |
| `$card-bg` | `#FFFFFF` | 卡片背景 |
| `$text-color` | `#333333` | 主文字 |
| `$text-secondary` | `#666666` | 次要文字 |
| `$text-placeholder` | `#999999` | 辅助文字 |
| `$text-disabled` | `#CCCCCC` | 禁用文字 |
| `$border-color` | `#F0F0F0` | 卡片分隔线/边框 |
| `$danger-color` | `#FF3B30` | 危险操作（退出登录等） |

**间距系统（统一为 8rpx 倍数）**
| Token | 值 | 用途 |
|---|---|---|
| `$spacing-xs` | `8rpx` | 极小间距 |
| `$spacing-sm` | `16rpx` | 小间距（标签间距等） |
| `$spacing-md` | `24rpx` | 标准间距（页面边距、卡片内边距、卡片间距） |
| `$spacing-lg` | `32rpx` | 大间距（区块间距） |
| `$spacing-xl` | `48rpx` | 超大间距（底部留白） |

**圆角系统**
| Token | 值 | 用途 |
|---|---|---|
| `$radius-sm` | `8rpx` | Tag 标签 |
| `$radius-md` | `16rpx` | 卡片、图标容器 |
| `$radius-lg` | `32rpx` | 次按钮描边 |
| `$radius-pill` | `999rpx` | 胶囊按钮 |

**字号系统**
| Token | 值 | 用途 |
|---|---|---|
| `$font-size-xs` | `20rpx` | 极小标注（角标文字） |
| `$font-size-sm` | `24rpx` | 辅助文字、标签、日期 |
| `$font-size-md` | `28rpx` | 正文 |
| `$font-size-lg` | `32rpx` | 卡片标题、小标题 |
| `$font-size-xl` | `36rpx` | 页面标题（居中） |

**组件规范**
| 组件 | 规范 |
|---|---|
| 导航栏 | 高度 88rpx，标题居中 36rpx 加粗，左右图标 44rpx |
| 卡片 | 白底、16rpx 圆角、24rpx 内边距、无阴影 |
| 主按钮 | 渐变蓝底、白色文字、40rpx 圆角、高度 80rpx |
| 次按钮 | 蓝色描边、蓝色文字、32rpx 圆角 |
| Tab 栏 | 底部固定，5 项，图标+文字，选中态蓝色 |
| Tag | 8rpx 圆角、浅蓝底/浅灰底、24rpx 字号 |
| 列表项 | 高度 ~96rpx，左侧文字，右侧箭头 › |

### 1.2 清理 `uni.scss`

- 删除未使用的 `$uni-*` 标准变量（uni-app 内置会自动提供）
- 保留并重命名业务 token 为清晰语义
- 补齐缺失 token（gradient、shadow、divider）
- 确保所有 token 使用 rpx 单位

---

## Phase 2：核心共享组件提取

从 52 个页面的重复模式中提取组件，放入 `components/` 目录。

### 2.1 组件清单（8 个）

| 组件 | 文件名 | 用途 | 当前重复位置 |
|---|---|---|---|
| `appNavBar` | `components/app-nav-bar/app-nav-bar.uvue` | 自定义导航栏 | 每个页面 |
| `appCard` | `components/app-card/app-card.uvue` | 白色圆角卡片容器 | 大量页面 |
| `appButton` | `components/app-button/app-button.uvue` | 主色/次要/禁用按钮 | 多个页面 |
| `appChip` | `components/app-chip/app-chip.uvue` | 筛选标签（选中态） | dashboard、courses、forum |
| `appTabs` | `components/app-tabs/app-tabs.uvue` | Tab 切换栏 | dashboard、forum、courses |
| `appListItem` | `components/app-list-item/app-list-item.uvue` | 设置项行（标签+值+箭头） | profile、settings |
| `appEmptyState` | `components/app-empty-state/app-empty-state.uvue` | 空状态（emoji+文字+操作） | 多个列表页 |
| `appBadge` | `components/app-badge/app-badge.uvue` | 未读数角标 | dashboard、notifications |

### 2.2 组件 Props 设计

#### `appNavBar`
```uts
defineProps<{
  title?: string        // 导航栏标题
  leftIcon?: string     // 左侧图标文字（默认 ‹）
  rightIcon?: string    // 右侧图标文字
  transparent?: boolean // 是否透明背景（渐变页面用）
}>()
defineEmits(['leftClick', 'rightClick'])
```

#### `appCard`
```uts
defineProps<{
  padding?: string      // 内边距（默认 24rpx）
  margin?: string       // 外边距（默认 24rpx）
  radius?: string       // 圆角（默认 16rpx）
  bgColor?: string      // 背景色（默认 #FFFFFF）
}>()
```

#### `appButton`
```uts
defineProps<{
  text?: string
  type?: 'primary' | 'secondary' | 'danger' | 'ghost'
  size?: 'large' | 'medium' | 'small'
  disabled?: boolean
  loading?: boolean
}>()
defineEmits(['click'])
```

#### `appChip`
```uts
defineProps<{
  text?: string
  active?: boolean
  size?: 'normal' | 'small'
}>()
defineEmits(['click'])
```

#### `appTabs`
```uts
defineProps<{
  tabs?: string[]
  activeIndex?: number
  style?: 'underline' | 'pill'
}>()
defineEmits(['change'])
```

#### `appListItem`
```uts
defineProps<{
  label?: string
  value?: string
  avatar?: string
  showArrow?: boolean
  danger?: boolean
}>()
defineEmits(['click'])
```

#### `appEmptyState`
```uts
defineProps<{
  icon?: string         // emoji
  text?: string
  actionText?: string   // 操作按钮文字
}>()
defineEmits(['action'])
```

#### `appBadge`
```uts
defineProps<{
  count?: number
  dot?: boolean         // 小圆点模式
}>()
```

---

## Phase 3：页面逐步迁移

按优先级逐页将硬编码 CSS 替换为 token + 组件。

### P0 — tabBar 一级页面（用户接触最多）

| 页面 | 文件 | 主要改动 |
|---|---|---|
| 首页 | `pages/dashboard/dashboard.uvue` | 渐变背景、卡片、Tab、Chip、Badge 全部替换 |
| 课程 | `pages/courses/courses.uvue` | 卡片、Chip、Tab、列表项替换 |
| AI学 | `pages/ai-assistant/version-select.uvue` | 卡片、按钮替换 |
| 交流 | `pages/forum/forum.uvue` | 卡片、Tab、帖子列表替换 |
| 我的 | `pages/profile/profile.uvue` | 用户卡片、统计卡片、菜单列表替换 |

**每个页面的改动内容：**
1. 硬编码色值 → token 引用（如 `#2979ff` → `$primary-color`）
2. 重复 DOM 结构 → 组件替换（如导航栏 → `<appNavBar>`）
3. 内联样式 → 组件 props
4. 重复 CSS 类删除

### P1 — 高频二级页面

| 页面 | 文件 |
|---|---|
| 课程详情 | `pages/courses/course-detail.uvue` |
| 章节学习 | `pages/courses/chapter-view.uvue` |
| 论坛详情 | `pages/forum/forum-detail.uvue` |
| 发布帖子 | `pages/forum/forum-create.uvue` |
| 考试 | `pages/exam/exam.uvue` |
| 模拟考试 | `pages/exam/mock-exam.uvue` |
| 题库练习 | `pages/practice/practice.uvue` |

### P2 — 中频页面

| 页面 | 文件 |
|---|---|
| 设置 | `pages/profile/settings.uvue` |
| 个人信息 | `pages/profile/personal-info.uvue` |
| 消息通知 | `pages/notifications/notifications.uvue` |
| 搜索 | `pages/search/search.uvue` |
| 签到 | `pages/forum/check-in.uvue` |

### P3 — 其余页面

剩余 ~20 个低频页面逐步替换。

---

## Phase 4：一致性验证

### 4.1 硬编码色值扫描

全局搜索并替换以下硬编码值：

| 硬编码值 | 替换为 Token |
|---|---|
| `#2979ff` | `$primary-color` |
| `#5b9aff` | `$primary-color-light` |
| `#1c9eff` | `$primary-color-dark` |
| `#333333` / `#333` | `$text-color` |
| `#666666` / `#666` | `$text-secondary` |
| `#999999` / `#999` | `$text-placeholder` |
| `#cccccc` / `#ccc` | `$text-disabled` |
| `#f8f8f8` / `#F5F5F5` | `$bg-page` |
| `#ffffff` / `#FFFFFF` | `$card-bg` |
| `#e5e5e5` / `#e0e0e0` | `$border-color` |
| `#F0F0F0` | `$border-color` |

### 4.2 border-radius 统一

当前混用 8rpx~48rpx，统一为：
- 卡片：16rpx（`$radius-md`）
- Tag：8rpx（`$radius-sm`）
- 按钮：40rpx（`$radius-pill`）
- 大卡片/Modal：24rpx（`$radius-lg`）

### 4.3 字号统一

当前从 20rpx 到 44rpx 有多种值，统一为 5 级：
- xs: 20rpx → sm: 24rpx → md: 28rpx → lg: 32rpx → xl: 36rpx

---

## 实施顺序

```
Phase 1（基础）
  ├── 1.1 编写 docs/ui-spec.md
  └── 1.2 清理 uni.scss

Phase 2（组件）
  ├── 2.1 appNavBar
  ├── 2.2 appCard
  ├── 2.3 appButton
  ├── 2.4 appChip
  ├── 2.5 appTabs
  ├── 2.6 appListItem
  ├── 2.7 appEmptyState
  └── 2.8 appBadge

Phase 3（迁移）
  ├── P0: 5 个 tabBar 页面
  ├── P1: 7 个高频页面
  ├── P2: 5 个中频页面
  └── P3: 其余页面

Phase 4（验证）
  ├── 4.1 色值扫描
  ├── 4.2 圆角统一
  └── 4.3 字号统一
```

---

## 注意事项

1. **uni-app X 组件限制**：`.uvue` 组件使用 UTS 语法，defineProps/defineEmits 与标准 Vue 3 略有差异
2. **样式隔离**：`styleIsolationVersion: "2"` 已开启，组件样式默认隔离
3. **rpx 单位**：所有尺寸必须使用 rpx，不能用 px（除状态栏高度外）
4. **渐变背景**：tabBar 页面特有的渐变背景通过 `appCard` 或页面 `.container` class 控制
5. **emoji 图标**：设计稿中使用 emoji 作为图标，暂不引入图标库
