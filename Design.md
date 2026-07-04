# 和润天下设计风格规范 (Design.md)

> 适用于：和润天下人工智能科技有限公司官网与所有 ForkLift Pro 业务前端
> 维护者：前端组
> 最近更新：2026-07-04

---

## 一、品牌识别

### 1.1 公司信息
- **公司全称**：和润天下人工智能科技有限公司
- **品牌名（中）**：和润天下
- **品牌名（英）**：HRWAI
- **品牌口号**：AI赋能叉车行业 · 用AI让每一台叉车的价值透明可见

### 1.2 Logo 规范
官网 Logo 采用 36×36 圆角矩形（`rx=8`）+ 房屋图标的组合，使用蓝绿渐变填充：

```
<svg width="36" height="36" viewBox="0 0 36 36" fill="none">
  <rect width="36" height="36" rx="8" fill="url(#portal-logo-grad)"/>
  <path d="M10 26V18L18 10L26 18V26" stroke="#fff" stroke-width="2.5"
        stroke-linecap="round" stroke-linejoin="round" fill="none"/>
  <path d="M15 26V21H21V26" stroke="#fff" stroke-width="2"
        stroke-linecap="round" stroke-linejoin="round"/>
  <defs>
    <linearGradient id="portal-logo-grad" x1="0" y1="0" x2="36" y2="36">
      <stop stop-color="#0EA5E9"/>
      <stop offset="1" stop-color="#14B8A6"/>
    </linearGradient>
  </defs>
</svg>
```

学员端继续沿用 ForkLift Pro 三角+圆点 Logo，但渐变色同步改为蓝绿渐变（`#0EA5E9 → #14B8A6`）。

---

## 二、色彩系统

### 2.1 主色 - Sky 蓝绿
基于 Tailwind Sky 色板，主色 `#0EA5E9`（sky-500），传递科技感与专业感。

| Token | 值 | 用途 |
|-------|-----|------|
| `--color-primary-50` | `#F0F9FF` | 浅色背景 |
| `--color-primary-100` | `#E0F2FE` | 选中态背景 |
| `--color-primary-200` | `#BAE6FD` | 边框、Hero 副标色 |
| `--color-primary-300` | `#7DD3FC` | 浅色禁用态 |
| `--color-primary-400` | `#38BDF8` | 渐变中点 |
| `--color-primary-500` | `#0EA5E9` | **主色** - 按钮、链接、强调 |
| `--color-primary-600` | `#0284C7` | Hover 态 |
| `--color-primary-700` | `#0369A1` | Active 态 |
| `--color-primary-800` | `#075985` | 选中文字色 |
| `--color-primary-900` | `#0C4A6E` | 深色文本 |

### 2.2 强调色 - Teal 青
基于 Tailwind Teal 色板，强调色 `#14B8A6`（teal-500），与主色形成渐变。

| Token | 值 |
|-------|-----|
| `--color-accent-50` | `#F0FDFA` |
| `--color-accent-100` | `#CCFBF1` |
| `--color-accent-200` | `#99F6E4` |
| `--color-accent-300` | `#5EEAD4` |
| `--color-accent-400` | `#2DD4BF` |
| `--color-accent-500` | `#14B8A6` |
| `--color-accent-600` | `#0D9488` |
| `--color-accent-700` | `#0F766E` |

### 2.3 中性色 - Slate
基于 Tailwind Slate 色板，用于文本、背景、边框。

| Token | 值 | 用途 |
|-------|-----|------|
| `--color-text-primary` | `#0F172A` | 标题、正文主色 |
| `--color-text-secondary` | `#475569` | 次要文本 |
| `--color-text-tertiary` | `#64748B` | 描述、辅助文本 |
| `--color-text-muted` | `#94A3B8` | 占位、Footer 文字 |
| `--color-text-inverse` | `#FFFFFF` | 深色背景上的文字 |
| `--color-text-on-dark` | `#F1F5F9` | 深色区块文字 |
| `--color-bg-page` | `#F8FAFC` | 页面背景 |
| `--color-bg-card` | `#FFFFFF` | 卡片背景 |
| `--color-bg-sidebar` | `#0F172A` | 侧边栏/深色区块 |
| `--color-border` | `#E2E8F0` | 默认边框 |
| `--color-border-light` | `#F1F5F9` | 浅色边框 |
| `--color-border-dark` | `#CBD5E1` | 深色边框 |
| `--color-border-darker` | `#334155` | 深色区块边框 |
| `--surface-dark` | `#0F172A` | 创始人区块/Footer 背景 |
| `--surface-dark-alt` | `#1E293B` | 二维码占位方块 |
| `--surface-card-alt` | `#F8FAFC` | 替代卡片背景 |

