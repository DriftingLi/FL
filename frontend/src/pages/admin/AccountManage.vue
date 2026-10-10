<script setup lang="ts">
/**
 * 管理员管理（#1621 段4）。
 *
 * 一页两件事：**看**（每个管理员挂着哪个角色、是不是超管）与**改挂**（下拉选角色，立即保存）。
 *
 * 边界：新建/停用/改口令不在本批（新建要接口令写面，停用要给 admin 表加 status 列并让登录
 * 与能力解析都尊重它）——那时本页再加「新建」与「停用」两个动作。
 *
 * 接不住的动作由后端拒：把**最后一个超管**改挂成普通角色会返回 409，文案经拦截器 toast
 * 直接给到操作者（前端不预判「谁是最后一个」——那需要在前端重算一遍服务端不变式）。
 */
import { computed, onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api/admin'
import type { AdminAccountDTO, AdminRoleDTO } from '@/api/generated/admin'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
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

const roleOptions = computed(() => roles.value.map(r => ({ id: r.role_id, label: r.name })))

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
</script>

<template>
  <UiAsyncSection
    :error="loadError"
    :loading="loading"
    :retrying="retrying"
    :empty="isEmpty"
    :skeleton="false"
    empty-title="还没有管理员"
    empty-description="管理员账号由后端初始化"
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
            <el-option v-for="role in roleOptions" :key="role.id" :label="role.label" :value="role.id" />
            <el-option v-if="!row.role_id" label="（未授权）" :value="0" />
          </el-select>
        </template>
      </el-table-column>
    </el-table>
  </UiAsyncSection>
</template>

<style scoped>
.account-role-select {
  width: 100%;
  max-width: 200px;
}
</style>
