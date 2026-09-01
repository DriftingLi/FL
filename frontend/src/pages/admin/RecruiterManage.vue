<template>
<div class="recruiter-manage-page">
<div class="page-header">
<h2>企业招聘者管理</h2>
<el-button type="primary" @click="openAddDialog">
<el-icon><Plus /></el-icon> 新增招聘者
</el-button>
</div>
<div class="filter-bar">
<el-input
v-model="searchKeyword"
placeholder="搜索企业名或账号"
clearable
style="width: 280px"
@clear="search"
@keyup.enter="search"
>
<template #prefix><el-icon><Search /></el-icon></template>
</el-input>
<el-button type="primary" @click="search">搜索</el-button>
</div>

<el-table :data="list" v-loading="loading" stripe border style="width: 100%" row-key="id">
<el-table-column prop="id" label="ID" width="70" align="center" />
<el-table-column prop="company_name" label="企业名" min-width="180" />
<el-table-column prop="credit_code" label="统一社会信用代码" min-width="180" />
<el-table-column prop="contact_name" label="联系人" min-width="120" />
<el-table-column prop="contact_phone" label="联系电话" min-width="140" />
<el-table-column label="状态" width="100" align="center">
<template #default="{ row }">
<el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">{{ row.status === 1 ? '正常' : '禁用' }}</el-tag>
</template>
</el-table-column>
<el-table-column prop="created_at" label="创建时间" width="180" align="center">
<template #default="{ row }">{{ formatDateTime(row.created_at) }}</template>
</el-table-column>
<el-table-column label="操作" width="110" fixed="right" align="center">
<template #default="{ row }">
<el-dropdown trigger="click" @command="(cmd: string) => handleAction(cmd, row)">
<el-button type="primary" link size="small">
操作<el-icon class="el-icon--right"><ArrowDown /></el-icon>
</el-button>
<template #dropdown>
<el-dropdown-menu>
<el-dropdown-item command="edit">编辑企业信息</el-dropdown-item>
<el-dropdown-item command="resetPwd">重置密码</el-dropdown-item>
<el-dropdown-item command="toggle">{{ row.status === 1 ? '禁用' : '启用' }}</el-dropdown-item>
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

<el-dialog v-model="dialogVisible" title="新增企业招聘者" width="520px" destroy-on-close>
<el-form ref="formRef" :model="formData" :rules="formRules" label-width="110px">
<el-form-item label="用户名" prop="username">
<el-input v-model="formData.username" placeholder="请输入用户名" maxlength="20" />
</el-form-item>
<el-form-item label="密码" prop="password">
<el-input v-model="formData.password" type="password" placeholder="请输入密码（6-20位）" maxlength="20" show-password />
</el-form-item>
<el-form-item label="企业名称" prop="company_name">
<el-input v-model="formData.company_name" placeholder="请输入企业名称" />
</el-form-item>
<el-form-item label="统一社会信用代码" prop="credit_code">
<el-input v-model="formData.credit_code" placeholder="请输入统一社会信用代码" />
</el-form-item>
<el-form-item label="经营范围" prop="business_scope">
<el-input v-model="formData.business_scope" placeholder="请输入经营范围" />
</el-form-item>
<el-form-item label="联系人" prop="contact_name">
<el-input v-model="formData.contact_name" placeholder="请输入联系人" />
</el-form-item>
<el-form-item label="联系电话" prop="contact_phone">
<el-input v-model="formData.contact_phone" placeholder="请输入联系电话" />
</el-form-item>
<el-form-item label="联系邮箱" prop="contact_email">
<el-input v-model="formData.contact_email" placeholder="请输入联系邮箱" />
</el-form-item>
</el-form>
<template #footer>
<el-button @click="dialogVisible = false">取消</el-button>
<el-button type="primary" :loading="submitting" @click="handleSubmit">确认创建</el-button>
</template>
</el-dialog>

