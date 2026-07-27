<template>
  <div class="student-manage-page">
    <div class="page-header">
      <h2>学员管理</h2>
      <el-button type="primary" @click="openAddDialog">
        <el-icon><Plus /></el-icon> 新增学员
      </el-button>
    </div>

    <div class="filter-bar">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索用户名或姓名"
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

    <el-table :data="students" v-loading="loading" stripe border style="width: 100%">
      <el-table-column prop="student_id" label="ID" width="70" align="center" />
      <el-table-column prop="username" label="用户名" min-width="160" />
      <el-table-column prop="name" label="姓名" min-width="140" />
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '正常' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="注册时间" width="200" align="center">
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
        @size-change="loadStudents"
        @current-change="loadStudents"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      title="新增学员"
      width="480px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formData" :rules="formRules" label-width="100px">
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="formData.phone" placeholder="请输入手机号" maxlength="11" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="formData.password" type="password" placeholder="请输入密码（6-20字符）" maxlength="20" show-password />
        </el-form-item>
        <el-form-item label="姓名" prop="name">
          <el-input v-model="formData.name" placeholder="请输入姓名（2-10字符）" maxlength="10" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确认添加</el-button>
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
        <el-form-item label="学员">
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

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus, Search, ArrowDown } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance, FormRules } from 'element-plus'
import { adminApi } from '@/api/admin'
import { phoneRules, passwordRules, nameRules } from '@/utils/validate'

interface StudentRow {
  student_id: number
  username: string
  name: string
  status: number
  created_at: string
}

const loading = ref(false)
const students = ref<StudentRow[]>([])
const searchKeyword = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance>()
const formData = reactive({
  phone: '',
  password: '',
  name: ''
})

const formRules: FormRules = {
  phone: phoneRules,
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

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

async function loadStudents() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (searchKeyword.value) {
      params.keyword = searchKeyword.value
    }

    const res = await adminApi.getStudents(params)
    if (res.code === 200) {
      students.value = res.data.students || []
      total.value = res.data.total || 0
    }
  } catch (error) {
    console.error('加载学员列表失败:', error)
    ElMessage.error('加载学员列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  currentPage.value = 1
  loadStudents()
}

function openAddDialog() {
  formData.phone = ''
  formData.password = ''
  formData.name = ''
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()

  submitting.value = true
  try {
    const res = await adminApi.addStudent({
      phone: formData.phone,
      password: formData.password,
      name: formData.name
    })
    if (res.code === 201) {
      ElMessage.success('学员添加成功')
      dialogVisible.value = false
      loadStudents()
    }
  } catch (error) {
    console.error('添加学员失败:', error)
  } finally {
    submitting.value = false
  }
}

function openResetPwdDialog(row: StudentRow) {
  pwdFormData.id = row.student_id
  pwdFormData.name = row.name
  pwdFormData.password = ''
  pwdDialogVisible.value = true
}

async function handleResetPwd() {
  if (!pwdFormRef.value) return
  await pwdFormRef.value.validate()

  pwdSubmitting.value = true
  try {
    await adminApi.resetStudentPassword(pwdFormData.id, pwdFormData.password)
    ElMessage.success('密码已重置')
    pwdDialogVisible.value = false
  } catch (error) {
    console.error('重置密码失败:', error)
  } finally {
    pwdSubmitting.value = false
  }
}

async function handleToggleStatus(row: StudentRow) {
  try {
    await adminApi.toggleStudentStatus(row.student_id)
    ElMessage.success(row.status === 1 ? '已禁用' : '已启用')
    loadStudents()
  } catch (error) {
    console.error('切换状态失败:', error)
  }
}

async function handleDelete(studentId: number) {
  try {
    const res = await adminApi.deleteStudent(studentId)
    if (res.code === 200) {
      ElMessage.success('学员已删除')
      loadStudents()
    }
  } catch (error) {
    console.error('删除学员失败:', error)
    ElMessage.error('删除学员失败')
  }
}

// 操作下拉菜单统一入口
async function handleAction(cmd: string, row: StudentRow) {
  switch (cmd) {
    case 'resetPwd':
      openResetPwdDialog(row)
      break
    case 'toggle':
      handleToggleStatus(row)
      break
    case 'delete':
      try {
        await ElMessageBox.confirm('确定删除该学员？删除后不可恢复', '提示', {
          type: 'warning',
          confirmButtonText: '确定',
          cancelButtonText: '取消'
        })
        await handleDelete(row.student_id)
      } catch {
        // 用户取消
      }
      break
  }
}

onMounted(() => {
  loadStudents()
})
</script>

<style scoped>
.student-manage-page {
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
  .student-manage-page {
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
