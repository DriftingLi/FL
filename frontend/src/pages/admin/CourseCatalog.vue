<template>
  <div class="course-catalog-page">
    <div class="page-header">
      <h2>课程目录管理</h2>
      <div class="header-actions">
        <el-button @click="openCertificateDialog()">
          <el-icon><Tickets /></el-icon> 证书模板
        </el-button>
        <el-button type="primary" @click="openDirectionDialog()">
          <el-icon><Plus /></el-icon> 新增专业方向
        </el-button>
      </div>
    </div>

    <el-alert
      class="catalog-tip"
      type="info"
      :closable="false"
      show-icon
      title="目录结构：专业方向 → 课程等级 → 课程 → 章节，支持增删改与排序（方向/等级/课程在所属层级内排序，章节在课程内排序）"
    />

    <div class="catalog-body" v-loading="loading">
      <el-tree
        v-if="treeData.length > 0"
        :data="treeData"
        node-key="__key"
        :props="{ label: 'name', children: 'children' }"
        default-expand-all
        :expand-on-click-node="false"
        class="catalog-tree"
      >
        <template #default="{ data }">
          <div class="tree-node" :class="`node-${data.__type}`">
            <span class="node-label" @click="handleNodeClick(data)">
              <el-icon v-if="data.__type === 'direction'" class="node-icon"><FolderOpened /></el-icon>
              <el-icon v-else-if="data.__type === 'level'" class="node-icon"><Collection /></el-icon>
              <el-icon v-else-if="data.__type === 'course'" class="node-icon"><Notebook /></el-icon>
              <el-icon v-else class="node-icon"><Document /></el-icon>
              <span class="node-name">{{ data.name }}</span>
              <el-tag v-if="data.__type === 'course'" size="small" :type="data.status === 1 ? 'success' : 'info'">
                {{ data.status === 1 ? '上架' : '下架' }}
              </el-tag>
              <span v-if="data.__type === 'level' && data.code" class="node-sub">{{ data.code }}</span>
              <span v-if="data.__type === 'course' && data.level_name" class="node-sub">{{ data.level_name }}</span>
            </span>
            <span class="node-actions" @click.stop>
              <template v-if="data.__type === 'direction'">
                <el-button link size="small" @click="openLevelDialog()">新增等级</el-button>
                <el-button link size="small" @click="moveNode(data, -1)" :disabled="!canMove(data, -1)">
                  <el-icon><Top /></el-icon>
                </el-button>
                <el-button link size="small" @click="moveNode(data, 1)" :disabled="!canMove(data, 1)">
                  <el-icon><Bottom /></el-icon>
                </el-button>
                <el-button link size="small" @click="openDirectionDialog(data)">编辑</el-button>
                <el-popconfirm title="删除该方向后，其下课程将变为未关联方向（等级保留），确定？" @confirm="handleDeleteDirection(data)">
                  <template #reference>
                    <el-button link size="small" type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
              <template v-else-if="data.__type === 'level'">
                <el-button link size="small" @click="openCourseDialog(data)">新增课程</el-button>
                <el-button link size="small" @click="moveNode(data, -1)" :disabled="!canMove(data, -1)">
                  <el-icon><Top /></el-icon>
                </el-button>
                <el-button link size="small" @click="moveNode(data, 1)" :disabled="!canMove(data, 1)">
                  <el-icon><Bottom /></el-icon>
                </el-button>
                <el-button link size="small" @click="openLevelDialog(data)">编辑</el-button>
                <el-popconfirm title="删除该等级后，其下课程将变为未关联等级，确定？" @confirm="handleDeleteLevel(data)">
                  <template #reference>
                    <el-button link size="small" type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
              <template v-else-if="data.__type === 'course'">
                <el-button link size="small" @click="openChapterDialog(data)">新增章节</el-button>
                <el-button link size="small" @click="moveNode(data, -1)" :disabled="!canMove(data, -1)">
                  <el-icon><Top /></el-icon>
                </el-button>
                <el-button link size="small" @click="moveNode(data, 1)" :disabled="!canMove(data, 1)">
                  <el-icon><Bottom /></el-icon>
                </el-button>
                <el-button link size="small" @click="openCourseDialog(null, data)">编辑</el-button>
                <el-popconfirm title="删除该课程会级联删除其章节，确定？" @confirm="handleDeleteCourse(data)">
                  <template #reference>
                    <el-button link size="small" type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
              <template v-else>
                <el-button link size="small" @click="moveNode(data, -1)" :disabled="!canMove(data, -1)">
                  <el-icon><Top /></el-icon>
                </el-button>
                <el-button link size="small" @click="moveNode(data, 1)" :disabled="!canMove(data, 1)">
                  <el-icon><Bottom /></el-icon>
                </el-button>
                <el-button link size="small" @click="openChapterDialog(null, data)">编辑</el-button>
                <el-popconfirm title="确定删除该章节？" @confirm="handleDeleteChapter(data)">
                  <template #reference>
                    <el-button link size="small" type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
            </span>
          </div>
        </template>
      </el-tree>

      <el-empty v-else-if="!loading" description="暂无目录数据，点击右上角「新增专业方向」开始搭建" />
    </div>

    <!-- 专业方向对话框 -->
    <el-dialog v-model="directionDialogVisible" :title="directionForm.specialty_id ? '编辑专业方向' : '新增专业方向'" width="460px" destroy-on-close>
      <el-form ref="directionFormRef" :model="directionForm" :rules="nameRules" label-width="90px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="directionForm.name" placeholder="如：叉车操作、叉车维修、安全规范、新能源电池" maxlength="30" show-word-limit />
        </el-form-item>
        <el-form-item label="编码" prop="code">
          <el-input v-model="directionForm.code" placeholder="可选，如 OPERATE/MAINTAIN/SAFETY/BATTERY" maxlength="30" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="directionDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitDirection">保存</el-button>
      </template>
    </el-dialog>

    <!-- 课程等级对话框（等级全局共享，不归属方向） -->
    <el-dialog v-model="levelDialogVisible" :title="levelForm.level_id ? '编辑课程等级' : '新增课程等级'" width="460px" destroy-on-close>
      <el-form ref="levelFormRef" :model="levelForm" :rules="nameRules" label-width="90px">
        <el-form-item label="等级名称" prop="name">
          <el-input v-model="levelForm.name" placeholder="如：入门、进阶、专项、认证" maxlength="30" show-word-limit />
        </el-form-item>
        <el-form-item label="编码" prop="code">
          <el-input v-model="levelForm.code" placeholder="可选，如 BASIC/ADVANCED/SPECIAL/CERTIFIED" maxlength="30" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="levelDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitLevel">保存</el-button>
      </template>
    </el-dialog>

    <!-- 课程对话框 -->
    <el-dialog v-model="courseDialogVisible" :title="courseForm.course_id ? '编辑课程' : '新增课程'" width="680px" destroy-on-close>
      <el-form ref="courseFormRef" :model="courseForm" :rules="courseRules" label-width="120px">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="课程名称" prop="name">
              <el-input v-model="courseForm.name" placeholder="请输入课程名称" maxlength="50" show-word-limit />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="所属方向" prop="specialty_id">
              <el-select v-model="courseForm.specialty_id" placeholder="请选择方向" style="width: 100%">
                <el-option v-for="d in directions" :key="d.specialty_id" :label="d.name" :value="d.specialty_id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="课程等级" prop="level_id">
              <el-select v-model="courseForm.level_id" placeholder="请选择等级" style="width: 100%">
                <el-option v-for="l in levelOptions" :key="l.level_id" :label="l.name" :value="l.level_id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="上架状态" prop="status">
              <el-radio-group v-model="courseForm.status">
                <el-radio :value="1">上架</el-radio>
                <el-radio :value="0">下架</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="理论学时" prop="theory_hours">
              <el-input-number v-model="courseForm.theory_hours" :min="0" :max="999" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="实操学时" prop="practice_hours">
              <el-input-number v-model="courseForm.practice_hours" :min="0" :max="999" style="width: 100%" />
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="前置课程" prop="prerequisite_course_ids">
              <el-select
                v-model="courseForm.prerequisite_course_ids"
                multiple
                filterable
                collapse-tags
                placeholder="选择需要先完成的课程（可多选）"
                style="width: 100%"
              >
                <el-option
                  v-for="c in courseOptions"
                  :key="c.course_id"
                  :label="c.name"
                  :value="c.course_id"
                  :disabled="c.course_id === courseForm.course_id"
                />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="证书模板" prop="certificate_template_id">
              <el-select v-model="courseForm.certificate_template_id" clearable placeholder="不关联则留空" style="width: 100%">
                <el-option v-for="t in certificateTemplates" :key="t.id" :label="t.name" :value="t.id" />
              </el-select>
            </el-form-item>
          </el-col>
          <el-col :span="24">
            <el-form-item label="课程简介" prop="description">
              <el-input v-model="courseForm.description" type="textarea" :rows="2" placeholder="请输入课程简介" maxlength="500" show-word-limit />
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
      <template #footer>
        <el-button @click="courseDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCourse">保存</el-button>
      </template>
    </el-dialog>

    <!-- 章节对话框 -->
    <el-dialog v-model="chapterDialogVisible" :title="chapterForm.chapter_id ? '编辑章节' : '新增章节'" width="520px" destroy-on-close>
      <el-form ref="chapterFormRef" :model="chapterForm" :rules="nameRules" label-width="90px">
        <el-form-item v-if="!chapterForm.chapter_id" label="所属课程" prop="course_id">
          <el-select v-model="chapterForm.course_id" placeholder="请选择课程" style="width: 100%">
            <el-option v-for="c in courseOptions" :key="c.course_id" :label="c.name" :value="c.course_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="章节标题" prop="name">
          <el-input v-model="chapterForm.name" placeholder="请输入章节标题" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="时长(分钟)" prop="duration">
          <el-input-number v-model="chapterForm.duration" :min="0" :max="9999" style="width: 100%" />
        </el-form-item>
        <el-form-item label="内容链接" prop="content_url">
          <el-input v-model="chapterForm.content_url" placeholder="外部内容链接（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="chapterDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitChapter">保存</el-button>
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
      <el-form ref="certificateFormRef" :model="certificateForm" :rules="nameRules" label-width="110px">
        <el-form-item label="模板名称" prop="name">
          <el-input v-model="certificateForm.name" placeholder="如：叉车维修初级工证书" maxlength="50" show-word-limit />
        </el-form-item>
        <el-form-item label="编码" prop="code">
          <el-input v-model="certificateForm.code" placeholder="可选" maxlength="30" />
        </el-form-item>
        <el-form-item label="有效期(天)" prop="validity_days">
          <el-input-number v-model="certificateForm.validity_days" :min="1" :max="36500" style="width: 100%" />
        </el-form-item>
        <el-form-item label="说明" prop="description">
          <el-input v-model="certificateForm.description" type="textarea" :rows="3" maxlength="300" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="certificateFormVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCertificate">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Plus, FolderOpened, Collection, Notebook, Document, Top, Bottom, Tickets } from '@element-plus/icons-vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { trainingApi } from '@/api/training'