<el-dialog v-model="editDialogVisible" title="编辑企业信息" width="520px" destroy-on-close>
<el-form ref="editFormRef" :model="editForm" :rules="editFormRules" label-width="110px">
<el-form-item label="用户名" prop="username">
<el-input v-model="editForm.username" placeholder="请输入用户名（4-20位字母/数字/下划线）" maxlength="20" />
</el-form-item>
<el-form-item label="企业名称" prop="company_name">
<el-input v-model="editForm.company_name" placeholder="请输入企业名称" />
</el-form-item>
<el-form-item label="统一社会信用代码" prop="credit_code">
<el-input v-model="editForm.credit_code" placeholder="请输入统一社会信用代码" />
</el-form-item>
<el-form-item label="经营范围" prop="business_scope">
<el-input v-model="editForm.business_scope" placeholder="请输入经营范围" />
</el-form-item>
<el-form-item label="联系人" prop="contact_name">
<el-input v-model="editForm.contact_name" placeholder="请输入联系人" />
</el-form-item>
<el-form-item label="联系电话" prop="contact_phone">
<el-input v-model="editForm.contact_phone" placeholder="请输入联系电话" />
</el-form-item>
<el-form-item label="联系邮箱" prop="contact_email">
<el-input v-model="editForm.contact_email" placeholder="请输入联系邮箱" />
</el-form-item>
</el-form>
<template #footer>
<el-button @click="editDialogVisible = false">取消</el-button>
<el-button type="primary" :loading="editing" @click="handleEditSubmit">保存修改</el-button>
</template>
</el-dialog>

<el-dialog v-model="pwdDialogVisible" title="重置密码" width="440px" destroy-on-close>
<el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-width="90px">
<el-form-item label="招聘者">
<span>{{ pwdForm.username }}</span>
</el-form-item>
<el-form-item label="新密码" prop="password">
<el-input v-model="pwdForm.password" type="password" placeholder="请输入新密码（6-20位）" maxlength="20" show-password />
</el-form-item>
</el-form>
<template #footer>
<el-button @click="pwdDialogVisible = false">取消</el-button>
<el-button type="primary" :loading="pwdSubmitting" @click="handleResetPwd">确认重置</el-button>
</template>
</el-dialog>
</div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus, Search, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormItemRule, FormRules } from 'element-plus'
import { adminApi, type AdminRecruiter } from '@/api/admin'
import { usernameRules } from '@/utils/validate'
import { useAdminTable } from '@/composables/useAdminTable'
import { formatDateTime } from '@/utils/format'

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const formData = reactive({
  username: '', password: '', company_name: '', credit_code: '', business_scope: '', contact_name: '', contact_phone: '', contact_email: ''
})

// 必填校验与后端 ValidateRecruiterInput 逐条同源（#416：前后端校验口径单点，不各写一套）
const required = (msg: string) => ({ required: true, message: msg, trigger: 'blur' })
// 编辑弹窗用户名规则：可选（空 = 不改用户名）；填写时校验格式。
// 与创建弹窗 usernameRules 的差异：不强制必填——历史数据可能存在 <4 位用户名，保持原值不校验。
const editUsernameRules: FormItemRule[] = [
  { validator: (_r, v, cb) => {
    const s = String(v ?? '').trim()
    if (s === '') { cb() ; return }
    if (s.length < 4 || s.length > 20) { cb(new Error('长度在4到20个字符')); return }
    if (!/^[a-zA-Z0-9_]+$/.test(s)) { cb(new Error('只能包含字母、数字和下划线')); return }
    cb()
  }, trigger: 'blur' }
]

const formRules: FormRules = {
  username: usernameRules,
  password: [
    { required: true, message: '密码不能为空', trigger: 'blur' },
    { min: 6, max: 20, message: '密码长度需为 6-20 位', trigger: 'blur' },
  ],
  company_name: [required('企业名称不能为空')],
  credit_code: [required('统一社会信用代码不能为空')],
  business_scope: [required('经营范围不能为空')],
  contact_name: [required('联系人不能为空')],
  contact_phone: [required('联系电话不能为空')],
  contact_email: [required('联系邮箱不能为空')],
}

const editDialogVisible = ref(false)
const editing = ref(false)
const editFormRef = ref<FormInstance>()
const editFormRules: FormRules = {
  username: editUsernameRules,
  company_name: formRules.company_name,
  credit_code: formRules.credit_code,
  business_scope: formRules.business_scope,
  contact_name: formRules.contact_name,
  contact_phone: formRules.contact_phone,
  contact_email: formRules.contact_email,
}

