<template>
  <div class="credentials-page">
    <div class="page-header">
      <h2>证件管理</h2>
      <el-button type="primary" @click="openDialog()">
        <el-icon><Plus /></el-icon> 新增证件
      </el-button>
    </div>

    <el-card shadow="never">
      <el-table :data="filtered" v-loading="loading" stripe>
        <el-table-column label="证件名称" min-width="220">
          <template #default="{ row }">
            <span>{{ row.name }}</span>
            <el-tag size="small" :type="row.category === 'special_operation' ? '' : 'success'" style="margin-left: 6px">
              {{ row.category === 'special_operation' ? '特种作业' : `技能等级 L${row.level}` }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="code" label="编码" width="180" />
        <el-table-column prop="description" label="描述" min-width="200" show-overflow-tooltip />
        <el-table-column prop="sort_order" label="排序" width="70" align="center" />
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">{{ row.status === 1 ? '启用' : '停用' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row, $index }">
            <el-button link size="small" @click="openDialog(row)">编辑</el-button>
            <el-button link size="small" @click="move(row, $index, -1)" :disabled="$index === 0">上移</el-button>
            <el-button link size="small" @click="move(row, $index, 1)" :disabled="$index === filtered.length - 1">下移</el-button>
            <el-popconfirm title="确定删除？" @confirm="handleDelete(row)">
              <template #reference>
                <el-button link size="small" type="danger">删除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <div style="margin-top: 12px; display:flex; gap:12px;">
        <el-select v-model="filterCategory" placeholder="类别" clearable style="width: 160px" @change="applyFilter">
          <el-option label="特种作业" value="special_operation" />
          <el-option label="技能等级" value="skill_level" />
        </el-select>
        <el-input v-model="keyword" placeholder="搜索编码/名称" clearable style="width: 220px" @input="applyFilter" />
      </div>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="form.id ? '编辑证件' : '新增证件'" width="520px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="rules" label-width="90px">
        <el-form-item label="编码" prop="code">
          <el-input v-model="form.code" placeholder="如 forklift_n1" :disabled="!!form.id" />
        </el-form-item>
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="如 叉车司机N1证" />
        </el-form-item>
        <el-form-item label="类别" prop="category">
          <el-select v-model="form.category" placeholder="选择类别" style="width: 100%">
            <el-option label="特种作业上岗证" value="special_operation" />
            <el-option label="职业技能等级" value="skill_level" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="form.category === 'skill_level'" label="等级" prop="level">
          <el-select v-model="form.level" placeholder="选择等级 1-5" style="width: 100%">
            <el-option :label="'五级/初级工'" :value="5" />
            <el-option :label="'四级/中级工'" :value="4" />
            <el-option :label="'三级/高级工'" :value="3" />
            <el-option :label="'二级/技师'" :value="2" />
            <el-option :label="'一级/高级技师'" :value="1" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选" />
        </el-form-item>
        <el-form-item label="排序" prop="sort_order">
          <el-input-number v-model="form.sort_order" :min="0" :max="999" style="width: 100%" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-switch v-model="form.status" :active-value="1" :inactive-value="0" active-text="启用" inactive-text="停用" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { credentialApi, type CredentialDict, type CredentialPayload } from '@/api/credential'

const loading = ref(false)
const list = ref<CredentialDict[]>([])
const filterCategory = ref<string>('')
const keyword = ref('')

const filtered = computed(() => {
  return list.value.filter(c => {
    if (filterCategory.value && c.category !== filterCategory.value) return false
    if (keyword.value) {
      const k = keyword.value.toLowerCase()
      if (!c.code.toLowerCase().includes(k) && !c.name.toLowerCase().includes(k)) return false
    }
    return true
  })
})

function applyFilter() {
  // reactive via computed
}

async function load() {
  loading.value = true
  try {
    const data = await credentialApi.listAdminCredentials()
    list.value = data.credentials || []
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref<FormInstance | null>(null)
const form = reactive<{ id: number | null; code: string; name: string; category: 'special_operation' | 'skill_level' | ''; level: number | null; description: string; sort_order: number; status: number }>({
  id: null,
  code: '',
  name: '',
  category: '',
  level: null,
  description: '',
  sort_order: 0,
  status: 1
})

const rules = {
  code: [{ required: true, message: '请输入编码', trigger: 'blur' }],
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  category: [{ required: true, message: '请选择类别', trigger: 'change' }]
}

function openDialog(row?: CredentialDict) {
  if (row) {
    form.id = row.id
    form.code = row.code
    form.name = row.name
    form.category = row.category
    form.level = row.level
    form.description = row.description
    form.sort_order = row.sort_order
    form.status = row.status
  } else {
    form.id = null
    form.code = ''
    form.name = ''
    form.category = '' as any
    form.level = null
    form.description = ''
    form.sort_order = list.value.length + 1
    form.status = 1
  }
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()
  if (form.category === 'skill_level' && (form.level === null || form.level === undefined)) {
    ElMessage.warning('请选择技能等级')
    return
  }
  submitting.value = true
  try {
    const payload: CredentialPayload = {
      code: form.code,
      name: form.name,
      category: form.category as any,
      description: form.description,
      sort_order: form.sort_order,
      status: form.status
    }
    if (form.category === 'skill_level') payload.level = form.level!
    if (form.id) {
      // 编辑时不传 code（后端 code 为空表示不改动）
      const { code, ...rest } = payload as any
      await credentialApi.updateCredential(form.id, rest)
      ElMessage.success('已更新')
    } else {
      await credentialApi.createCredential(payload)
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    await load()
  } catch (e) {
    console.error(e)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(row: CredentialDict) {
  try {
    await credentialApi.deleteCredential(row.id)
    ElMessage.success('已删除')
    await load()
  } catch (e) {
    console.error(e)
  }
}

async function move(row: CredentialDict, idx: number, delta: number) {
  const target = filtered.value[idx + delta]
  if (!target) return
  try {
    await credentialApi.swapCredential(row.id, target.id)
    ElMessage.success('排序已更新')
    await load()
  } catch (e) {
    console.error(e)
  }
}

onMounted(load)
</script>

<style scoped>
.credentials-page {
  padding: 20px;
}
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.page-header h2 {
  font-size: 20px;
  margin: 0;
}
</style>
