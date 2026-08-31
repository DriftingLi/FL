<template>
  <div class="tutor-manage-page">
    <div class="page-header">
      <h2>导师管理</h2>
      <UiButton variant="primary" @click="openAddDialog">
        <el-icon><Plus /></el-icon> 新增导师
      </UiButton>
    </div>

    <div class="filter-bar">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索用户名或姓名"
        clearable
        style="width: 280px"
        @clear="search"
        @keyup.enter="search"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <UiButton variant="primary" @click="search">搜索</UiButton>
    </div>

    <el-table :data="list" v-loading="loading" stripe border style="width: 100%">
      <el-table-column prop="tutor_id" label="ID" width="70" align="center" />
      <el-table-column prop="username" label="用户名" min-width="160" />
      <el-table-column prop="name" label="姓名" min-width="140" />
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="200" align="center">
        <template #default="{ row }">
          {{ formatDateTime(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90" fixed="right" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
            <UiButton variant="primary" link size="small">
              操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </UiButton>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="resetPwd">重置密码</el-dropdown-item>
                <el-dropdown-item command="toggle">{{ row.status === 1 ? '禁用' : '启用' }}</el-dropdown-item>
                <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </template>
      </el-table-column>
    </el-table>

    <div class="pagination-wrapper" v-if="total > pageSize">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @size-change="load"
        @current-change="load"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      title="新增导师"
      width="480px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formData" :rules="formRules" label-width="100px">
        <el-form-item label="用户名" prop="username">
          <el-input v-model="formData.username" placeholder="请输入用户名（3-20字符）" maxlength="20" show-word-limit />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="formData.password" type="password" placeholder="请输入密码（6-20字符）" maxlength="20" show-password />
        </el-form-item>
        <el-form-item label="姓名" prop="name">
          <el-input v-model="formData.name" placeholder="请输入姓名（2-10字符）" maxlength="10" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <UiButton @click="dialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="submitting" @click="handleSubmit">确认添加</UiButton>
      </template>
    </el-dialog>

    <!-- 重置密码弹窗 -->
    <el-dialog
      v-model="pwdDialogVisible"
      title="重置密码"
      width="440px"
      destroy-on-close
    >
      <el-form ref="pwdFormRef" :model="pwdFormData" :rules="pwdFormRules" label-width="90px">
        <el-form-item label="导师">
          <span>{{ pwdFormData.name }}</span>
        </el-form-item>
        <el-form-item label="新密码" prop="password">
          <el-input v-model="pwdFormData.password" type="password" placeholder="请输入新密码（6-20字符）" maxlength="20" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <UiButton @click="pwdDialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="pwdSubmitting" @click="handleResetPwd">确认重置</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus, Search, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { adminApi, type AdminTutor } from '@/api/admin'
import { useAdminTable } from '@/composables/useAdminTable'
import { formatDateTime } from '@/utils/format'
import { usernameRules, passwordRules, nameRules } from '@/utils/validate'
import UiButton from '@/components/ui/UiButton.vue'

type TutorRow = AdminTutor

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const formData = reactive({
  username: '',
  password: '',
  name: ''
})

const formRules: FormRules = {
  username: usernameRules,
  password: passwordRules,
  name: nameRules
}

// 重置密码弹窗
const pwdDialogVisible = ref(false)
const pwdSubmitting = ref(false)
const pwdFormRef = ref<FormInstance>()
const pwdFormData = reactive({
  id: 0,
  name: '',
  password: ''
})

const pwdFormRules: FormRules = {
  password: passwordRules
}

async function handleToggleStatus(row: TutorRow) {
  try {
    await adminApi.toggleTutorStatus(row.tutor_id)
    ElMessage.success(row.status === 1 ? '已禁用' : '已启用')
    load()
  } catch (error) {
    console.error('切换状态失败:', error)
  }
}

async function handleDelete(tutorId: number) {
  try {
    await adminApi.deleteTutor(tutorId)
    ElMessage.success('导师已删除')
    load()
  } catch (error) {
    console.error('删除导师失败:', error)
    /* 错误已由拦截器提示 */
  }
}

// admin 列表状态机：本页只声明 fetch 与行操作 adapter
const table = useAdminTable<AdminTutor>({
  fetch: async (paging, filters) => {
    const data = await adminApi.getTutors({
      page: paging.page,
      page_size: paging.pageSize,
      keyword: filters.keyword ? String(filters.keyword) : undefined
    })
    return { list: data?.tutors || [], total: data?.total || 0 }
  },
  actions: {
    resetPwd: openResetPwdDialog,
    toggle: handleToggleStatus,
    delete: deleteRow
  }
})
const { loading, list, total, currentPage, pageSize, searchKeyword, load, search, handleAction } = table

function deleteRow(row: AdminTutor): Promise<void> {
  return table.confirmDelete(row, async r => {
    await handleDelete(r.tutor_id)
  }, '确定删除该导师？删除后不可恢复')
}

function openAddDialog() {
  formData.username = ''
  formData.password = ''
  formData.name = ''
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()

  submitting.value = true
  try {
    await adminApi.addTutor({
      username: formData.username,
      password: formData.password,
      name: formData.name
    })
    ElMessage.success('导师添加成功')
    dialogVisible.value = false
    load()
  } catch (error) {
    console.error('添加导师失败:', error)
  } finally {
    submitting.value = false
  }
}

function openResetPwdDialog(row: TutorRow) {
  pwdFormData.id = row.tutor_id
  pwdFormData.name = row.name
  pwdFormData.password = ''
  pwdDialogVisible.value = true
}

async function handleResetPwd() {
  if (!pwdFormRef.value) return
  await pwdFormRef.value.validate()

  pwdSubmitting.value = true
  try {
    await adminApi.resetTutorPassword(pwdFormData.id, pwdFormData.password)
    ElMessage.success('密码已重置')
    pwdDialogVisible.value = false
  } catch (error) {
    console.error('重置密码失败:', error)
  } finally {
    pwdSubmitting.value = false
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.tutor-manage-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 22px;
  color: var(--color-text-primary);
}

.filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

@media screen and (max-width: 768px) {
  .tutor-manage-page {
    padding: 12px;
  }

  .page-header {
    flex-wrap: wrap;
    gap: 8px;
  }

  .page-header h2 {
    font-size: 18px;
  }

  .filter-bar {
    flex-direction: column;
    gap: 8px;
  }

  .filter-bar .el-input {
    width: 100% !important;
  }

  .el-table {
    overflow-x: auto;
  }
}
</style>