### 2.4 语义色
| Token | 值 | 用途 |
|-------|-----|------|
| `--color-success` | `#10B981` | 成功提示 |
| `--color-warning` | `#F59E0B` | 警告提示 |
| `--color-danger` | `#EF4444` | 错误/危险 |
| `--color-info` | `#6B7280` | 中性提示 |

### 2.5 渐变
| Token | 值 | 用途 |
|-------|-----|------|
| `--gradient-brand` | `linear-gradient(135deg, #0EA5E9 0%, #14B8A6 100%)` | 品牌 Logo、主按钮 |
| `--gradient-hero` | `linear-gradient(135deg, #0F172A 0%, #075985 50%, #0284C7 100%)` | 学员端 Hero |
| `--gradient-accent` | `linear-gradient(135deg, #0D9488 0%, #14B8A6 100%)` | 强调按钮 |
| `--gradient-card` | `linear-gradient(180deg, rgba(14,165,233,0.04) 0%, rgba(255,255,255,0) 100%)` | 卡片底纹 |
| `--gradient-sidebar` | `linear-gradient(180deg, #0F172A 0%, #1E293B 100%)` | 侧边栏 |
| `--gradient-hero-overlay` | `linear-gradient(135deg, rgba(14,165,233,0.85) 0%, rgba(20,184,166,0.75) 100%)` | 官网 Hero 遮罩 |
| `--gradient-dark-section` | `linear-gradient(180deg, #0F172A 0%, #1E293B 100%)` | 深色区块 |
| `--gradient-cta-banner` | `linear-gradient(135deg, #0EA5E9 0%, #14B8A6 60%, #0D9488 100%)` | 官网 CTA Banner |

---

## 三、字体系统

### 3.1 字体族
| Token | 字体栈 | 用途 |
|-------|--------|------|
| `--font-display` | `'DM Sans', -apple-system, BlinkMacSystemFont, sans-serif` | 标题、数字 |
| `--font-body` | `'Noto Sans SC', -apple-system, 'PingFang SC', 'Microsoft YaHei', sans-serif` | 正文 |
| `--font-mono` | `'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace` | 代码、等宽 |

> 字体通过 Google Fonts CDN 加载（见 `index.html`），中文优先 Noto Sans SC，英文优先 DM Sans。

### 3.2 字号阶梯
| Token | 值 | 用途 |
|-------|-----|------|
| `--text-xs` | 12px | 版权、标签 |
| `--text-sm` | 14px | 辅助文本、按钮 |
| `--text-base` | 16px | 正文 |
| `--text-lg` | 18px | 副标题 |
| `--text-xl` | 20px | 卡片标题 |
| `--text-2xl` | 24px | 区块副标 |
| `--text-3xl` | 30px | 区块主标题 |
| `--text-4xl` | 36px | 数据数字 |
| `--text-5xl` | 48px | Hero 大标题 |

### 3.3 字重与行高
- 字重：`--font-normal: 400`、`--font-medium: 500`、`--font-semibold: 600`、`--font-bold: 700`
- 行高：`--leading-tight: 1.25`、`--leading-normal: 1.5`、`--leading-relaxed: 1.75`
- 字间距：`-0.025em`（标题）、`0`（正文）、`0.05em`（全大写副标）

---

## 四、间距与圆角

### 4.1 间距体系（4px 基础）
| Token | 值 | 用途 |
|-------|-----|------|
| `--space-1` | 4px | 微间距 |
| `--space-2` | 8px | 紧凑间距 |
| `--space-3` | 12px | 默认小间距 |
| `--space-4` | 16px | 默认间距 |
| `--space-5` | 20px | 卡片内边距 |
| `--space-6` | 24px | 区块内边距 |
| `--space-8` | 32px | 大间距 |
| `--space-10` | 40px | 卡片间距 |
| `--space-12` | 48px | 区块内大间距 |
| `--space-16` | 64px | 区块标题与内容间距 |
| `--space-20` | 80px | CTA 内边距 |
| `--section-padding` | 6rem (96px) | 区块上下内边距 |
| `--section-padding-mobile` | 3rem (48px) | 移动端区块内边距 |

