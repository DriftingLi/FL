<template>
  <div class="admin-manage-page">
    <div class="page-header">
      <h2>管理员管理</h2>
      <el-button type="primary" @click="openAddDialog">
        <el-icon><Plus /></el-icon> 新增管理员
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

    <el-table :data="admins" v-loading="loading" stripe border style="width: 100%">
      <el-table-column prop="admin_id" label="ID" width="70" align="center" />
      <el-table-column prop="username" label="用户名" width="160" />
      <el-table-column prop="name" label="姓名" width="140" />
      <el-table-column prop="created_at" label="创建时间" width="200" align="center">
        <template #default="{ row }">
          {{ formatDate(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right" align="center">
        <template #default="{ row }">
          <el-tooltip v-if="row.admin_id === currentAdminId" content="不能删除当前登录的管理员" placement="top">
            <el-button type="info" link size="small" disabled>删除</el-button>
          </el-tooltip>
          <el-popconfirm v-else title="确定删除该管理员？删除后不可恢复" @confirm="handleDelete(row.admin_id)">
            <template #reference>
              <el-button type="danger" link size="small">删除</el-button>
            </template>
          </el-popconfirm>
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
        @size-change="loadAdmins"
        @current-change="loadAdmins"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      title="新增管理员"
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
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">确认添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Plus, Search } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { adminApi } from '@/api/admin'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const currentAdminId = computed(() => authStore.userInfo?.user_id)

const loading = ref(false)
const admins = ref([])
const searchKeyword = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref(null)
const formData = reactive({
  username: '',
  password: '',
  name: ''
})

const formRules = {
  username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 20, message: '用户名长度为3-20个字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 6, max: 20, message: '密码长度为6-20个字符', trigger: 'blur' }
  ],
  name: [
    { required: true, message: '请输入姓名', trigger: 'blur' },
    { min: 2, max: 10, message: '姓名长度为2-10个字符', trigger: 'blur' }
  ]
}

function formatDate(dateStr) {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

async function loadAdmins() {
  loading.value = true
  try {
    const params = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (searchKeyword.value) {
      params.keyword = searchKeyword.value
    }

    const res = await adminApi.getAdmins(params)
    if (res.code === 200) {
      admins.value = res.data.admins || []
      total.value = res.data.total || 0
    }
  } catch (error) {
    console.error('加载管理员列表失败:', error)
    ElMessage.error('加载管理员列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  currentPage.value = 1
  loadAdmins()
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
    const res = await adminApi.addAdmin({
      username: formData.username,
      password: formData.password,
      name: formData.name
    })
    if (res.code === 201) {
      ElMessage.success('管理员添加成功')
      dialogVisible.value = false
      loadAdmins()
    }
  } catch (error) {
    console.error('添加管理员失败:', error)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(adminId) {
  try {
    const res = await adminApi.deleteAdmin(adminId)
    if (res.code === 200) {
      ElMessage.success('管理员已删除')
      loadAdmins()
    }
  } catch (error) {
    console.error('删除管理员失败:', error)
    ElMessage.error('删除管理员失败')
  }
}

onMounted(() => {
  loadAdmins()
})
</script>

<style scoped>
.admin-manage-page {
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
  .admin-manage-page {
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