import { adminApi, type CoursePayload, type ChapterPayload } from '@/api/admin'
import type { CatalogDirectionNode, CertificateTemplate } from '@/api/training'

type TreeNode = {
  __key: string
  __type: 'direction' | 'level' | 'course' | 'chapter'
  name: string
  code?: string
  sort_order?: number
  status?: number
  level_name?: string
  duration?: number
  content_url?: string
  specialty_id?: number
  level_id?: number
  course_id?: number
  chapter_id?: number
  theory_hours?: number
  practice_hours?: number
  prerequisite_course_ids?: number[]
  certificate_template_id?: number | null
  description?: string
  children?: TreeNode[]
  [key: string]: unknown
}

const loading = ref(false)
const submitting = ref(false)
const treeData = ref<TreeNode[]>([])
const directions = ref<CatalogDirectionNode[]>([])
const certificateTemplates = ref<CertificateTemplate[]>([])

// ===== 树构建 =====
function buildTree(tree: { specialties: CatalogDirectionNode[] }) {
  const nodes: TreeNode[] = (tree.specialties || []).map((d, di) => ({
    __key: `dir-${d.specialty_id}`,
    __type: 'direction' as const,
    name: d.name,
    code: d.code,
    sort_order: d.sort_order ?? di,
    specialty_id: d.specialty_id,
    children: (d.levels || []).map((l, li) => ({
      __key: `lvl-${l.level_id}`,
      __type: 'level' as const,
      name: l.name,
      code: l.code,
      sort_order: l.sort_order ?? li,
      specialty_id: d.specialty_id,
      level_id: l.level_id,
      children: (l.courses || []).map((c, ci) => ({
        __key: `crs-${c.course_id}`,
        __type: 'course' as const,
        name: c.name,
        status: c.status,
        sort_order: c.sort_order ?? ci,
        specialty_id: d.specialty_id,
        level_id: l.level_id,
        course_id: c.course_id,
        level_name: l.name,
        theory_hours: c.theory_hours,
        practice_hours: c.practice_hours,
        prerequisite_course_ids: c.prerequisite_course_ids,
        certificate_template_id: c.certificate_template_id,
        description: c.description,
        children: ((c as Record<string, unknown>).chapters as { chapter_id: number; title: string; order_num?: number; duration?: number }[] | undefined || []).map((ch, chi) => ({
          __key: `chp-${ch.chapter_id}`,
          __type: 'chapter' as const,
          name: ch.title,
          sort_order: ch.order_num ?? chi,
          duration: ch.duration,
          course_id: c.course_id,
          chapter_id: ch.chapter_id
        }))
      }))
    }))
  }))
  treeData.value = nodes
  directions.value = tree.specialties || []
}

