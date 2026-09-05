<!--
  AuthPageShell：认证页公共外壳（方案 D「主次分离」视觉底座）。
  主次分离：白底极简卡片 + 角色 badge + 主表单（密码为主）+「或使用以下方式登录」分隔线 + 图标按钮切换。
  切换方式：点击图标按钮/返回链接更新 activeAlt，当前为空(null)=主方式，非空=对应 alt 方式视图。
  slot：main（主表单）、#alt-key（各 alt 方式表单）、footer（页脚链接区）。
  样式：模板走原子类（R1），scoped 仅保留局部变量、badge 动态分类色、divider 伪元素。
-->
<template>
  <div class="auth-page flex min-h-screen items-center justify-center bg-panel p-6 max-[480px]:p-4">
    <div
      class="auth-card w-full max-w-[420px] rounded-card border border-line bg-panel px-10 pt-11 pb-7 shadow-[0_1px_2px_rgba(15,23,42,0.04),0_8px_24px_-8px_rgba(15,23,42,0.08)] max-[480px]:px-[22px] max-[480px]:pt-8 max-[480px]:pb-[22px]"
    >
      <!-- 明暗模式切换（三态），与 SidebarLayout 同构。认证页是独立布局、不走 SidebarLayout，
           曾漏装导致深色偏好用户在登录页看到崩坏画面且无法切回（#554），故补于此。 -->
      <button
        class="theme-toggle fixed right-4 top-4 z-[var(--z-sticky)] flex h-9 w-9 cursor-pointer items-center justify-center rounded-pill border border-line bg-panel text-ink shadow-raised transition-[background,box-shadow,transform] duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:bg-canvas active:scale-[0.94]"
        :aria-label="themeLabel"
        :title="themeLabel"
        @click="themeStore.cycle()"
      >
        <el-icon v-if="themeStore.resolved === 'dark'" :size="18"><Moon /></el-icon>
        <svg v-else-if="themeStore.mode === 'system'" class="theme-half-icon h-[18px] w-[18px]" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 3a9 9 0 0 0 0 18V3Z" fill="currentColor" opacity="0.9"/>
          <path d="M12 6a6 6 0 0 0 0 12 8 8 0 0 1 0-12Z" fill="var(--color-bg-card)"/>
          <path d="M12 3a9 9 0 0 1 9 9 9 9 0 0 1-9 9V3Z" fill="none" stroke="currentColor" stroke-width="1.5"/>
          <path d="M16.8 6.2l1.4-1.4M21 12h2M16.8 17.8l1.4 1.4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
        </svg>
        <el-icon v-else :size="18"><Sunny /></el-icon>
      </button>

      <header class="card-header mb-5 text-center">
        <span
          class="badge mb-3.5 inline-block rounded-pill px-3 py-1 text-xs font-semibold tracking-[0.02em]"
          :class="`badge-${badgeTone}`"
        >{{ badgeText }}</span>
        <h1 class="title m-0 mb-2 text-2xl font-bold tracking-[-0.02em] text-ink">{{ title }}</h1>
        <p class="subtitle m-0 text-sm leading-normal text-ink-3">{{ subtitle }}</p>
      </header>

      <main v-show="activeAlt === null" class="view mt-1">
        <slot name="main" />
      </main>

      <main
        v-for="m in altModes"
        :key="m.key"
        v-show="activeAlt === m.key"
        class="view alt-view mt-1 w-full"
      >
        <button
          type="button"
          class="back inline-block cursor-pointer border-0 bg-transparent p-0 pb-4 text-[13px] text-ui-600 transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] hover:text-ui-700"
          @click="goMain"
        >← {{ backLabel }}</button>
        <slot :name="`alt-${m.key}`" />
      </main>

      <section v-if="altModes.length" class="alt-area mt-[22px]">
        <div class="divider mb-4 flex items-center gap-2.5 text-xs text-ink-muted">{{ dividerText }}</div>
        <div class="alt-row flex justify-center gap-[22px]">
          <button
            v-for="m in altModes"
            :key="m.key"
            type="button"
            class="alt group flex cursor-pointer flex-col items-center gap-1.5 border-0 bg-transparent p-1 text-xs transition-colors duration-[var(--duration-fast)] ease-[var(--ease-default)] aria-pressed:font-semibold"
            :class="m.key === 'wechat' ? 'aria-pressed:text-[var(--wechat-green)]' : 'aria-pressed:text-ui-600'"
            :aria-pressed="activeAlt === m.key"
            @click="selectAlt(m.key)"
          >
            <span
              class="ic flex h-11 w-11 items-center justify-center rounded-card border border-line bg-panel text-ink-3 transition-all duration-[var(--duration-fast)] ease-[var(--ease-default)] group-hover:border-ui-300 group-hover:text-ui-600"
              :class="m.key === 'wechat'
                ? 'group-aria-pressed:border-[var(--wechat-green)] group-aria-pressed:bg-[var(--wechat-green)] group-aria-pressed:text-white group-aria-pressed:shadow-[0_4px_10px_rgba(7,193,96,0.28)]'
                : 'group-aria-pressed:border-ui-500 group-aria-pressed:bg-ui-500 group-aria-pressed:text-white group-aria-pressed:shadow-[0_4px_10px_rgba(13,148,136,0.28)]'"
            >
              <component :is="m.icon" :size="19" />
            </span>
            <span>{{ m.label }}</span>
          </button>
        </div>
      </section>

      <footer v-if="$slots.footer" class="card-footer mt-6 text-center">
        <slot name="footer" />
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import { Moon, Sunny } from '@element-plus/icons-vue'
import { useThemeStore } from '@/stores/theme'

