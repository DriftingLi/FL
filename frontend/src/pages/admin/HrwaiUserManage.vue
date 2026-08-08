<script setup lang="ts">
// HRWAI 用户管理(管理员后台)
// 对应后端 /api/admin/hrwai-users/*(统一管理 hrwai_users 表)
// 合并原学员管理与评估用户管理,支持分页列表 + 关键词搜索 + 新增 + 编辑 + 重置密码 + 启用/禁用 + 删除
import { ref, reactive, onMounted } from 'vue'
import { Plus, Search, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { adminApi, type HrwaiUser } from '@/api/admin'
import { phoneRules, passwordRules, nameRules, emailRules, companyRules } from '@/utils/validate'

const loading = ref(false)
const users = ref<HrwaiUser[]>([])
const searchKeyword = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

// 新增弹窗
const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const formData = reactive({
  phone: '',
  password: '',
  name: '',
  email: '',
  company: ''
})

// 重置密码弹窗
const pwdDialogVisible = ref(false)
const pwdSubmitting = ref(false)
const pwdFormRef = ref<FormInstance>()
const pwdFormData = reactive({
  id: 0,
  name: '',
  password: ''
})

const formRules: FormRules = {
  phone: phoneRules,
  password: passwordRules,
  name: nameRules,
  email: emailRules,
  company: companyRules
}

const pwdFormRules: FormRules = {
  password: passwordRules
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

async function loadUsers() {
  loading.value = true
  try {
    const data = await adminApi.getHrwaiUsers({
      page: currentPage.value,
      page_size: pageSize.value,
      keyword: searchKeyword.value || undefined
    })
    users.value = data?.list ?? []
    total.value = data?.total ?? 0
  } catch (error) {
    console.error('加载 HRWAI 用户列表失败:', error)
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  currentPage.value = 1
  loadUsers()
}

function openCreateDialog() {
  Object.assign(formData, {
    phone: '',
    password: '',
    name: '',
    email: '',
    company: ''
  })
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()

  submitting.value = true
  try {
    const created = await adminApi.createHrwaiUser({
      phone: formData.phone,
      password: formData.password,
      name: formData.name,
      email: formData.email || undefined,
      company: formData.company || undefined
    })
    const nickname = created?.nickname
    ElMessage.success(nickname ? `用户添加成功，默认昵称：${nickname}` : '用户添加成功')
    dialogVisible.value = false
    loadUsers()
  } catch (error) {
    // 错误提示已由 request 拦截器处理
    console.error('保存 HRWAI 用户失败:', error)
  } finally {
    submitting.value = false
  }
}

function openResetPwdDialog(row: HrwaiUser) {
  pwdFormData.id = row.id
  pwdFormData.name = row.name
  pwdFormData.password = ''
  pwdDialogVisible.value = true
}

async function handleResetPwd() {
  if (!pwdFormRef.value) return
  await pwdFormRef.value.validate()

  pwdSubmitting.value = true
  try {
    await adminApi.resetHrwaiUserPassword(pwdFormData.id, pwdFormData.password)
    ElMessage.success('密码已重置')
    pwdDialogVisible.value = false
  } catch (error) {
    console.error('重置密码失败:', error)
  } finally {
    pwdSubmitting.value = false
  }
}

async function handleToggleStatus(row: HrwaiUser) {
  try {
    const toggled = await adminApi.toggleHrwaiUserStatus(row.id)
    const next = toggled?.status
    ElMessage.success(next === 1 ? '已启用' : '已禁用')
    loadUsers()
  } catch (error) {
    console.error('切换状态失败:', error)
  }
}

async function handleDelete(row: HrwaiUser) {
  try {
    await adminApi.deleteHrwaiUser(row.id)
    ElMessage.success('用户已删除')
    loadUsers()
  } catch (error) {
    console.error('删除用户失败:', error)
  }
}

// 操作下拉菜单统一入口
async function handleAction(cmd: string, row: HrwaiUser) {
  switch (cmd) {
    case 'resetPwd':
      openResetPwdDialog(row)
      break
    case 'toggle':
      handleToggleStatus(row)
      break
    case 'delete':
      try {
        await ElMessageBox.confirm('确定删除该用户？删除后不可恢复', '提示', {
          type: 'warning',
          confirmButtonText: '确定',
          cancelButtonText: '取消'
        })
        await handleDelete(row)
      } catch {
        // 用户取消
      }
      break
  }
}

onMounted(() => {
  loadUsers()
})
</script>

<template>
  <div class="hrwai-user-manage-page">
    <div class="page-header">
      <h2>HRWAI 用户管理</h2>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon> 新增用户
      </el-button>
    </div>

    <div class="filter-bar">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索用户名 / 姓名 / 昵称 / 手机号"
        clearable
        style="width: 280px"
        @clear="handleSearch"
        @keyup.enter="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-button type="primary" @click="handleSearch">搜索</el-button>
    </div>

    <el-table :data="users" v-loading="loading" stripe border style="width: 100%">
      <el-table-column prop="id" label="ID" width="70" align="center" />
      <el-table-column prop="username" label="用户名" min-width="140" />
      <el-table-column prop="name" label="姓名" min-width="120" />
      <el-table-column prop="nickname" label="昵称" min-width="120">
        <template #default="{ row }">
          {{ row.nickname || '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="phone" label="手机号" width="130" />
      <el-table-column prop="email" label="邮箱" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.email || '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="company" label="公司" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          {{ row.company || '-' }}
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="注册时间" width="160" align="center">
        <template #default="{ row }">
          {{ formatDate(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="90" fixed="right" align="center">
        <template #default="{ row }">
          <el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
            <el-button type="primary" link size="small">
              操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
            </el-button>
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
        @size-change="loadUsers"
        @current-change="loadUsers"
      />
    </div>

    <!-- 新增弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      title="新增 HRWAI 用户"
      width="520px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formData" :rules="formRules" label-width="90px">
        <el-form-item label="手机号" prop="phone">
          <el-input
            v-model="formData.phone"
            placeholder="请输入手机号"
            maxlength="11"
          />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="formData.password" type="password" placeholder="请输入密码（6-20字符）" maxlength="20" show-password />
        </el-form-item>
        <el-form-item label="姓名" prop="name">
          <el-input v-model="formData.name" placeholder="请输入姓名（2-10字符）" maxlength="10" show-word-limit />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="formData.email" placeholder="选填" maxlength="50" />
        </el-form-item>
        <el-form-item label="公司" prop="company">
          <el-input v-model="formData.company" placeholder="选填" maxlength="50" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确认</el-button>
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
        <el-form-item label="用户">
          <span>{{ pwdFormData.name }}</span>
        </el-form-item>
        <el-form-item label="新密码" prop="password">
          <el-input v-model="pwdFormData.password" type="password" placeholder="请输入新密码（6-20字符）" maxlength="20" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="pwdDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="pwdSubmitting" @click="handleResetPwd">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.hrwai-user-manage-page {
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
  color: #303133;
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
  .hrwai-user-manage-page {
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