// ===== 排序 =====
function siblingsOf(data: TreeNode): TreeNode[] {
  const type = data.__type
  if (type === 'direction') return treeData.value
  if (type === 'level') {
    const parent = treeData.value.find(d => d.specialty_id === data.specialty_id)
    return (parent?.children as TreeNode[] | undefined) || []
  }
  if (type === 'course') {
    const dir = treeData.value.find(d => d.specialty_id === data.specialty_id)
    const level = dir?.children?.find((l: TreeNode) => l.level_id === data.level_id)
    return (level?.children as TreeNode[] | undefined) || []
  }
  const dir = treeData.value.find(d => d.specialty_id === (data.specialty_id as number | undefined))
  const level = dir?.children?.find((l: TreeNode) => l.level_id === (data.level_id as number | undefined))
  const course = level?.children?.find((c: TreeNode) => c.course_id === data.course_id)
  return (course?.children as TreeNode[] | undefined) || []
}

function canMove(data: TreeNode, delta: number): boolean {
  const sibs = siblingsOf(data)
  const idx = sibs.findIndex(s => s.__key === data.__key)
  const target = idx + delta
  return target >= 0 && target < sibs.length
}

async function moveNode(data: TreeNode, delta: number) {
  const sibs = siblingsOf(data)
  const idx = sibs.findIndex(s => s.__key === data.__key)
  const target = sibs[idx + delta]
  if (!target) return
  submitting.value = true
  try {
    const thisOrder = data.sort_order ?? idx
    const targetOrder = target.sort_order ?? idx + delta
    if (data.__type === 'direction') {
      await trainingApi.updateDirection(data.specialty_id as number, { sort_order: targetOrder })
      await trainingApi.updateDirection(target.specialty_id as number, { sort_order: thisOrder })
    } else if (data.__type === 'level') {
      await trainingApi.updateLevel(data.level_id as number, { sort_order: targetOrder })
      await trainingApi.updateLevel(target.level_id as number, { sort_order: thisOrder })
    } else if (data.__type === 'course') {
      await adminApi.updateCourse(data.course_id as number, { sort_order: targetOrder } as CoursePayload)
      await adminApi.updateCourse(target.course_id as number, { sort_order: thisOrder } as CoursePayload)
    } else {
      await adminApi.updateChapter(data.chapter_id as number, { order_num: targetOrder } as ChapterPayload)
      await adminApi.updateChapter(target.chapter_id as number, { order_num: thisOrder } as ChapterPayload)
    }
    ElMessage.success('排序已更新')
    await loadTree()
  } catch (error) {
    console.error('排序失败:', error)
    ElMessage.error('排序更新失败')
  } finally {
    submitting.value = false
  }
}

