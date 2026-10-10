<script setup lang="ts">
/**
 * 角色权限配置（#1621 段4）。
 *
 * 形态：一页列出全部管理角色，每个角色按**侧栏分组**勾选能力键；受保护角色（超级管理员）
 * 整卡只读（能力恒为受保护全集，不可改不可删）——防自锁第一层的界面侧。
 *
 * 可授权集合取自**接口返回的并集**（受保护角色贡献受保护全集，其余角色贡献各自已授权），
 * 不在此另抄一份能力清单：清单的事实源是后端 authz 能力表 → codegen → 运行时下发。
 *
 * 分组标题不按资源域（#1639）：能力键已与侧栏叶子一一对应，按域分组会让「课程管理」与
 * 「岗位管理」（同属 catalog 域）挤在一个标题下，超管看到的仍是一团。分组因此从
 * config/pages.ts 派生——组的顺序与标题都取侧栏那一份声明，这里不另抄。
 */
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api/admin'
import type { AdminRoleDTO } from '@/api/generated/admin'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCheckbox from '@/components/ui/UiCheckbox.vue'
import UiCheckboxGroup from '@/components/ui/UiCheckboxGroup.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { describeCapability } from '@/utils/capabilityWords'
import { navGroups, pages } from '@/config/pages'

/** 管理端侧栏分组：顺序与标题都以 config/pages.ts 的声明为准。 */
const MANAGE_NAV_GROUPS = navGroups.manage ?? []

/** 挂不到任何管理页上的能力键（数据导出/积分扣罚/举报处置/目录作者…）的落点。 */
const ACTION_GROUP = { key: '__action__', label: '动作能力（不占侧栏页面）' }

/**
 * 能力键 → 它所属管理页的侧栏分组 key。
 *
 * 由页面描述符派生而不是手写对照表：一页一键（#1639）之后这层映射就是侧栏本身，
 * 手写一份就会在下次改侧栏时静默漂移。只有带 nav 的页面参与——详情页（如内容精选编辑）
 * 与列表页共享同一个能力键，谁声明了 nav 谁就代表这个键在侧栏里的位置。
 */
const capabilityGroupKey: ReadonlyMap<string, string> = (() => {
  const map = new Map<string, string>()
  for (const page of pages) {
    if (page.workspace !== 'manage' || !page.nav || !page.capability) continue
    if (!map.has(page.capability)) map.set(page.capability, page.nav.group)
  }
  return map
})()

const roles = ref<AdminRoleDTO[]>([])
/** 每张卡的能力勾选草稿：role_id → 已勾选能力键（保存前的本地态）。 */
const draft = ref<Record<number, string[]>>({})
const savingId = ref<number | null>(null)
const newRoleName = ref('')

const { loading, loadError, retrying, isEmpty, run, retry } = useAsyncPage(
  // loader **只取数**（ADR-0069 决策 1 的锁：loader 内不得写页面 ref）
  () => adminApi.listAdminRoles(),
  {
    itemsRef: roles,
    // 写回走 apply 槽：它在代数校验之后执行，旧轮结果连 ref 都不碰
    apply: res => {
      roles.value = res?.roles ?? []
    }
  }
)

// 装载后把能力集灌进草稿（数组拷贝，避免直接改接口返回的对象）
async function load(): Promise<void> {
  await run()
  const next: Record<number, string[]> = {}
  for (const role of roles.value) next[role.role_id] = [...role.capabilities]
  draft.value = next
}

onMounted(load)

/**
 * 可授权集合 = 所有角色能力的并集（含受保护角色的受保护全集），按侧栏分组呈现。
 *
 * 展示名走 utils/capabilityWords（#1630）：收敛前这里把能力键与资源域**原样**印给超管看
 * （一屏英文）。组标题取侧栏分组的 label，勾选框取能力键的中文名，**组内排序也按中文名**，
 * 原始键留在 title 上以便对照后端日志。空组不渲染——没有任何角色持有该组能力时，
 * 一个只有标题的分组只会让界面变长。
 */
