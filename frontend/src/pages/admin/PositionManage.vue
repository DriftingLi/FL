<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-ink">岗位管理</h1>
      <UiButton variant="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> 新增岗位
      </UiButton>
    </div>

    <div class="rounded-card border border-line bg-panel">
      <el-table :data="list" v-loading="loading" stripe>
        <el-table-column prop="position_id" label="ID" width="80" align="center" />
        <el-table-column prop="name" label="岗位名称" min-width="160" />
        <el-table-column prop="code" label="编码" width="160" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="sort_order" label="排序" width="80" align="center" />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="160" align="center">
          <template #default="{ row }">
            <UiButton link size="small" @click="openDialog(row)">编辑</UiButton>
            <el-popconfirm title="删除岗位不会删除职位/简历，仅解除关联，确定？" @confirm="handleDelete(row)">
              <template #reference>
                <UiButton variant="danger" link size="small">删除</UiButton>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div class="p-3 text-xs text-ink-3">岗位字典由管理员维护；职位发布与简历「期望岗位」都从这里选取（与专业方向解绑）。</div>
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑岗位' : '新增岗位'" width="480px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="80px">
        <el-form-item label="岗位名称" prop="name">
          <el-input v-model="form.name" maxlength="50" placeholder="如：叉车维修技师" />
        </el-form-item>
        <el-form-item label="编码" prop="code">
          <el-input v-model="form.code" maxlength="30" placeholder="唯一编码，如：maintenance_tech" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" maxlength="200" />
        </el-form-item>
        <el-form-item label="状态">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="停用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <UiButton @click="dialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="submitting" @click="handleSubmit">保存</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { unwrappedRequest } from '@/api/request'
import UiButton from '@/components/ui/UiButton.vue'

interface PositionItem {
  position_id: number
  name: string
  code: string
  description: string
  sort_order: number
  status: number
}

const list = ref<PositionItem[]>([])
const loading = ref(false)
const dialogVisible = ref(false)
const editing = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance | null>(null)
const form = reactive<{ id: number | null; name: string; code: string; description: string; status: number }>({ id: null, name: '', code: '', description: '', status: 1 })
const formRules = {
  name: [{ required: true, message: '请输入岗位名称', trigger: 'blur' }],
  code: [{ required: true, message: '请输入唯一编码', trigger: 'blur' }]
}

async function load() {
  loading.value = true
  try {
    const res: any = await unwrappedRequest.get('/admin/positions', { headers: { 'X-Silent': '1' } })
    list.value = res?.positions || []
  } catch {}
  loading.value = false
}

function openDialog(item?: PositionItem) {
  editing.value = !!item
  form.id = item?.position_id ?? null
  form.name = item?.name ?? ''
  form.code = item?.code ?? ''
  form.description = item?.description ?? ''
  form.status = item?.status ?? 1
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()
  submitting.value = true
  try {
    const payload = { name: form.name, code: form.code, description: form.description, status: form.status }
    if (editing.value && form.id != null) {
      await unwrappedRequest.put(`/admin/position/${form.id}`, payload)
      ElMessage.success('岗位已更新')
    } else {
      await unwrappedRequest.post('/admin/position', payload)
      ElMessage.success('岗位创建成功')
    }
    dialogVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDelete(item: PositionItem) {
  try {
    await unwrappedRequest.delete(`/admin/position/${item.position_id}`)
    ElMessage.success('岗位已删除')
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '删除失败')
  }
}

onMounted(load)
</script>