// ===== 加载 =====
async function loadTree() {
  loading.value = true
  try {
    const res = await trainingApi.getAdminCatalogTree()
    if (res.code === 200) {
      buildTree(res.data)
    }
  } catch (error) {
    console.error('加载目录失败:', error)
    ElMessage.error('加载目录失败')
  } finally {
    loading.value = false
  }
}

async function loadCertificateTemplates() {
  try {
    const res = await trainingApi.getCertificateTemplates()
    if (res.code === 200) {
      certificateTemplates.value = res.data.certificate_templates || []
    }
  } catch (error) {
    console.error('加载证书模板失败:', error)
  }
}

// ===== 方向 =====
const directionDialogVisible = ref(false)
const directionFormRef = ref<FormInstance | null>(null)
const directionForm = reactive<{ specialty_id: number | null; name: string; code: string }>({
  specialty_id: null,
  name: '',
  code: ''
})

const nameRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }]
}

function resetDirectionForm() {
  directionForm.specialty_id = null
  directionForm.name = ''
  directionForm.code = ''
}

function openDirectionDialog(data?: TreeNode) {
  resetDirectionForm()
  if (data) {
    directionForm.specialty_id = data.specialty_id as number
    directionForm.name = data.name
    directionForm.code = data.code || ''
  }
  directionDialogVisible.value = true
}