### 4.2 圆角
| Token | 值 | 用途 |
|-------|-----|------|
| `--radius-sm` | 6px | 小元素 |
| `--radius-md` | 8px | 按钮、输入框 |
| `--radius-lg` | 12px | 卡片 |
| `--radius-xl` | 16px | 大卡片、模态框 |
| `--radius-2xl` | 20px | 特殊容器 |
| `--radius-full` | 9999px | 圆形、胶囊 |

---

## 五、阴影层级

**设计原则**：静态表面以边框为主、阴影极弱；只有浮层（dropdown / modal / toast）使用较强阴影。

| Token | 值 | 用途 |
|-------|-----|------|
| `--shadow-xs` | `0 1px 2px rgba(0,0,0,0.05)` | 静态卡片 |
| `--shadow-sm` | `0 1px 3px rgba(0,0,0,0.1), 0 1px 2px rgba(0,0,0,0.06)` | 默认卡片 |
| `--shadow-md` | `0 4px 6px -1px rgba(0,0,0,0.1), 0 2px 4px -1px rgba(0,0,0,0.06)` | 悬浮卡片 |
| `--shadow-lg` | `0 10px 15px -3px rgba(0,0,0,0.1), 0 4px 6px -2px rgba(0,0,0,0.05)` | Hover 态卡片 |
| `--shadow-xl` | `0 20px 25px -5px rgba(0,0,0,0.1), 0 10px 10px -5px rgba(0,0,0,0.04)` | 抽屉、下拉 |
| `--shadow-glow` | `0 0 20px rgba(14,165,233,0.3)` | 主色辉光 |

---

## 六、动效规范

### 6.1 过渡时长
| Token | 时长 | 用途 |
|-------|------|------|
| `--duration-fast` | 150ms | Hover、点击反馈 |
| `--duration-normal` | 250ms | 默认过渡 |
| `--duration-slow` | 350ms | 大区块显隐 |

### 6.2 缓动函数
- `--ease-default: cubic-bezier(0.4, 0, 0.2, 1)` - 默认
- `--ease-in: cubic-bezier(0.4, 0, 1, 1)` - 进入
- `--ease-out: cubic-bezier(0, 0, 0.2, 1)` - 离开
- `--ease-spring: cubic-bezier(0.34, 1.56, 0.64, 1)` - 弹性

### 6.3 Keyframe 动画
`global.css` 提供 4 个预置动画工具类：

- `.animate-fade-in-up` - 从下方淡入上移（350ms）
- `.animate-fade-in` - 淡入（250ms）
- `@keyframes pulse` - 脉动（用于加载/通知）
- `@keyframes float` - 上下浮动（用于装饰元素）

### 6.4 滚动交互
官网导航栏滚动行为：
- 初始透明背景（在 Hero 上方显示白色文字）
- `window.scrollY > 80` 时切换为 `var(--surface-dark)` 深色背景 + 阴影
- 锚点点击通过 `scrollIntoView({ behavior: 'smooth' })` 平滑滚动
- 实时高亮当前所在区块对应的导航项

---

## 七、组件视觉规范

### 7.1 按钮
- **主按钮（`.btn-primary`）**：`--gradient-brand` 背景、白字、`--radius-md` 圆角、min-height 44px
- **次按钮（`.btn-outline`）**：透明背景、白色描边 2px、Hover 时半透明白底
- **线框按钮（`.btn-coop`）**：`--color-primary-500` 描边、Hover 时填充主色
- **CTA 按钮（`.btn-cta`）**：白色描边、半透明白底 Hover
- 所有按钮：min-height 44px（移动端可达 48px）、`--duration-fast` 过渡、Hover 微上移 `translateY(-1px)`

### 7.2 卡片
- 默认：`--color-bg-card` 背景、`--color-border` 边框、`--shadow-sm` 阴影
- Hover：阴影升级到 `--shadow-lg`、`translateY(-2px)` 上浮
- 圆角：`--radius-lg`（12px）
- 内边距：`--space-8`（32px）

### 7.3 区块标题
所有 section 标题使用统一结构：
- 居中对齐
- `--text-3xl` 字号、`--font-bold` 字重
- 下方 3px 渐变下划线（`--gradient-brand`）
- 与下方内容间距 `--space-16`

### 7.4 图标
- 服务卡片图标：48×48 圆角方块，`--color-primary-50` 底色 + `--color-primary-500` 图标色
- 合作卡片图标：56×56 圆角方块，同上
- 服务保障编号：48×48 圆形，`--color-primary-50` 底色 + `--color-primary-500` 数字
- 全局图标统一使用 Element Plus 图标或内联 SVG，不引入第三方图标库

