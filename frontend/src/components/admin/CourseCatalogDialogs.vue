<script setup lang="ts">
// 课程目录管理弹窗表单（专业方向/课程等级/证书模板）：从 CourseCatalog 拆分，逻辑原样保留。
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { Plus } from '@element-plus/icons-vue'
import { trainingApi, type CatalogDirectionNode, type CatalogLevel, type CertificateTemplate } from '@/api/training'

defineProps<{
  certificateTemplates: CertificateTemplate[]
  submitting: boolean
}>()

const emit = defineEmits<{
  (e: 'catalog-changed'): void
  (e: 'certificates-changed'): void
}>()

const nameRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  code: [{ required: true, message: '请输入唯一编码', trigger: 'blur' }]
}

// ===== 方向 =====
const directionDialogVisible = ref(false)
const directionFormRef = ref<FormInstance | null>(null)
const directionForm = reactive<{ specialty_id: number | null; name: string; code: string }>({
  specialty_id: null,
  name: '',
  code: ''
})

function openDirectionDialog(d?: CatalogDirectionNode | null) {
  directionForm.specialty_id = d?.specialty_id ?? null
  directionForm.name = d?.name ?? ''
  directionForm.code = d?.code ?? ''
  directionDialogVisible.value = true
}

async function submitDirection() {
  if (!directionFormRef.value) return
  await directionFormRef.value.validate()
  try {
    // trainingApi 已解包信封：成功即业务成功（拦截器保证 200/201 才 resolve）
    if (directionForm.specialty_id) {
      await trainingApi.updateDirection(directionForm.specialty_id, {
        name: directionForm.name,
        code: directionForm.code
      })
      ElMessage.success('已更新')
    } else {
      await trainingApi.createDirection({ name: directionForm.name, code: directionForm.code })
      ElMessage.success('已创建')
    }
    directionDialogVisible.value = false
    emit('catalog-changed')
  } catch (error) {
    console.error('保存方向失败:', error)
    /* 错误已由拦截器提示 */
  }
}

// ===== 等级 =====
const levelDialogVisible = ref(false)
const levelFormRef = ref<FormInstance | null>(null)
const levelForm = reactive<{ level_id: number | null; name: string; code: string }>({
  level_id: null,
  name: '',
  code: ''
})

function openLevelDialog(l?: CatalogLevel | null) {
  levelForm.level_id = l?.level_id ?? null
  levelForm.name = l?.name ?? ''
  levelForm.code = l?.code ?? ''
  levelDialogVisible.value = true
}

async function submitLevel() {
  if (!levelFormRef.value) return
  await levelFormRef.value.validate()
  try {
    // trainingApi 已解包信封：成功即业务成功（拦截器保证 200/201 才 resolve）
    if (levelForm.level_id) {
      await trainingApi.updateLevel(levelForm.level_id, { name: levelForm.name, code: levelForm.code })
      ElMessage.success('已更新')
    } else {
      await trainingApi.createLevel({ name: levelForm.name, code: levelForm.code })
      ElMessage.success('已创建')
    }
    levelDialogVisible.value = false
    emit('catalog-changed')
  } catch (error) {
    console.error('保存等级失败:', error)
    /* 错误已由拦截器提示 */
  }
}

// ===== 证书模板 =====
const certificateDialogVisible = ref(false)
const certificateLoading = ref(false)
const certificateFormVisible = ref(false)
const certificateFormRef = ref<FormInstance | null>(null)
const certificateForm = reactive<{ id: number | null; name: string; code: string; validity_days: number; description: string }>({
  id: null,
  name: '',
  code: '',
  validity_days: 365,
  description: ''
})

const certificateRules = {
  name: [{ required: true, message: '请输入模板名称', trigger: 'blur' }],
  code: [{ required: true, message: '请输入唯一编码', trigger: 'blur' }],
  validity_days: [{ required: true, message: '请输入有效期', trigger: 'change' }]
}

function openCertificateDialog() {
  certificateDialogVisible.value = true
}

function openCertificateForm(tpl?: CertificateTemplate | null) {
  certificateForm.id = tpl?.id ?? null
  certificateForm.name = tpl?.name ?? ''
  certificateForm.code = tpl?.code ?? ''
  certificateForm.validity_days = tpl?.validity_days ?? 365
  certificateForm.description = tpl?.description ?? ''
  certificateFormVisible.value = true
}

async function submitCertificate() {
  if (!certificateFormRef.value) return
  await certificateFormRef.value.validate()
  try {
    // trainingApi 已解包信封：成功即业务成功（拦截器保证 200/201 才 resolve）
    const payload = {
      name: certificateForm.name,
      code: certificateForm.code,
      validity_days: certificateForm.validity_days,
      description: certificateForm.description
    }
    if (certificateForm.id) {
      await trainingApi.updateCertificateTemplate(certificateForm.id, payload)
      ElMessage.success('已更新')
    } else {
      await trainingApi.createCertificateTemplate(payload)
      ElMessage.success('已创建')
    }
    certificateFormVisible.value = false
    emit('certificates-changed')
  } catch (error) {
    console.error('保存模板失败:', error)
    /* 错误已由拦截器提示 */
  }
}