const capabilityGroups = computed(() => {
  const seen = new Set<string>()
  const keys: string[] = []
  for (const role of roles.value) {
    for (const key of role.capabilities) {
      if (seen.has(key)) continue
      seen.add(key)
      keys.push(key)
    }
  }
  const byLabel = (list: string[]): string[] =>
    list.slice().sort((a, b) => describeCapability(a).localeCompare(describeCapability(b), 'zh-Hans-CN'))
  const buckets = [
    ...MANAGE_NAV_GROUPS.map(g => ({
      key: g.key,
      label: g.label,
      keys: byLabel(keys.filter(k => capabilityGroupKey.get(k) === g.key))
    })),
    { ...ACTION_GROUP, keys: byLabel(keys.filter(k => !capabilityGroupKey.has(k))) }
  ]
  return buckets.filter(b => b.keys.length > 0)
})

function isDirty(role: AdminRoleDTO): boolean {
  const current = [...role.capabilities].sort().join(',')
  const edited = [...(draft.value[role.role_id] ?? [])].sort().join(',')
  return current !== edited
}

async function save(role: AdminRoleDTO): Promise<void> {
  savingId.value = role.role_id
  try {
    await adminApi.updateAdminRole(role.role_id, {
      name: role.name,
      remark: role.remark,
      capabilities: draft.value[role.role_id] ?? []
    })
    ElMessage.success('已保存')
    await load()
  } finally {
    savingId.value = null
  }
}

async function create(): Promise<void> {
  const name = newRoleName.value.trim()
  if (!name) {
    ElMessage.warning('请填写角色名')
    return
  }
  await adminApi.createAdminRole({ name, capabilities: [] })
  newRoleName.value = ''
  ElMessage.success('角色已创建')
  await load()
}

async function remove(role: AdminRoleDTO): Promise<void> {
  try {
    await useConfirm().confirm(`确定要删除角色「${role.name}」吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  await adminApi.deleteAdminRole(role.role_id)
  ElMessage.success('已删除')
  await load()
}
</script>

<template>
  <div class="role-manage">
    <div class="role-create">
      <UiInput v-model="newRoleName" placeholder="新角色名" class="role-create-input" />
      <UiButton type="primary" @click="create">新建角色</UiButton>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :retrying="retrying"
      :empty="isEmpty"
      :skeleton="false"
      empty-title="还没有角色"
      empty-description="新建角色后按侧栏分组勾选它能做的事"
      error-title="角色列表加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retry"
    >
      <section v-for="role in roles" :key="role.role_id" class="role-card">
        <header class="role-head">
          <span class="role-name">{{ role.name }}</span>
          <UiTag :tone="role.protected ? 'warning' : 'neutral'">
            {{ role.protected ? '受保护 · 超管' : `${role.capabilities.length} 项能力` }}
          </UiTag>
          <span v-if="role.remark" class="role-remark">{{ role.remark }}</span>
          <span class="role-actions">
            <template v-if="!role.protected">
              <UiButton size="small" :disabled="!isDirty(role)" :loading="savingId === role.role_id" @click="save(role)">
                保存
              </UiButton>
              <UiButton size="small" @click="remove(role)">删除</UiButton>
            </template>
          </span>
        </header>

        <!-- 受保护角色的能力恒为全集：勾选框只读，避免「勾了却没保存」的错觉 -->
        <div v-for="group in capabilityGroups" :key="group.key" class="cap-group">
          <div class="cap-group-title">{{ group.label }}</div>
          <UiCheckboxGroup v-model="draft[role.role_id]" :disabled="role.protected">
            <UiCheckbox v-for="key in group.keys" :key="key" :value="key" :label="describeCapability(key)">
              <span :title="key">{{ describeCapability(key) }}</span>
            </UiCheckbox>
          </UiCheckboxGroup>
        </div>
      </section>
    </UiAsyncSection>
  </div>
</template>

<style scoped>
.role-manage {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.role-create {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.role-create-input {
  max-width: 240px;
}

.role-card {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  padding: var(--space-4);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  background: var(--color-bg-card);
}

.role-head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}

.role-name {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
}

.role-remark {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.role-actions {
  display: flex;
  gap: var(--space-2);
  margin-left: auto;
}

.cap-group {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
}

.cap-group-title {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}
</style>