const editForm = reactive({ id: 0, username: '', company_name: '', credit_code: '', business_scope: '', contact_name: '', contact_phone: '', contact_email: '' })

const pwdDialogVisible = ref(false)
const pwdSubmitting = ref(false)
const pwdFormRef = ref<FormInstance>()
const pwdForm = reactive({ id: 0, username: '', password: '' })
const pwdRules: FormRules = {
  password: [
    { required: true, message: '新密码不能为空', trigger: 'blur' },
    { min: 6, max: 20, message: '密码长度需为 6-20 位', trigger: 'blur' },
  ],
}

function openEditDialog(row: AdminRecruiter) {
  editForm.id = row.id
  editForm.username = row.username
  editForm.company_name = row.company_name
  editForm.credit_code = row.credit_code
  editForm.business_scope = row.business_scope
  editForm.contact_name = row.contact_name
  editForm.contact_phone = row.contact_phone
  editForm.contact_email = row.contact_email
  editDialogVisible.value = true
}

function openResetPwdDialog(row: AdminRecruiter) {
  pwdForm.id = row.id
  pwdForm.username = row.username
  pwdForm.password = ''
  pwdDialogVisible.value = true
}

async function handleEditSubmit() {
  if (!editFormRef.value) return
  await editFormRef.value.validate()
  editing.value = true
  try {
    const payload: any = {
      company_name: editForm.company_name,
      credit_code: editForm.credit_code,
      business_scope: editForm.business_scope,
      contact_name: editForm.contact_name,
      contact_phone: editForm.contact_phone,
      contact_email: editForm.contact_email,
    }
    if (editForm.username.trim()) payload.username = editForm.username.trim()
    await adminApi.editRecruiter(editForm.id, payload)
    ElMessage.success('企业信息已更新')
    editDialogVisible.value = false
    load()
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    editing.value = false
  }
}

async function handleResetPwd() {
  if (!pwdFormRef.value) return
  await pwdFormRef.value.validate()
  pwdSubmitting.value = true
  try {
    await adminApi.resetRecruiterPassword(pwdForm.id, pwdForm.password)
    ElMessage.success('密码已重置')
    pwdDialogVisible.value = false
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    pwdSubmitting.value = false
  }
}

function handleAction(cmd: string, row: AdminRecruiter) {
  if (cmd === 'edit') openEditDialog(row)
  else if (cmd === 'resetPwd') openResetPwdDialog(row)
  else if (cmd === 'toggle') handleToggle(row)
}

async function handleToggle(row: AdminRecruiter) {
  const next = row.status === 1 ? '禁用' : '启用'
  try {
    await ElMessageBox.confirm(`确定${next}该招聘者账号？`, '提示', { type: 'warning' })
  } catch {
    return
  }
  try {
    await adminApi.toggleRecruiterStatus(row.id)
    ElMessage.success(`已${next}`)
    load()
  } catch {
    /* 错误已由拦截器提示 */
  }
}

// admin 列表状态机：只声明 fetch 与行操作 adapter
const table = useAdminTable<AdminRecruiter>({
  fetch: async (paging, filters) => {
    const data = await adminApi.getRecruiters({
      page: paging.page,
      page_size: paging.pageSize,
      keyword: filters.keyword ? String(filters.keyword) : undefined
    })
    return { list: data?.items || [], total: data?.total || 0 }
  },
  actions: {}
})
const { loading, list, total, currentPage, pageSize, searchKeyword, load, search } = table

function openAddDialog() {
  formData.username = ''; formData.password = ''; formData.company_name = ''; formData.credit_code = '';
  formData.business_scope = ''; formData.contact_name = ''; formData.contact_phone = ''; formData.contact_email = '';
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()
  submitting.value = true
  try {
    await adminApi.addRecruiter({ ...formData })
    ElMessage.success('招聘者创建成功')
    dialogVisible.value = false
    load()
  } catch {
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped>
.recruiter-manage-page { padding: 16px; }
.page-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.page-header h2 { font-size: 18px; font-weight: 600; margin: 0; }
.filter-bar { display: flex; gap: 8px; margin-bottom: 12px; }
.pagination-wrapper { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>