async function submitDirection() {
  if (!directionFormRef.value) return
  await directionFormRef.value.validate()
  submitting.value = true
  try {
    if (directionForm.specialty_id) {
      const res = await trainingApi.updateDirection(directionForm.specialty_id, { name: directionForm.name, code: directionForm.code })
      if (res.code === 200) ElMessage.success('已更新')
    } else {
      const res = await trainingApi.createDirection({ name: directionForm.name, code: directionForm.code })
      if (res.code === 201) ElMessage.success('已创建')
    }
    directionDialogVisible.value = false
    await loadTree()
  } catch (error) {
    console.error('保存方向失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteDirection(data: TreeNode) {
  try {
    const res = await trainingApi.deleteDirection(data.specialty_id as number)
    if (res.code === 200) {
      ElMessage.success('已删除')
      await loadTree()
    }
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
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

function resetLevelForm() {
  levelForm.level_id = null
  levelForm.name = ''
  levelForm.code = ''
}

function openLevelDialog(level?: TreeNode | null) {
  resetLevelForm()
  if (level) {
    levelForm.level_id = level.level_id as number
    levelForm.name = level.name
    levelForm.code = level.code || ''
  }
  levelDialogVisible.value = true
}

async function submitLevel() {
  if (!levelFormRef.value) return
  await levelFormRef.value.validate()
  submitting.value = true
  try {
    if (levelForm.level_id) {
      const res = await trainingApi.updateLevel(levelForm.level_id, { name: levelForm.name, code: levelForm.code })
      if (res.code === 200) ElMessage.success('已更新')
    } else {
      const res = await trainingApi.createLevel({
        name: levelForm.name,
        code: levelForm.code
      })
      if (res.code === 201) ElMessage.success('已创建')
    }
    levelDialogVisible.value = false
    await loadTree()
  } catch (error) {
    console.error('保存等级失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteLevel(data: TreeNode) {
  try {
    const res = await trainingApi.deleteLevel(data.level_id as number)
    if (res.code === 200) {
      ElMessage.success('已删除')
      await loadTree()
    }
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
  }
}

// ===== 课程 =====
const courseDialogVisible = ref(false)
const courseFormRef = ref<FormInstance | null>(null)
const courseForm = reactive<Record<string, any>>({
  course_id: null,
  name: '',
  specialty_id: null,
  level_id: null,
  status: 1,
  theory_hours: 0,
  practice_hours: 0,
  prerequisite_course_ids: [],
  certificate_template_id: null,
  description: ''
})

const courseRules = {
  name: [{ required: true, message: '请输入课程名称', trigger: 'blur' }],
  specialty_id: [{ required: true, message: '请选择专业方向', trigger: 'change' }],
  level_id: [{ required: true, message: '请选择课程等级', trigger: 'change' }]
}

const levelOptions = computed(() => {
  const list: { level_id: number; name: string; specialty_id: number }[] = []
  for (const d of directions.value) {
    for (const l of d.levels || []) {
      list.push({ level_id: l.level_id, name: `${d.name} · ${l.name}`, specialty_id: d.specialty_id })
    }
  }
  return list
})

const courseOptions = computed(() => {
  const list: { course_id: number; name: string }[] = []
  for (const d of directions.value) {
    for (const l of d.levels || []) {
      for (const c of l.courses || []) {
        list.push({ course_id: c.course_id, name: `${d.name}/${l.name} · ${c.name}` })
      }
    }
  }
  return list
})

function resetCourseForm() {
  courseForm.course_id = null
  courseForm.name = ''
  courseForm.specialty_id = null
  courseForm.level_id = null
  courseForm.status = 1
  courseForm.theory_hours = 0
  courseForm.practice_hours = 0
  courseForm.prerequisite_course_ids = []
  courseForm.certificate_template_id = null
  courseForm.description = ''
}

function openCourseDialog(level?: TreeNode | null, course?: TreeNode | null) {
  resetCourseForm()
  if (course) {
    courseForm.course_id = course.course_id
    courseForm.name = course.name
    courseForm.specialty_id = course.specialty_id
    courseForm.level_id = course.level_id
    courseForm.status = course.status ?? 1
    courseForm.theory_hours = course.theory_hours ?? 0
    courseForm.practice_hours = course.practice_hours ?? 0
    courseForm.prerequisite_course_ids = course.prerequisite_course_ids || []
    courseForm.certificate_template_id = course.certificate_template_id ?? null
    courseForm.description = course.description || ''
  } else if (level) {
    courseForm.specialty_id = level.specialty_id
    courseForm.level_id = level.level_id
  }
  courseDialogVisible.value = true
}

async function submitCourse() {
  if (!courseFormRef.value) return
  await courseFormRef.value.validate()
  submitting.value = true
  try {
    const payload: Record<string, unknown> = {
      name: courseForm.name,
      specialty_id: courseForm.specialty_id,
      level_id: courseForm.level_id,
      status: courseForm.status,
      theory_hours: courseForm.theory_hours,
      practice_hours: courseForm.practice_hours,
      prerequisite_course_ids: courseForm.prerequisite_course_ids,
      certificate_template_id: courseForm.certificate_template_id,
      description: courseForm.description
    }
    let ok = false
    if (courseForm.course_id) {
      const res = await adminApi.updateCourse(courseForm.course_id, payload as unknown as CoursePayload)
      ok = res.code === 200
    } else {
      const res = await adminApi.createCourse(payload as unknown as CoursePayload)
      ok = res.code === 201
    }
    if (ok) {
      ElMessage.success(courseForm.course_id ? '课程已更新' : '课程已创建')
      courseDialogVisible.value = false
      await loadTree()
    }
  } catch (error) {
    console.error('保存课程失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteCourse(data: TreeNode) {
  try {
    const res = await adminApi.deleteCourse(data.course_id as number)
    if (res.code === 200) {
      ElMessage.success('已删除')
      await loadTree()
    }
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
  }
}

// ===== 章节 =====
const chapterDialogVisible = ref(false)
const chapterFormRef = ref<FormInstance | null>(null)
const chapterForm = reactive<{ chapter_id: number | null; course_id: number | null; name: string; duration: number; content_url: string }>({
  chapter_id: null,
  course_id: null,
  name: '',
  duration: 0,
  content_url: ''
})

function resetChapterForm() {
  chapterForm.chapter_id = null
  chapterForm.course_id = null
  chapterForm.name = ''
  chapterForm.duration = 0
  chapterForm.content_url = ''
}

function openChapterDialog(course?: TreeNode | null, chapter?: TreeNode | null) {
  resetChapterForm()
  if (chapter) {
    chapterForm.chapter_id = chapter.chapter_id as number
    chapterForm.course_id = chapter.course_id as number
    chapterForm.name = chapter.name
    chapterForm.duration = chapter.duration ?? 0
    chapterForm.content_url = chapter.content_url || ''
  } else if (course) {
    chapterForm.course_id = course.course_id as number
  }
  chapterDialogVisible.value = true
}

async function submitChapter() {
  if (!chapterFormRef.value) return
  await chapterFormRef.value.validate()
  submitting.value = true
  try {
    const payload: Record<string, unknown> = {
      title: chapterForm.name,
      duration: chapterForm.duration,
      content_url: chapterForm.content_url
    }
    let ok = false
    if (chapterForm.chapter_id) {
      const res = await adminApi.updateChapter(chapterForm.chapter_id, payload as unknown as ChapterPayload)
      ok = res.code === 200
    } else {
      if (!chapterForm.course_id) return
      const res = await adminApi.createChapter(chapterForm.course_id, payload as unknown as ChapterPayload)
      ok = res.code === 201
    }
    if (ok) {
      ElMessage.success(chapterForm.chapter_id ? '章节已更新' : '章节已创建')
      chapterDialogVisible.value = false
      await loadTree()
    }
  } catch (error) {
    console.error('保存章节失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteChapter(data: TreeNode) {
  try {
    const res = await adminApi.deleteChapter(data.chapter_id as number)
    if (res.code === 200) {
      ElMessage.success('已删除')
      await loadTree()
    }
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
  }
}

// ===== 证书模板 =====
const certificateDialogVisible = ref(false)
const certificateLoading = ref(false)
const certificateFormVisible = ref(false)
const certificateFormRef = ref<FormInstance | null>(null)
const certificateForm = reactive<{ id: number | null; name: string; code: string; validity_days: number | null; description: string }>({
  id: null,
  name: '',
  code: '',
  validity_days: null,
  description: ''
})

function openCertificateDialog() {
  certificateDialogVisible.value = true
  certificateLoading.value = true
  loadCertificateTemplates().finally(() => {
    certificateLoading.value = false
  })
}

function resetCertificateForm() {
  certificateForm.id = null
  certificateForm.name = ''
  certificateForm.code = ''
  certificateForm.validity_days = null
  certificateForm.description = ''
}

function openCertificateForm(template?: CertificateTemplate) {
  resetCertificateForm()
  if (template) {
    certificateForm.id = template.id
    certificateForm.name = template.name
    certificateForm.code = template.code || ''
    certificateForm.validity_days = template.validity_days ?? null
    certificateForm.description = template.description || ''
  }
  certificateFormVisible.value = true
}

async function submitCertificate() {
  if (!certificateFormRef.value) return
  await certificateFormRef.value.validate()
  submitting.value = true
  try {
    const payload = {
      name: certificateForm.name,
      code: certificateForm.code,
      validity_days: certificateForm.validity_days ?? undefined,
      description: certificateForm.description
    }
    if (certificateForm.id) {
      const res = await trainingApi.updateCertificateTemplate(certificateForm.id, payload)
      if (res.code === 200) ElMessage.success('模板已更新')
    } else {
      const res = await trainingApi.createCertificateTemplate(payload)
      if (res.code === 201) ElMessage.success('模板已创建')
    }
    certificateFormVisible.value = false
    await loadCertificateTemplates()
  } catch (error) {
    console.error('保存模板失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteCertificate(template: CertificateTemplate) {
  try {
    const res = await trainingApi.deleteCertificateTemplate(template.id)
    if (res.code === 200) {
      ElMessage.success('已删除')
      await loadCertificateTemplates()
    }
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
  }
}

function handleNodeClick(data: TreeNode) {
  if (data.__type === 'course') {
    openCourseDialog(null, data)
  }
}

onMounted(() => {
  loadTree()
  loadCertificateTemplates()
})
</script>

<style scoped>
.course-catalog-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.page-header h2 {
  font-size: 22px;
  color: #303133;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.catalog-tip {
  margin-bottom: 16px;
}

.catalog-body {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 12px 8px;
  min-height: 200px;
}

.catalog-tree {
  background: transparent;
}

.tree-node {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 2px 4px;
  border-radius: 4px;
  flex: 1;
  min-width: 0;
}

.tree-node:hover {
  background: #f5f7fa;
}

.node-label {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  cursor: pointer;
}

.node-icon {
  color: #909399;
  font-size: 15px;
}

.node-name {
  font-size: 14px;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-sub {
  font-size: 12px;
  color: #909399;
}

.node-actions {
  display: none;
  align-items: center;
  gap: 2px;
  margin-left: 12px;
  flex-shrink: 0;
}

.tree-node:hover .node-actions {
  display: inline-flex;
}

.certificate-header {
  margin-bottom: 12px;
  display: flex;
  justify-content: flex-end;
}

@media screen and (max-width: 768px) {
  .course-catalog-page {
    padding: 12px;
  }
  .page-header {
    flex-wrap: wrap;
    gap: 8px;
  }
  .tree-node {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
  .node-actions {
    flex-wrap: wrap;
    display: inline-flex;
  }
}
</style>