async function handleDeleteCertificate(tpl: CertificateTemplate) {
  try {
    // trainingApi 已解包信封：成功即业务成功
    await trainingApi.deleteCertificateTemplate(tpl.id)
    ElMessage.success('已删除')
    emit('certificates-changed')
  } catch (error) {
    console.error('删除模板失败:', error)
    /* 错误已由拦截器提示 */
  }
}

defineExpose({ openDirectionDialog, openLevelDialog, openCertificateDialog })
</script>

<template>
  <!-- 专业方向对话框 -->
  <el-dialog v-model="directionDialogVisible" :title="directionForm.specialty_id ? '编辑专业方向' : '新增专业方向'" width="460px" destroy-on-close>
    <el-form ref="directionFormRef" :model="directionForm" :rules="nameRules" label-width="80px">
      <el-form-item label="名称" prop="name">
        <el-input v-model="directionForm.name" placeholder="如：操作、维修、安全、电池" maxlength="30" />
      </el-form-item>
      <el-form-item label="编码" prop="code">
        <el-input v-model="directionForm.code" placeholder="唯一编码，如 OPERATE" maxlength="30" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="directionDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submitDirection">保存</el-button>
    </template>
  </el-dialog>

  <!-- 课程等级对话框（等级全局共享） -->
  <el-dialog v-model="levelDialogVisible" :title="levelForm.level_id ? '编辑课程等级' : '新增课程等级'" width="460px" destroy-on-close>
    <el-form ref="levelFormRef" :model="levelForm" :rules="nameRules" label-width="80px">
      <el-form-item label="等级名称" prop="name">
        <el-input v-model="levelForm.name" placeholder="如：入门、进阶、专项、认证" maxlength="30" />
      </el-form-item>
      <el-form-item label="编码" prop="code">
        <el-input v-model="levelForm.code" placeholder="唯一编码，如 BEGINNER" maxlength="30" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="levelDialogVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submitLevel">保存</el-button>
    </template>
  </el-dialog>

  <!-- 证书模板管理 -->
  <el-dialog v-model="certificateDialogVisible" title="证书模板管理" width="720px" destroy-on-close>
    <div class="certificate-header">
      <el-button type="primary" size="small" @click="openCertificateForm()">
        <el-icon><Plus /></el-icon> 新增模板
      </el-button>
    </div>
    <el-table :data="certificateTemplates" stripe border size="small" v-loading="certificateLoading">
      <el-table-column prop="id" label="ID" width="60" align="center" />
      <el-table-column prop="name" label="模板名称" min-width="140" show-overflow-tooltip />
      <el-table-column prop="code" label="编码" width="110" />
      <el-table-column label="默认有效期(天)" width="120" align="center">
        <template #default="{ row }">
          {{ row.validity_days ?? '-' }}
        </template>
      </el-table-column>
      <el-table-column prop="description" label="说明" min-width="160" show-overflow-tooltip />
      <el-table-column label="操作" width="120" align="center">
        <template #default="{ row }">
          <el-button link size="small" @click="openCertificateForm(row)">编辑</el-button>
          <el-popconfirm title="确定删除该模板？" @confirm="handleDeleteCertificate(row)">
            <template #reference>
              <el-button link size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
    <el-empty v-if="!certificateLoading && certificateTemplates.length === 0" description="暂无证书模板" />
    <template #footer>
      <el-button @click="certificateDialogVisible = false">关闭</el-button>
    </template>
  </el-dialog>

  <el-dialog v-model="certificateFormVisible" :title="certificateForm.id ? '编辑模板' : '新增模板'" width="480px" destroy-on-close append-to-body>
    <el-form ref="certificateFormRef" :model="certificateForm" :rules="certificateRules" label-width="110px">
      <el-form-item label="模板名称" prop="name">
        <el-input v-model="certificateForm.name" placeholder="如：叉车维修技能培训合格证书" maxlength="50" />
      </el-form-item>
      <el-form-item label="编码" prop="code">
        <el-input v-model="certificateForm.code" placeholder="唯一编码，如 FORKLIFT_MAINTENANCE_CERT" maxlength="30" />
      </el-form-item>
      <el-form-item label="有效期(天)" prop="validity_days">
        <el-input-number v-model="certificateForm.validity_days" :min="1" :max="36500" style="width: 100%" />
      </el-form-item>
      <el-form-item label="说明">
        <el-input v-model="certificateForm.description" type="textarea" :rows="3" maxlength="300" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="certificateFormVisible = false">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="submitCertificate">保存</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.certificate-header {
  margin-bottom: var(--space-3);
}
</style>