export type AltModeKey = 'email' | 'phone' | 'wechat'

export interface AltMode {
  key: AltModeKey
  label: string
  icon: Component
}

const props = withDefaults(
  defineProps<{
    title: string
    subtitle: string
    badgeText: string
    badgeTone: 'student' | 'tutor' | 'admin'
    /** alt 方式图标按钮列表；空数组则不渲染分隔线与图标按钮区 */
    altModes?: AltMode[]
    /** 当前激活的 alt 方式；null = 主方式 */
    activeAlt?: AltModeKey | null
    /** alt 视图顶部「← 返回 xxx」文案（默认「返回密码登录」） */
    backLabel?: string
    /** 分隔线文案（默认「或使用以下方式登录」） */
    dividerText?: string
  }>(),
  {
    altModes: () => [],
    activeAlt: null,
    backLabel: '返回密码登录',
    dividerText: '或使用以下方式登录'
  }
)

const emit = defineEmits<{
  (e: 'select-alt', key: AltModeKey | null): void
}>()

const themeStore = useThemeStore()

const themeLabel = computed(() => {
  if (themeStore.mode === 'system') return '跟随系统（点击切换为浅色）'
  return themeStore.resolved === 'dark' ? '深色模式（点击切换为跟随系统）' : '浅色模式（点击切换为深色）'
})

const altModes = props.altModes

function selectAlt(key: AltModeKey) {
  emit('select-alt', key)
}

function goMain() {
  emit('select-alt', null)
}
</script>

<style scoped>
/* R1 允许保留的三类：组件局部变量、动态分类色、伪元素。其余样式已全部原子化。 */
.auth-page {
  /* 微信品牌绿：品牌恒定色，深浅底上都成立、不随主题翻转。
     用组件局部变量承载，供模板 bg-[var(--wechat-green)] 引用。 */
  --wechat-green: #07c160;
}

/* badge 分类色底：color-mix 透明叠底（写死 rgba 无法跟随深浅主题切换） */
.badge-student {
  background: color-mix(in srgb, var(--color-accent-500) 10%, transparent);
  color: var(--color-accent-700);
}

.badge-tutor {
  background: color-mix(in srgb, var(--color-primary-500) 10%, transparent);
  color: var(--color-primary-700);
}

.badge-admin {
  background: color-mix(in srgb, var(--color-violet-500) 10%, transparent);
  color: var(--color-violet-500);
}

.divider::before,
.divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--color-border);
}
</style>

<!--
  ===== 认证表单共享样式（非 scoped，#569）=====
  slot 注入内容携带的是页面自己的 scopeId，外壳的 scoped 规则永远命不中
  （这正是 #569 的根因：这批规则自 2026 年迁移时起从未生效，#568 已删除死代码）。
  因此这里用「非 scoped + .auth-page 根前缀」：既穿透 slot，又不会泄漏到其他页面
  （类名 code-row/captcha-img/footer-link 等在 profile 弹窗等处也有同名，但都不在
  .auth-page 内，天然隔离）。

  与旧死代码的三处刻意差异：
  1. 色值全部走 design-tokens token（#554 原则），深浅主题自动跟随
  2. 不再覆盖 .auth-btn 的 --el-button-* 颜色变量 —— 主按钮由 UiButton primary
     走全站统一的品牌渐变（element-overrides），覆盖回纯色反而是倒退
  3. 输入框焦点环用 color-mix 品牌色，替代旧的 rgba(37,99,235) 蓝色残留
-->
<style>
.auth-page .auth-form {
  margin-top: 4px;
}

.auth-page .code-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.auth-page .code-input {
  flex: 1;
}

.auth-page .code-btn {
  min-width: 124px;
}

.auth-page .captcha-row {
  display: flex;
  gap: 10px;
  width: 100%;
  align-items: center;
}

.auth-page .captcha-input {
  flex: 1;
}

.auth-page .captcha-img {
  height: 40px;
  width: 110px;
  border-radius: 8px;
  border: 1px solid var(--color-border);
  cursor: pointer;
  background: var(--color-bg-page);
  flex-shrink: 0;
}

/* 主按钮：仅布局形态；颜色走 UiButton primary 的全站品牌渐变 */
.auth-page .auth-btn {
  width: 100%;
  height: 48px;
  font-size: 16px;
  font-weight: 600;
  border-radius: 12px;
  letter-spacing: 0.08em;
  margin-top: 8px;
}

.auth-page .form-input .el-input__wrapper {
  border-radius: 12px;
  padding: 4px 14px;
  box-shadow: 0 0 0 1px var(--color-border) inset;
  transition: all var(--duration-base) var(--ease-default);
  background: var(--color-bg-page);
}

.auth-page .form-input .el-input__wrapper:hover {
  box-shadow: 0 0 0 1px var(--color-border-dark) inset;
  background: var(--color-bg-card);
}

.auth-page .form-input .el-input__wrapper.is-focus {
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--color-primary-500) 20%, transparent) inset;
  background: var(--color-bg-card);
}

.auth-page .form-input .el-input__prefix-inner {
  color: var(--color-text-muted);
}

.auth-page .form-footer {
  text-align: center;
}

.auth-page .footer-text {
  font-size: 14px;
  color: var(--color-text-muted);
}

.auth-page .footer-link {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-primary-500);
  text-decoration: none;
  margin-left: 4px;
  transition: color var(--duration-fast) var(--ease-default);
}

.auth-page .footer-link:hover {
  color: var(--color-primary-700);
}
</style>
