<script setup lang="ts">
/**
 * 讲师角色面板（#2599 / 规格 E7）。
 *
 * 为什么单独成组件而不塞进 RoleManage.vue 的卡片循环：管理角色与讲师角色是**两条平行的链**
 * （各自一张表、各自一套能力词表、各自的守卫键），共用一套卡片循环会让「这排勾选框改的是
 * 管理员还是讲师」只能靠标题分辨。这里保留同一套视觉语法（卡片 + 按组勾选 + 受保护只读），
 * 但数据源与可授权集合各自独立。
 *
 * 可授权集合取自**接口返回的并集**（受保护的全权讲师贡献讲师侧 7 键全集），不在此另抄一份
 * 能力清单：清单的事实源是后端 authz 能力表 → codegen → 运行时下发。同理，这里也不自己
 * 判断「哪些键属于讲师侧」——后端已经拒收管理员侧键（400），前端把返回的键如实画出来即可。
 */
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi, type AdminTutorRole } from '@/api/admin'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCheckbox from '@/components/ui/UiCheckbox.vue'
import UiCheckboxGroup from '@/components/ui/UiCheckboxGroup.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'
import { describeCapability } from '@/utils/capabilityWords'

const roles = ref<AdminTutorRole[]>([])
/** 每张卡的能力勾选草稿：role_id → 已勾选能力键（保存前的本地态）。 */
const draft = ref<Record<number, string[]>>({})
const savingId = ref<number | null>(null)
const newRoleName = ref('')

const { loading, loadError, retrying, isEmpty, run, retry } = useAsyncPage(
  // loader 只取数（ADR-0069 决策 1 的锁：loader 内不得写页面 ref）
  () => adminApi.listTutorRoles(),
  {
    itemsRef: roles,
    apply: res => {
      roles.value = res?.roles ?? []
    }
  }
)

async function load(): Promise<void> {
  await run()
  const next: Record<number, string[]> = {}
  for (const role of roles.value) next[role.role_id] = [...role.capabilities]
  draft.value = next
}

onMounted(load)

/**
 * 可授权集合 = 所有讲师角色能力的并集，按中文名排序。
 *
 * 不分组（与管理端那排不同）：讲师侧只有 7 键、且没有「侧栏分组」可挂——讲师端侧栏
 * 是平铺一列（navGroups 没有 tutor 分组），硬套管理端的分组只会造出几个只有一项的标题。
 */
const capabilityKeys = computed(() => {
  const seen = new Set<string>()
  for (const role of roles.value) {
    for (const key of role.capabilities) seen.add(key)
  }
  return [...seen].sort((a, b) => describeCapability(a).localeCompare(describeCapability(b), 'zh-Hans-CN'))
})

function isDirty(role: AdminTutorRole): boolean {
  const current = [...role.capabilities].sort().join(',')
  const edited = [...(draft.value[role.role_id] ?? [])].sort().join(',')
  return current !== edited
}

async function save(role: AdminTutorRole): Promise<void> {
  savingId.value = role.role_id
  try {
    await adminApi.updateTutorRole(role.role_id, {
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
  await adminApi.createTutorRole({ name, capabilities: [] })
  newRoleName.value = ''
  ElMessage.success('角色已创建')
  await load()
}

async function remove(role: AdminTutorRole): Promise<void> {
  try {
    await useConfirm().confirm(`确定要删除讲师角色「${role.name}」吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  await adminApi.deleteTutorRole(role.role_id)
  ElMessage.success('已删除')
  await load()
}
</script>

<template>
  <section class="tutor-role-panel">
    <header class="panel-head">
      <h3>讲师角色</h3>
      <span class="panel-hint">
        挂到讲师账号上的角色。受保护的全权讲师能力恒为讲师侧全集，不可改不可删。
      </span>
    </header>

    <div class="role-create">
      <UiInput v-model="newRoleName" placeholder="新讲师角色名" class="role-create-input" />
      <UiButton type="primary" @click="create">新建角色</UiButton>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :retrying="retrying"
      :empty="isEmpty"
      :skeleton="false"
      empty-title="还没有讲师角色"
      empty-description="新建角色后勾选它能做的事"
      error-title="讲师角色加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retry"
    >
      <section v-for="role in roles" :key="role.role_id" class="role-card">
        <header class="role-head">
          <span class="role-name">{{ role.name }}</span>
          <UiTag :tone="role.protected ? 'warning' : 'neutral'">
            {{ role.protected ? '受保护 · 全权' : `${role.capabilities.length} 项能力` }}
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
        <UiCheckboxGroup v-model="draft[role.role_id]" :disabled="role.protected" class="cap-grid">
          <UiCheckbox v-for="key in capabilityKeys" :key="key" :value="key">
            <span :title="key">{{ describeCapability(key) }}</span>
          </UiCheckbox>
        </UiCheckboxGroup>
      </section>
    </UiAsyncSection>
  </section>
</template>

<style scoped>
.tutor-role-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  margin-top: var(--space-6);
}

.panel-head {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
}

.panel-head h3 {
  font-size: var(--text-lg);
  color: var(--color-text-primary);
}

.panel-hint {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
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

.cap-grid {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-1) var(--space-4);
}
</style>
