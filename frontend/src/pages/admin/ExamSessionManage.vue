<template>
  <div class="exam-session-manage">
    <div class="page-header">
      <h2>考试场次管理</h2>
      <el-button type="primary" @click="showCreateDialog">创建考试场次</el-button>
    </div>

    <el-table :data="sessions" stripe v-loading="loading">
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="name" label="考试名称" />
      <el-table-column prop="start_time" label="开始时间" width="180">
        <template #default="{ row }">{{ formatDateTime(row.start_time) }}</template>
      </el-table-column>
      <el-table-column prop="duration" label="时长(分)" width="90" />
      <el-table-column prop="pass_score" label="及格分" width="80" />
      <el-table-column prop="status" label="状态" width="100">
        <template #default="{ row }">
          <el-tag :type="statusType[row.status]" size="small">{{ statusMap[row.status] }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="250">
        <template #default="{ row }">
          <el-button v-if="row.status === 'upcoming'" size="small" @click="editSession(row)">编辑</el-button>
          <el-button v-if="row.status === 'upcoming'" size="small" type="success" @click="changeStatus(row, 'ongoing')">开始</el-button>
          <el-button v-if="row.status === 'ongoing'" size="small" type="warning" @click="changeStatus(row, 'finished')">结束</el-button>
          <el-button v-if="row.status === 'upcoming'" size="small" type="danger" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog v-model="dialogVisible" :title="editingId ? '编辑考试场次' : '创建考试场次'" width="500px">
      <el-form :model="sessionForm" label-width="100px">
        <el-form-item label="考试名称" required>
          <el-input v-model="sessionForm.name" placeholder="请输入考试名称" />
        </el-form-item>
        <el-form-item label="开始时间" required>
          <el-date-picker v-model="sessionForm.start_time" type="datetime" placeholder="选择开始时间" />
        </el-form-item>
        <el-form-item label="结束时间" required>
          <el-date-picker v-model="sessionForm.end_time" type="datetime" placeholder="选择结束时间" />
        </el-form-item>
        <el-form-item label="时长(分钟)">
          <el-input-number :model-value="90" disabled />
          <span class="form-tip">固定90分钟</span>
        </el-form-item>
        <el-form-item label="及格分数">
          <el-input-number :model-value="60" disabled />
          <span class="form-tip">固定60分</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitSession" :loading="submitting">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { levelExamApi } from '@/api/levelExam'

const statusMap = { upcoming: '未开始', ongoing: '进行中', finished: '已结束' }
const statusType = { upcoming: 'info', ongoing: 'success', finished: '' }

const loading = ref(false)
const sessions = ref([])
const dialogVisible = ref(false)
const editingId = ref(null)
const submitting = ref(false)
const sessionForm = ref({
  name: '', start_time: '', end_time: ''
})

onMounted(() => loadData())

async function loadData() {
  loading.value = true
  try {
    const res = await levelExamApi.getSessions({ page: 1, page_size: 50 })
    sessions.value = res.data?.sessions || []
  } catch (e) {} finally { loading.value = false }
}

function showCreateDialog() {
  editingId.value = null
  sessionForm.value = { name: '', start_time: '', end_time: '' }
  dialogVisible.value = true
}

function editSession(row) {
  editingId.value = row.id
  sessionForm.value = { name: row.name, start_time: row.start_time, end_time: row.end_time }
  dialogVisible.value = true
}

function toLocalISOString(date) {
  const d = new Date(date)
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}

function formatDateTime(dtStr) {
  if (!dtStr) return ''
  const d = new Date(dtStr)
  if (isNaN(d.getTime())) return dtStr
  const pad = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

async function submitSession() {
  submitting.value = true
  try {
    const data = {
      name: sessionForm.value.name,
      start_time: sessionForm.value.start_time ? toLocalISOString(sessionForm.value.start_time) : '',
      end_time: sessionForm.value.end_time ? toLocalISOString(sessionForm.value.end_time) : ''
    }
    if (editingId.value) {
      await levelExamApi.updateSession(editingId.value, data)
      ElMessage.success('更新成功')
    } else {
      await levelExamApi.createSession(data)
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    await loadData()
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  } finally { submitting.value = false }
}

async function changeStatus(row, status) {
  try {
    await ElMessageBox.confirm(`确定将考试状态改为"${statusMap[status]}"？`, '提示', { type: 'warning' })
    await levelExamApi.updateSessionStatus(row.id, status)
    ElMessage.success('状态更新成功')
    await loadData()
  } catch (e) {}
}

async function handleDelete(row) {
  try {
    await ElMessageBox.confirm('确定删除此考试场次？', '提示', { type: 'warning' })
    await levelExamApi.deleteSession(row.id)
    ElMessage.success('删除成功')
    await loadData()
  } catch (e) {}
}
</script>

<style scoped>
.exam-session-manage { padding: 0; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.page-header h2 { margin: 0; }
.form-tip { color: #909399; font-size: 12px; margin-left: 8px; }
</style>
