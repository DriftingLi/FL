<script setup lang="ts">
/**
 * 管理员管理（#1621 段4 立页；#1632 补齐新建与删除）。
 *
 * 一页三件事：**看**（每个管理员挂着哪个角色、是不是超管）、**改挂**（下拉选角色，立即保存）、
 * **新建/删除**（#1632 之前这两件事没有端点，界面上点不动）。
 *
 * 边界：**停用**不在本批（要给 admin 表加 status 列并让登录与能力解析都尊重它）；
 * 口令重置也不在（管理员改自己的口令走资料页，代重置要接会话吊销）。
 *
 * 接不住的动作由后端拒，前端不预判：把**最后一个超管**降级/删除、删自己、账号名重复
 * 都返回 409，文案经拦截器 toast 直接给到操作者（在前端重算这些不变式＝第二份真源）。
 */
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api/admin'
import type { AdminAccountDTO, AdminRoleDTO } from '@/api/generated/admin'
import { useAsyncPage } from '@/composables/useAsyncPage'
import { useConfirm } from '@/composables/useConfirm'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiFormField from '@/components/ui/UiFormField.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiTag from '@/components/ui/UiTag.vue'

defineOptions({ name: 'AdminAccountManage' })

const accounts = ref<AdminAccountDTO[]>([])
const roles = ref<AdminRoleDTO[]>([])
const savingId = ref<number | null>(null)

const { loading, loadError, retrying, isEmpty, run, retry } = useAsyncPage(
  // loader **只取数**（ADR-0069）：账号与角色并行拉，写回交给 apply 槽
  async () => {
    const [accountRes, roleRes] = await Promise.all([adminApi.listAdminAccounts(), adminApi.listAdminRoles()])
    return { accounts: accountRes?.accounts ?? [], roles: roleRes?.roles ?? [] }
  },
  {
    itemsRef: accounts,
    apply: res => {
      accounts.value = res?.accounts ?? []
      roles.value = res?.roles ?? []
    }
  }
)

onMounted(run)

const roleOptions = computed(() => roles.value.map(r => ({ value: r.role_id, label: r.name })))

/** 当前选中值：未挂角色的账号用 0 表示（el-select 需要一个具体值，null 会显示成占位符）。 */
function selectedRoleId(account: AdminAccountDTO): number {
  return account.role_id || 0
}

async function assign(account: AdminAccountDTO, roleId: number): Promise<void> {
  if (!roleId || roleId === account.role_id) return
  savingId.value = account.admin_id
  try {
    await adminApi.assignAdminRole(account.admin_id, roleId)
    ElMessage.success('已保存')
    await run()
  } finally {
    savingId.value = null
  }
}

// ===== 新建（#1632）=====

const createVisible = ref(false)
const creating = ref(false)
const form = reactive({ username: '', name: '', password: '', roleId: 0 })
/** 表单级错误（只做「填没填、长度够不够」这类入口前置校验；不变式一律由后端拒）。 */
const formError = ref('')

function openCreate(): void {
  form.username = ''
  form.name = ''
  form.password = ''
  form.roleId = 0
  formError.value = ''
  createVisible.value = true
}

/**
 * 新建管理员。口令长度规则（6-20 位）在这里是**入口前置提示**，
 * 判据本体仍在后端动作里（core.ValidatePasswordLength，ADR-0064 决策 4）——
 * 这里拦下只是为了不让一个明显不合规的口令白跑一趟。
 */
async function create(): Promise<void> {
  const username = form.username.trim()
  const name = form.name.trim()
  if (!username) {
    formError.value = '请填写账号'
    return
  }
  if (!name) {
    formError.value = '请填写姓名'
    return
  }
  if (form.password.length < 6 || form.password.length > 20) {
    formError.value = '口令长度需为 6-20 位'
    return
  }
  formError.value = ''
  creating.value = true
  try {
    await adminApi.createAdminAccount({ username, name, password: form.password, role_id: form.roleId })
    ElMessage.success('管理员已创建')
    createVisible.value = false
    await run()
  } finally {
    creating.value = false
  }
}

