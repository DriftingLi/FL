<script setup lang="ts">
/**
 * 顶栏（#1619 无缝外壳）：棋盘格的第一行。
 *
 * 结构（列宽 = 棋盘格的列）：
 *   [ 左格：身份 + 移动端汉堡 ] [ 中区：工作区控件插槽 ] [ 右区：工具位 ]
 * 左格宽度与侧栏**同源**（都用 --sidebar-width / --sidebar-collapsed-width），
 * 故侧栏收起时左格一并收窄（只剩头像）。
 *
 * 为什么身份区在顶栏而不在侧栏（ADR-0072）：外壳三块同底后，侧栏顶部再放一块用户区
 * 会与顶栏的左格重复；身份（头像+昵称）留在棋盘格左格，交互收进右区，一个入口一处职责。
 *
 * **改判（2026-10-28，ADR-0072 决策 4）**：主题入口从用户菜单里**独立**出来，作为一个图标
 * 按钮坐在通知铃铛旁（工具位）。原因是原形态把它藏进了「点开才看见」的菜单里：换外观是
 * **高频、随时可反悔的偏好切换**，与「退出登录」这种低频破坏性动作不同级，混在一处反而
 * 让人以为换主题也要先点开自己的头像。菜单里因此只剩身份只读行 + 退出登录。
 */
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import UiIcon from '@/components/ui/UiIcon.vue'
import { href } from '@/config/pages'
import { useAuthStore } from '@/stores/auth'
import { describeRole } from '@/utils/roleWords'
import { useConfirm } from '@/composables/useConfirm'
import NotificationPanel from './NotificationPanel.vue'
import ThemeToggle from '@/components/ui/ThemeToggle.vue'

const props = defineProps<{
  /** 侧栏是否收起（左格随之收窄；由 SidebarLayout 透传） */
  collapsed: boolean
}>()

defineEmits<{
  /** 移动端点汉堡：请求打开侧栏抽屉 */
  'open-mobile': []
}>()

const authStore = useAuthStore()
const router = useRouter()

const nickname = computed(
  () => authStore.userInfo?.username || authStore.userInfo?.account || '未登录'
)
const initial = computed(() => nickname.value.charAt(0) || '?')
const avatarUrl = computed(() =>
  authStore.userInfo?.avatar_url ? String(authStore.userInfo.avatar_url) : ''
)
// 角色徽标：从侧栏常驻位收进用户菜单（ADR-0072）——称谓仍取角色词表单点
const roleLabel = computed(() => describeRole(authStore.userInfo?.role))
// 徽标配色按角色分档（沿用侧栏身份区原有的四档语义色，只是搬进浮层）
const roleClass = computed(() => authStore.userInfo?.role || 'hrwai_user')
// 通知入口只对学员端显示（沿用侧栏底部的既有判据）
const showNotifications = computed(
  () => authStore.isLoggedIn && authStore.userInfo?.role === 'hrwai_user'
)

async function onCommand(command: string) {
  if (command === 'logout') {
    try {
      await useConfirm().confirm('确定要退出登录吗？', '提示', {
        confirmButtonText: '确定',
        cancelButtonText: '取消',
        type: 'warning'
      })
      // 登出单点：revoke + 清本地都在 store.signOut 里，这里只管 confirm 与落点
      await authStore.signOut()
      router.push(href('Login'))
    } catch {
      // 用户取消：不做任何操作
    }
  }
}
</script>

<template>
  <header class="app-topbar" :class="{ 'is-collapsed': props.collapsed }">
    <!-- 左格：身份（纯展示）+ 移动端汉堡 -->
    <div class="topbar-left">
      <button
        type="button"
        class="topbar-burger"
        aria-label="打开菜单"
        @click="$emit('open-mobile')"
      >
        <UiIcon name="menu" :size="20" />
      </button>
      <img v-if="avatarUrl" :src="avatarUrl" class="topbar-avatar" alt="头像" />
      <span v-else class="topbar-avatar is-fallback">{{ initial }}</span>
      <span class="topbar-nickname" :title="nickname">{{ nickname }}</span>
    </div>

    <!-- 中区：工作区控件（学员端放证件切换器；其余端为空） -->
    <div class="topbar-center">
      <slot name="center" />
    </div>

    <!-- 右区：工具位 -->
    <div class="topbar-actions">
      <!-- 主题入口（改判，见文件顶部）：高频偏好切换，独立成图标按钮，与铃铛同级 -->
      <ThemeToggle variant="topbar" />
      <NotificationPanel v-if="showNotifications" />

      <el-dropdown trigger="click" placement="bottom-end" @command="onCommand">
        <button type="button" class="topbar-menu-btn" aria-label="用户菜单">
          <UiIcon name="user" :size="18" />
          <UiIcon name="chevron-down" class="topbar-menu-arrow" :size="12" />
        </button>
        <template #dropdown>
          <el-dropdown-menu>
            <div class="topbar-identity-row">
              <span class="topbar-identity-name">{{ nickname }}</span>
              <span class="topbar-role-badge" :class="roleClass">{{ roleLabel }}</span>
            </div>
            <el-dropdown-item divided command="logout">
              <span class="topbar-menu-item is-danger">
                <UiIcon name="log-out" :size="14" />
                退出登录
              </span>
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </header>
</template>

