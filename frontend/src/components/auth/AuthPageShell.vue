<!--
  AuthPageShell：认证页公共外壳（方案 D「主次分离」视觉底座）。
  主次分离：白底极简卡片 + 角色 badge + 主表单（密码为主）+「或使用以下方式登录」分隔线 + 图标按钮切换。
  切换方式：点击图标按钮/返回链接更新 activeAlt，当前为空(null)=主方式，非空=对应 alt 方式视图。
  slot：main（主表单）、#alt-key（各 alt 方式表单）、footer（页脚链接区）。
-->
<template>
  <div class="auth-page">
    <div class="auth-card">
      <header class="card-header">
        <span class="badge" :class="`badge-${badgeTone}`">{{ badgeText }}</span>
        <h1 class="title">{{ title }}</h1>
        <p class="subtitle">{{ subtitle }}</p>
      </header>

      <main v-show="activeAlt === null" class="view">
        <slot name="main" />
      </main>

      <main
        v-for="m in altModes"
        :key="m.key"
        v-show="activeAlt === m.key"
        class="view alt-view"
      >
        <button type="button" class="back" @click="goMain">← {{ backLabel }}</button>
        <slot :name="`alt-${m.key}`" />
      </main>

      <section v-if="altModes.length" class="alt-area">
        <div class="divider">{{ dividerText }}</div>
        <div class="alt-row">
          <button
            v-for="m in altModes"
            :key="m.key"
            type="button"
            class="alt"
            :class="[`alt-${m.key}`, { active: activeAlt === m.key }]"
            :aria-pressed="activeAlt === m.key"
            @click="selectAlt(m.key)"
          >
            <span class="ic"><component :is="m.icon" :size="19" /></span>
            <span>{{ m.label }}</span>
          </button>
        </div>
      </section>

      <footer v-if="$slots.footer" class="card-footer">
        <slot name="footer" />
      </footer>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Component } from 'vue'

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
  (e: 'update:activeAlt', key: AltModeKey | null): void
}>()

const altModes = props.altModes

function selectAlt(key: AltModeKey) {
  emit('select-alt', key)
  emit('update:activeAlt', key)
}

function goMain() {
  emit('select-alt', null)
  emit('update:activeAlt', null)
}
</script>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
  padding: 24px;
}

.auth-card {
  width: 100%;
  max-width: 420px;
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  box-shadow:
    0 1px 2px rgba(15, 23, 42, 0.04),
    0 8px 24px -8px rgba(15, 23, 42, 0.08);
  padding: 44px 40px 28px;
}

.card-header {
  text-align: center;
  margin-bottom: 20px;
}

.badge {
  display: inline-block;
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.02em;
  margin-bottom: 14px;
}

.badge-student {
  background: rgba(37, 99, 235, 0.08);
  color: #1d4ed8;
}

.badge-tutor {
  background: rgba(13, 148, 136, 0.08);
  color: #0f766e;
}

.badge-admin {
  background: rgba(124, 58, 237, 0.08);
  color: #6d28d9;
}

.title {
  font-size: 24px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
  letter-spacing: -0.02em;
}

.subtitle {
  font-size: 14px;
  color: #64748b;
  margin: 0;
  line-height: 1.5;
}

.view {
  margin-top: 4px;
}

.alt-view {
  width: 100%;
}

.back {
  display: inline-block;
  border: none;
  background: none;
  padding: 0 0 16px;
  font-size: 13px;
  color: #2563eb;
  cursor: pointer;
}

.back:hover {
  color: #1d4ed8;
}

.alt-area {
  margin-top: 22px;
}

.divider {
  display: flex;
  align-items: center;
  gap: 10px;
  color: #94a3b8;
  font-size: 12px;
  margin-bottom: 16px;
}

.divider::before,
.divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: #e2e8f0;
}

.alt-row {
  display: flex;
  justify-content: center;
  gap: 22px;
}

.alt {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  background: none;
  border: none;
  padding: 4px;
  color: #64748b;
  font-size: 12px;
}

.alt .ic {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: #fff;
  border: 1px solid #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #64748b;
  transition: all 0.15s;
}

.alt:hover .ic {
  border-color: #93c5fd;
  color: #2563eb;
}

.alt.active .ic {
  background: #2563eb;
  border-color: #2563eb;
  color: #fff;
  box-shadow: 0 4px 10px rgba(37, 99, 235, 0.28);
}

.alt.active {
  color: #2563eb;
  font-weight: 600;
}

.alt-wechat.active .ic {
  background: #07c160;
  border-color: #07c160;
}

.alt-wechat.active {
  color: #07c160;
}

.card-footer {
  text-align: center;
  margin-top: 24px;
}

@media screen and (max-width: 480px) {
  .auth-page {
    padding: 16px;
  }

  .auth-card {
    padding: 32px 22px 22px;
    border-radius: 12px;
  }

  .title {
    font-size: 21px;
  }
}
</style>