/** 删除管理员。确认框只问「删不删」，拦得住的是后端（409 文案由拦截器 toast）。 */
async function remove(account: AdminAccountDTO): Promise<void> {
  try {
    await useConfirm().confirm(`确定要删除管理员「${account.name || account.username}」吗？`, '提示', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning'
    })
  } catch {
    return
  }
  await adminApi.deleteAdminAccount(account.admin_id)
  ElMessage.success('已删除')
  await run()
}
</script>

<template>
  <!-- 单元素根：外壳的内层 <transition> 只能动画单根组件（多根会警告 non-element root 且没有进入动画） -->
  <div class="account-manage">
    <div class="account-actions">
      <UiButton type="primary" @click="openCreate">新建管理员</UiButton>
    </div>

    <UiAsyncSection
      :error="loadError"
      :loading="loading"
      :retrying="retrying"
      :empty="isEmpty"
      :skeleton="false"
      empty-title="还没有管理员"
      empty-description="用「新建管理员」建第一个，再给它挂角色"
      error-title="管理员列表加载失败"
      error-description="网络或服务端异常，可重试"
      @retry="retry"
    >
      <el-table v-loading="loading" :data="accounts" border>
        <el-table-column prop="admin_id" label="ID" width="80" align="center" />
        <el-table-column prop="username" label="账号" min-width="160" />
        <el-table-column prop="name" label="姓名" min-width="140" />
        <el-table-column label="身份" width="140">
          <template #default="{ row }">
            <UiTag v-if="row.protected" tone="warning">超级管理员</UiTag>
            <UiTag v-else-if="!row.role_id" tone="danger">未授权</UiTag>
            <UiTag v-else tone="neutral">普通管理员</UiTag>
          </template>
        </el-table-column>
        <el-table-column label="所挂角色" min-width="200">
          <template #default="{ row }">
            <el-select
              :model-value="selectedRoleId(row)"
              :loading="savingId === row.admin_id"
              placeholder="选择角色"
              class="account-role-select"
              @change="(value: number) => assign(row, value)"
            >
              <el-option v-for="role in roleOptions" :key="role.value" :label="role.label" :value="role.value" />
              <el-option v-if="!row.role_id" label="（未授权）" :value="0" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right" align="center">
          <template #default="{ row }">
            <UiButton size="small" @click="remove(row)">删除</UiButton>
          </template>
        </el-table-column>
      </el-table>
    </UiAsyncSection>

    <UiDialog
      v-model="createVisible"
      title="新建管理员"
      subtitle="账号建好后即出现在列表里，可在列表里给它挂角色"
      width="480px"
      :confirm-loading="creating"
      confirm-text="创建"
      @confirm="create"
    >
      <div class="account-form">
        <UiFormField label="账号" required for-id="admin-account-username">
          <UiInput id="admin-account-username" v-model="form.username" placeholder="登录用的账号名" />
        </UiFormField>
        <UiFormField label="姓名" required for-id="admin-account-name">
          <UiInput id="admin-account-name" v-model="form.name" placeholder="显示名" />
        </UiFormField>
        <UiFormField label="初始口令" required for-id="admin-account-password">
          <UiInput
            id="admin-account-password"
            v-model="form.password"
            type="password"
            placeholder="6-20 位"
            :maxlength="20"
          />
        </UiFormField>
        <UiFormField label="角色" for-id="admin-account-role">
          <UiSelect v-model="form.roleId" :options="roleOptions" placeholder="先不挂（未授权）" clearable />
        </UiFormField>
        <p v-if="formError" class="account-form-error" role="alert">{{ formError }}</p>
      </div>
    </UiDialog>
  </div>
</template>

<style scoped>
.account-manage {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
}

.account-actions {
  display: flex;
  justify-content: flex-end;
}

.account-role-select {
  width: 100%;
  max-width: 200px;
}

.account-form {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.account-form-error {
  margin: 0;
  font-size: var(--text-sm);
  color: var(--color-danger);
}
</style>