<style scoped>
/* 棋盘格第一行：左格宽 = 侧栏宽（同一组变量），中区弹性，右区自适应。
   整条顶栏与页面同底、无下边框 —— 「无感接缝」由同底承担，不用分隔线。 */
.app-topbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: var(--topbar-height);
  display: grid;
  grid-template-columns: var(--sidebar-width) minmax(0, 1fr) auto;
  align-items: center;
  background: var(--color-bg-page);
  /* 十字细线的**横线**（#1629）：顶栏下沿 1px 分界线；与侧栏右缘的竖线交叉成十字。
     这是对 ADR-0072「完全无缝」的**改判**：同色保留，但边界用一条细线明示。
     线色走 --color-border-seam（#1646）：接缝比通用分隔线清晰一档 */
  border-bottom: 1px solid var(--color-border-seam);
  z-index: var(--z-fixed);
}

.app-topbar.is-collapsed {
  grid-template-columns: var(--sidebar-collapsed-width) minmax(0, 1fr) auto;
}

/* 左格 */
.topbar-left {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: 0 var(--space-4);
  min-width: 0;
  height: 100%;
  /* 十字细线的**竖线**在顶栏内的那一段：与侧栏右边界连成一条（同一条缝、同一个线色） */
  border-right: 1px solid var(--color-border-seam);
}

.app-topbar.is-collapsed .topbar-left {
  justify-content: center;
  padding: 0 var(--space-2);
}

.topbar-avatar {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-full);
  object-fit: cover;
  flex-shrink: 0;
}

.topbar-avatar.is-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--gradient-brand);
  color: white;
  font-size: var(--text-sm);
  font-weight: var(--font-bold);
}

.topbar-nickname {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.app-topbar.is-collapsed .topbar-nickname {
  display: none;
}

/* 汉堡只在移动端出现（桌面端侧栏常驻，无需唤出入口） */
.topbar-burger {
  display: none;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  flex-shrink: 0;
}

.topbar-burger:hover {
  background: var(--color-bg-card);
  color: var(--color-primary-soft-ink);
}

/* 中区：与内容列左边缘对齐（内容区左右各 --space-6 内边距） */
.topbar-center {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: 0 var(--space-6);
  min-width: 0;
  /* 兜底：中区内容再高也不许溢出顶栏（溢出＝没有底色的文字压在内容上） */
  overflow: hidden;
}

/* 右区 */
.topbar-actions {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  padding-right: var(--space-6);
}

.topbar-menu-btn {
  display: flex;
  align-items: center;
  gap: 2px;
  height: 36px;
  padding: 0 var(--space-2);
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background var(--duration-fast) var(--ease-default);
}

.topbar-menu-btn:hover {
  background: var(--color-bg-card);
  color: var(--color-primary-soft-ink);
}

.topbar-menu-arrow {
  opacity: 0.7;
}

/* 菜单内：身份只读行 + 分组标题 + 选项 */
.topbar-identity-row {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-1) var(--space-4) var(--space-2);
  min-width: 160px;
}

.topbar-identity-name {
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-role-badge {
  flex-shrink: 0;
  padding: 1px var(--space-2);
  border-radius: var(--radius-full);
  background: var(--color-primary-soft);
  color: var(--color-primary-soft-ink);
  font-size: var(--text-xs);
}

.topbar-role-badge.student,
.topbar-role-badge.recruiter {
  background: var(--color-primary-soft);
  color: var(--color-primary-soft-ink);
}

.topbar-role-badge.tutor {
  background: var(--color-success-light);
  color: var(--color-success-strong);
}

.topbar-role-badge.admin {
  background: var(--color-violet-soft);
  color: var(--color-violet-soft-ink);
}

.topbar-menu-item {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
}

.topbar-menu-item.is-danger {
  color: var(--color-danger);
}

@media screen and (max-width: 768px) {
  /* 移动端：左格不再绑定侧栏宽度（侧栏是抽屉），汉堡接管唤出入口 */
  .app-topbar,
  .app-topbar.is-collapsed {
    grid-template-columns: auto minmax(0, 1fr) auto;
  }

  .topbar-burger {
    display: flex;
  }

  .topbar-center {
    padding: 0 var(--space-2);
  }

  .topbar-actions {
    padding-right: var(--space-3);
  }
}
</style>