### 7.5 导航栏（官网专属）
- `position: fixed`，z-index `--z-sticky`
- 透明 → 深色滚动切换（80px 阈值）
- 桌面端：Logo + 6 个锚点 + 登录/工作台 CTA
- 移动端（<768px）：汉堡菜单 + 抽屉式展开
- 活动锚点高亮：底部 2px 渐变下划线

---

## 八、响应式策略

### 8.1 断点
| 断点 | 范围 | 布局变化 |
|------|------|----------|
| 桌面 | `≥1024px` | 4×2 服务保障网格、4 列 Footer、3 列合作卡 |
| 平板 | `640px ~ 1023px` | 2×4 服务保障网格、2 列 Footer、2 列合作卡 |
| 手机 | `<640px` | 2×4 服务保障网格、1 列 Footer、1 列卡片 |

### 8.2 字号自适应
- Hero 标题：`clamp(2.5rem, 6vw, 4.5rem)`
- Hero 副标：`clamp(var(--text-base), 2vw, var(--text-lg))`
- CTA 标题：`clamp(var(--text-2xl), 4vw, var(--text-4xl))`
- 768px 以下 html `font-size: 14px`，480px 以下 `font-size: 13px`（已有 global.css 适配）

### 8.3 容器宽度
- 默认容器：`--container-page: 1280px`
- 窄容器（创始人/CTA）：`--container-narrow: 960px`
- 内边距：`--space-6`（24px），移动端 `--space-4`（16px）

### 8.4 移动端适配要点
- Element Plus 组件在 768px 以下自动适配（按钮 min-height 44px、对话框宽度 92% 等，见 `global.css`）
- Hero CTA 按钮在 640px 以下垂直堆叠、全宽
- 滚动条 6px 宽、自动隐藏

---

## 九、代码约定

### 9.1 样式书写
- **优先使用 CSS 变量**：颜色、间距、字号、圆角、阴影、动画时长全部引用 `var(--xxx)`，不硬编码
- **`<style scoped>`**：组件样式作用域隔离
- **BEM 命名**：如 `hero-section`、`service-card`、`nav-link`、`nav-link--active`
- **响应式查询**：统一使用 `@media screen and (max-width: 1024px / 768px / 480px)` 与 `@media (min-width: 768px)` 两种方向

### 9.2 Element Plus 主题
通过 `element-overrides.css` 中的 CSS 变量覆盖：
- `--el-color-primary` 及 light/dark 系对齐 `--color-primary-500`
- 圆角、字号、边框色、填充色全部映射到设计令牌
- 主按钮使用 `--gradient-brand` 渐变背景

### 9.3 资源引用
- 静态图片放 `frontend/public/images/`，引用路径 `/images/xxx.jpg`
- 字体走 Google Fonts CDN（`index.html` 中 `<link>`）
- SVG 图标内联使用，不引入第三方图标库（lucide / heroicons 等）
- 业务图片（用户上传、教学资源）走后端 `/static/` 接口，前端用 `fileUrl.ts` 拼接

### 9.4 设计令牌维护
- 所有令牌集中在 `frontend/src/assets/styles/design-tokens.css`
- 新增令牌需注释用途
- 修改主色后需同步更新 `element-overrides.css` 中的 `--el-color-primary-*` 系列
- 内嵌 SVG 渐变 `stop-color` 需手动同步（无法用 CSS 变量）

---

## 十、参考资源

- **设计稿**：`首页.html`（用户提供，含 Hero/About/Founder/Products/Cooperation/Service/CTA/Footer 9 个区块）
- **设计令牌源文件**：[design-tokens.css](file:///d:/叉车维修项目/frontend/src/assets/styles/design-tokens.css)
- **Element 主题覆盖**：[element-overrides.css](file:///d:/叉车维修项目/frontend/src/assets/styles/element-overrides.css)
- **全局样式**：[global.css](file:///d:/叉车维修项目/frontend/src/assets/styles/global.css)
- **官网首页**：[PortalHome.vue](file:///d:/叉车维修项目/frontend/src/pages/portal/PortalHome.vue)
- **官网导航**：[PortalNavbar.vue](file:///d:/叉车维修项目/frontend/src/components/layout/PortalNavbar.vue)
- **官网页脚**：[PortalFooter.vue](file:///d:/叉车维修项目/frontend/src/components/layout/PortalFooter.vue)
- **Tailwind 色板参考**：https://tailwindcss.com/docs/customizing-colors
