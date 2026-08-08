<template>
  <div class="cc-layout">
    <!-- 左侧目录导航 -->
    <aside class="cc-sidebar">
      <!-- 卡片 1：管理动作 -->
      <div class="cc-card cc-actions-card">
        <el-button size="small" type="primary" plain @click="openDirectionDialog()">新增方向</el-button>
        <el-button size="small" plain @click="openLevelDialog()">新增等级</el-button>
        <el-button size="small" plain @click="certificateDialogVisible = true">证书模板</el-button>
      </div>
      <!-- 卡片 2：专业方向 -->
      <div class="cc-card">
        <div class="cc-card-title">专业方向</div>
        <button
          class="cc-nav-item"
          :class="{ active: filterSpecialty === null }"
          @click="selectSpecialty(null)"
        >
          <span class="cc-nav-name">全部课程</span>
          <span class="cc-nav-count">{{ allCourses.length }}</span>
        </button>

        <div v-for="d in directions" :key="d.specialty_id" class="cc-nav-row">
          <button
            class="cc-nav-item"
            :class="{ active: filterSpecialty === d.specialty_id }"
            @click="selectSpecialty(d.specialty_id)"
          >
            <span class="cc-nav-name">{{ d.name }}</span>
            <span class="cc-nav-count">{{ countOfDirection(d.specialty_id) }}</span>
          </button>
          <span class="cc-nav-move">
            <el-icon class="cc-move-icon" @click.stop="moveDirection(d, -1)"><CaretTop /></el-icon>
            <el-icon class="cc-move-icon" @click.stop="moveDirection(d, 1)"><CaretBottom /></el-icon>
          </span>
        </div>

        <button
          v-if="unmountedCourses.length > 0"
          class="cc-nav-item cc-nav-warn"
          :class="{ active: filterSpecialty === -1 }"
          @click="selectSpecialty(-1)"
        >
          <span class="cc-nav-name">未挂载课程</span>
          <span class="cc-nav-count">{{ unmountedCourses.length }}</span>
        </button>
      </div>

      <!-- 卡片 3：课程等级 -->
      <div class="cc-card">
        <div class="cc-card-title">课程等级</div>
        <button
          class="cc-nav-item"
          :class="{ active: filterLevel === null }"
          @click="selectLevel(null)"
        >
          <span class="cc-nav-name">全部等级</span>
          <span class="cc-nav-count">{{ scopedCourses.length }}</span>
        </button>

        <div v-for="l in levels" :key="l.level_id" class="cc-level-row">
          <button
            class="cc-nav-item"
            :class="{ active: filterLevel === l.level_id }"
            @click="selectLevel(l.level_id)"
          >
            <span class="cc-nav-name">{{ l.name }}</span>
            <span class="cc-nav-count">{{ countOfLevel(l.level_id) }}</span>
          </button>
          <span class="cc-nav-move">
            <el-icon class="cc-move-icon" @click.stop="moveLevel(l, -1)"><CaretTop /></el-icon>
            <el-icon class="cc-move-icon" @click.stop="moveLevel(l, 1)"><CaretBottom /></el-icon>
          </span>
        </div>
      </div>
    </aside>

    <!-- 右侧课程表格 -->
    <main class="cc-main">
      <div class="cc-toolbar">
        <el-input v-model="keyword" placeholder="搜索课程名称…" clearable class="cc-search" @input="currentPage = 1">
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <el-select v-model="filterStatus" placeholder="状态" clearable style="width: 120px" @change="currentPage = 1">
          <el-option label="已上架" :value="1" />
          <el-option label="未上架" :value="0" />
        </el-select>
        <el-button type="primary" @click="openDrawer()">新增课程</el-button>
      </div>

      <el-table :data="pagedCourses" v-loading="loading" style="width: 100%">
        <el-table-column label="课程名称" min-width="220" show-overflow-tooltip>
          <template #default="{ row }">
            <el-tag v-if="row.status === 0" type="info" size="small">草稿</el-tag>
            <el-tag v-if="isUnmounted(row)" type="danger" size="small" style="margin-left: 4px">待补全</el-tag>
            <span class="cc-cell-name">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="方向 / 等级" width="150">
          <template #default="{ row }">
            <template v-if="!isUnmounted(row)">
              <el-tag :type="levelTagType(levelNameOf(row.level_id))" size="small">{{ levelNameOf(row.level_id) }}</el-tag>
              <span class="cc-cell-dim">{{ specialtyNameOf(row.specialty_id) }}</span>
            </template>
            <span v-else class="cc-cell-warn">缺少方向/等级</span>
          </template>
        </el-table-column>
        <el-table-column prop="chapter_count" label="章节" width="70" align="center" />
        <el-table-column label="学时" width="130">
          <template #default="{ row }">
            <span class="cc-cell-dim">理论{{ row.theory_hours || 0 }} / 实操{{ row.practice_hours || 0 }}</span>
          </template>
        </el-table-column>
        <el-table-column label="证书" min-width="150" show-overflow-tooltip>
          <template #default="{ row }">
            <span class="cc-cell-dim">{{ certificateNameOf(row.certificate_template_id) || '—' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'info'" size="small">
              {{ row.status === 1 ? '已上架' : '未上架' }}
            </el-tag>
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
                  <el-dropdown-item command="edit">编辑</el-dropdown-item>
                  <el-dropdown-item command="toggle">
                    {{ row.status === 1 ? '下架' : '上架' }}
                  </el-dropdown-item>
                  <el-dropdown-item v-if="canSortCourses" command="moveUp">上移</el-dropdown-item>
                  <el-dropdown-item v-if="canSortCourses" command="moveDown">下移</el-dropdown-item>
                  <el-dropdown-item command="delete" divided>删除</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="暂无课程" :image-size="60" />
        </template>
      </el-table>

      <div class="cc-pagination" v-if="filteredCourses.length > pageSize">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="filteredCourses.length"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next"
        />
      </div>
    </main>

    <!-- 课程编辑抽屉（全字段 + 章节管理） -->
    <el-drawer v-model="drawerVisible" :title="drawerForm.course_id ? '编辑课程' : '新增课程'" size="560px">
      <el-form ref="courseFormRef" :model="drawerForm" :rules="courseRules" label-width="96px">
        <el-form-item label="课程名称" prop="name">
          <el-input v-model="drawerForm.name" placeholder="课程名称" maxlength="50" />
        </el-form-item>
        <el-form-item label="专业方向" prop="specialty_id">
          <el-select v-model="drawerForm.specialty_id" placeholder="必选" style="width: 100%">
            <el-option v-for="d in directions" :key="d.specialty_id" :label="d.name" :value="d.specialty_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="课程等级" prop="level_id">
          <el-select v-model="drawerForm.level_id" placeholder="必选" style="width: 100%">
            <el-option v-for="l in levels" :key="l.level_id" :label="l.name" :value="l.level_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="学时">
          <div class="cc-hours">
            <el-input-number v-model="drawerForm.theory_hours" :min="0" :max="999" controls-position="right" />
            <span class="cc-hours-unit">理论</span>
            <el-input-number v-model="drawerForm.practice_hours" :min="0" :max="999" controls-position="right" />
            <span class="cc-hours-unit">实操</span>
          </div>
        </el-form-item>
        <el-form-item label="课程时长">
          <el-input-number v-model="drawerForm.duration" :min="0" :step="10" controls-position="right" />
          <span class="cc-hours-unit">分钟</span>
        </el-form-item>
        <el-form-item label="证书模板">
          <el-select v-model="drawerForm.certificate_template_id" clearable placeholder="不关联" style="width: 100%">
            <el-option v-for="t in certificateTemplates" :key="t.id" :label="t.name" :value="t.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="前置课程">
          <el-select
            v-model="drawerForm.prerequisite_course_ids"
            multiple
            filterable
            collapse-tags
            placeholder="选择需先完成的课程"
            style="width: 100%"
          >
            <el-option
              v-for="c in courseOptions"
              :key="c.course_id"
              :label="c.name"
              :value="c.course_id"
              :disabled="c.course_id === drawerForm.course_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="课程简介">
          <el-input v-model="drawerForm.description" type="textarea" :rows="2" maxlength="500" />
        </el-form-item>
        <el-form-item label="上架状态">
          <el-radio-group v-model="drawerForm.status">
            <el-radio :value="1">上架</el-radio>
            <el-radio :value="0">下架</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>

      <div class="cc-chapters" v-if="drawerForm.course_id">
        <div class="cc-chapters-head">
          章节管理
          <el-button size="small" type="primary" plain @click="openChapterDialog()">新增章节</el-button>
        </div>
        <div v-for="(ch, i) in drawerChapters" :key="ch.chapter_id" class="cc-chapter-row">
          <span class="cc-chapter-idx">{{ i + 1 }}</span>
          <span class="cc-chapter-title">{{ ch.title }}</span>
          <el-link :underline="'never'" @click="openChapterDialog(ch)">编辑</el-link>
          <el-link :underline="'never'" @click="moveChapter(ch, -1)">上移</el-link>
          <el-link :underline="'never'" @click="moveChapter(ch, 1)">下移</el-link>
          <el-popconfirm title="确定删除该章节？" @confirm="handleDeleteChapter(ch)">
            <template #reference>
              <el-link type="danger" :underline="'never'">删除</el-link>
            </template>
          </el-popconfirm>
        </div>
        <div v-if="drawerChapters.length === 0" class="cc-chapters-empty">暂无章节</div>
      </div>

      <template #footer>
        <el-button @click="drawerVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="submitCourse">保存</el-button>
      </template>
    </el-drawer>

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

    <!-- 章节对话框 -->
    <el-dialog v-model="chapterDialogVisible" :title="chapterForm.chapter_id ? '编辑章节' : '新增章节'" width="520px" destroy-on-close>
      <el-form ref="chapterFormRef" :model="chapterForm" :rules="chapterRules" label-width="90px">
        <el-form-item label="章节标题" prop="title">
          <el-input v-model="chapterForm.title" placeholder="章节标题" maxlength="100" />
        </el-form-item>
        <el-form-item label="时长(分钟)">
          <el-input-number v-model="chapterForm.duration" :min="0" :max="9999" style="width: 100%" />
        </el-form-item>
        <el-form-item label="内容链接">
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Plus, Search, ArrowDown, CaretTop, CaretBottom } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import { trainingApi, type CatalogDirectionNode, type CatalogLevel, type CertificateTemplate } from '@/api/training'
import { adminApi, type AdminCourseItem, type ChapterPayload } from '@/api/admin'
import { levelTagType } from '@/constants/level'

const loading = ref(false)
const submitting = ref(false)

// ===== 数据源：管理端课程列表（客户端过滤/分页，课程规模小） =====
const allCourses = ref<AdminCourseItem[]>([])
const directions = ref<CatalogDirectionNode[]>([])
const levels = ref<CatalogLevel[]>([])
const certificateTemplates = ref<CertificateTemplate[]>([])

const filterSpecialty = ref<number | null>(null)
const filterLevel = ref<number | null>(null)
const filterStatus = ref<number | null>(null)
const keyword = ref('')
const currentPage = ref(1)
const pageSize = ref(10)

const unmountedCourses = computed(() => allCourses.value.filter(c => isUnmounted(c)))
const mountedCourses = computed(() => allCourses.value.filter(c => !isUnmounted(c)))

function isUnmounted(c: AdminCourseItem): boolean {
  return c.specialty_id === null || c.specialty_id === undefined || c.level_id === null || c.level_id === undefined
}

function countOfDirection(id: number): number {
  return allCourses.value.filter(c => c.specialty_id === id).length
}

// 当前方向筛选范围内的课程（等级卡片计数随方向筛选实时调整）
const scopedCourses = computed(() => {
  if (filterSpecialty.value === -1) return unmountedCourses.value
  if (filterSpecialty.value !== null) {
    return allCourses.value.filter(c => c.specialty_id === filterSpecialty.value)
  }
  return allCourses.value
})

function countOfLevel(id: number): number {
  return scopedCourses.value.filter(c => c.level_id === id).length
}

const filteredCourses = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return allCourses.value.filter(c => {
    if (filterSpecialty.value === -1) {
      if (!isUnmounted(c)) return false
    } else if (filterSpecialty.value !== null && c.specialty_id !== filterSpecialty.value) {
      return false
    }
    if (filterSpecialty.value !== -1 && filterLevel.value !== null && c.level_id !== filterLevel.value) {
      return false
    }
    if (filterStatus.value !== null && c.status !== filterStatus.value) return false
    if (k && !c.name.toLowerCase().includes(k)) return false
    return true
  })
})

const pagedCourses = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredCourses.value.slice(start, start + pageSize.value)
})

function levelNameOf(levelIdValue?: number | null) {
  if (!levelIdValue) return ''
  return levels.value.find(l => l.level_id === levelIdValue)?.name || ''
}

function specialtyNameOf(specialtyIdValue?: number | null) {
  if (!specialtyIdValue) return ''
  return directions.value.find(d => d.specialty_id === specialtyIdValue)?.name || ''
}

function certificateNameOf(id?: number | null) {
  if (!id) return ''
  return certificateTemplates.value.find(t => t.id === id)?.name || ''
}

function selectSpecialty(id: number | null) {
  filterSpecialty.value = id
  currentPage.value = 1
}

function selectLevel(id: number | null) {
  filterLevel.value = id
  currentPage.value = 1
}

// 课程排序仅在同时选中具体方向 + 具体等级时可用（全部/未挂载不提供）
const canSortCourses = computed(
  () => filterSpecialty.value !== null && filterSpecialty.value > 0 && filterLevel.value !== null
)

// ===== 排序（后端 swap 端点，同值默认也生效） =====
async function moveDirection(d: CatalogDirectionNode, delta: -1 | 1) {
  const sibs = directions.value
  const idx = sibs.findIndex(s => s.specialty_id === d.specialty_id)
  const target = sibs[idx + delta]
  if (!target) return
  submitting.value = true
  try {
    // trainingApi 已解包信封：成功即业务成功
    await trainingApi.swapDirection(d.specialty_id, target.specialty_id)
    ElMessage.success('排序已更新')
    await loadCatalog()
  } catch (error) {
    console.error('排序失败:', error)
    ElMessage.error('排序更新失败')
  } finally {
    submitting.value = false
  }
}

async function moveLevel(l: CatalogLevel, delta: -1 | 1) {
  const sibs = levels.value
  const idx = sibs.findIndex(s => s.level_id === l.level_id)
  const target = sibs[idx + delta]
  if (!target) return
  submitting.value = true
  try {
    // trainingApi 已解包信封：成功即业务成功
    await trainingApi.swapLevel(l.level_id, target.level_id)
    ElMessage.success('排序已更新')
    await loadCatalog()
  } catch (error) {
    console.error('排序失败:', error)
    ElMessage.error('排序更新失败')
  } finally {
    submitting.value = false
  }
}

async function moveCourse(row: AdminCourseItem, delta: -1 | 1) {
  if (isUnmounted(row)) return
  const sibs = filteredCourses.value.filter(
    c => !isUnmounted(c) && c.specialty_id === row.specialty_id && c.level_id === row.level_id
  )
  const idx = sibs.findIndex(c => c.course_id === row.course_id)
  const target = sibs[idx + delta]
  if (!target) return
  submitting.value = true
  try {
    await adminApi.swapCourse(row.course_id, target.course_id)
    ElMessage.success('排序已更新')
    await loadCourses()
  } catch (error) {
    console.error('排序失败:', error)
    ElMessage.error('排序更新失败')
  } finally {
    submitting.value = false
  }
}

async function moveChapter(ch: { chapter_id: number; order_num?: number }, delta: -1 | 1) {
  const idx = drawerChapters.value.findIndex(c => c.chapter_id === ch.chapter_id)
  const target = drawerChapters.value[idx + delta]
  if (!target) return
  submitting.value = true
  try {
    await adminApi.updateChapter(ch.chapter_id, { order_num: (target.order_num ?? idx + delta) + 0 } as ChapterPayload)
    await adminApi.updateChapter(target.chapter_id, { order_num: (ch.order_num ?? idx) + 0 } as ChapterPayload)
    ElMessage.success('排序已更新')
    await loadDrawerDetail()
  } catch (error) {
    console.error('排序失败:', error)
    ElMessage.error('排序更新失败')
  } finally {
    submitting.value = false
  }
}

// ===== 加载 =====
async function loadCourses() {
  loading.value = true
  try {
    const data = await adminApi.getCourses({ page: 1, page_size: 500 })
    if (data) {
      allCourses.value = data.courses || []
    }
  } catch (error) {
    console.error('加载课程失败:', error)
    ElMessage.error('加载课程失败')
  } finally {
    loading.value = false
  }
}

async function loadCatalog() {
  try {
    // trainingApi 已解包信封：成功直接返回业务负载
    const [treeData, levelsData] = await Promise.all([
      trainingApi.getAdminCatalogTree(),
      trainingApi.getLevels()
    ])
    directions.value = treeData.specialties || []
    levels.value = levelsData.levels || []
  } catch (error) {
    console.error('加载目录失败:', error)
  }
}

async function loadCertificateTemplates() {
  try {
    // trainingApi 已解包信封：成功直接返回业务负载
    const data = await trainingApi.getCertificateTemplates()
    certificateTemplates.value = data.certificate_templates || []
  } catch (error) {
    console.error('加载证书模板失败:', error)
  }
}

const courseOptions = computed(() =>
  mountedCourses.value.map(c => ({
    course_id: c.course_id,
    name: `${specialtyNameOf(c.specialty_id) || '?'}/${levelNameOf(c.level_id) || '?'} · ${c.name}`
  }))
)

// ===== 方向 =====
const directionDialogVisible = ref(false)
const directionFormRef = ref<FormInstance | null>(null)
const directionForm = reactive<{ specialty_id: number | null; name: string; code: string }>({
  specialty_id: null,
  name: '',
  code: ''
})

const nameRules = {
  name: [{ required: true, message: '请输入名称', trigger: 'blur' }],
  code: [{ required: true, message: '请输入唯一编码', trigger: 'blur' }]
}

function openDirectionDialog(d?: CatalogDirectionNode | null) {
  directionForm.specialty_id = d?.specialty_id ?? null
  directionForm.name = d?.name ?? ''
  directionForm.code = d?.code ?? ''
  directionDialogVisible.value = true
}

async function submitDirection() {
  if (!directionFormRef.value) return
  await directionFormRef.value.validate()
  submitting.value = true
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
    await loadCatalog()
  } catch (error) {
    console.error('保存方向失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
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
  submitting.value = true
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
    await loadCatalog()
  } catch (error) {
    console.error('保存等级失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

// ===== 课程（抽屉） =====
const drawerVisible = ref(false)
const courseFormRef = ref<FormInstance | null>(null)
const drawerChapters = ref<{ chapter_id: number; title: string; duration?: number; order_num?: number }[]>([])
const drawerForm = reactive<Record<string, any>>({
  course_id: null,
  name: '',
  specialty_id: null,
  level_id: null,
  status: 1,
  theory_hours: 0,
  practice_hours: 0,
  duration: 0,
  certificate_template_id: null,
  prerequisite_course_ids: [],
  description: ''
})

const courseRules = {
  name: [{ required: true, message: '请输入课程名称', trigger: 'blur' }],
  specialty_id: [{ required: true, message: '请选择专业方向', trigger: 'change' }],
  level_id: [{ required: true, message: '请选择课程等级', trigger: 'change' }]
}

async function openDrawer(course?: AdminCourseItem | null) {
  drawerVisible.value = true
  drawerChapters.value = []
  if (!course) {
    Object.assign(drawerForm, {
      course_id: null,
      name: '',
      specialty_id: filterSpecialty.value !== null && filterSpecialty.value > 0 ? filterSpecialty.value : null,
      level_id: null,
      status: 1,
      theory_hours: 0,
      practice_hours: 0,
      duration: 0,
      certificate_template_id: null,
      prerequisite_course_ids: [],
      description: ''
    })
    return
  }
  Object.assign(drawerForm, {
    course_id: course.course_id,
    name: course.name,
    specialty_id: course.specialty_id,
    level_id: course.level_id,
    status: course.status,
    theory_hours: course.theory_hours || 0,
    practice_hours: course.practice_hours || 0,
    duration: course.duration || 0,
    certificate_template_id: course.certificate_template_id,
    prerequisite_course_ids: course.prerequisite_course_ids || [],
    description: course.description || ''
  })
  await loadDrawerDetail()
}

async function loadDrawerDetail() {
  try {
    const detail = await adminApi.getCourseDetail(drawerForm.course_id)
    if (detail) {
      Object.assign(drawerForm, {
        name: detail.name ?? drawerForm.name,
        specialty_id: detail.specialty_id ?? null,
        level_id: detail.level_id ?? null,
        status: detail.status ?? drawerForm.status,
        theory_hours: detail.theory_hours ?? 0,
        practice_hours: detail.practice_hours ?? 0,
        duration: detail.duration ?? 0,
        certificate_template_id: detail.certificate_template_id ?? null,
        prerequisite_course_ids: detail.prerequisite_course_ids || [],
        description: detail.description ?? ''
      })
      drawerChapters.value = detail.chapters || []
    }
  } catch (error) {
    console.error('加载课程详情失败:', error)
  }
}

async function submitCourse() {
  if (!courseFormRef.value) return
  await courseFormRef.value.validate()
  submitting.value = true
  try {
    const payload = {
      name: drawerForm.name,
      specialty_id: drawerForm.specialty_id,
      level_id: drawerForm.level_id,
      status: drawerForm.status,
      theory_hours: drawerForm.theory_hours,
      practice_hours: drawerForm.practice_hours,
      duration: drawerForm.duration,
      certificate_template_id: drawerForm.certificate_template_id ?? 0,
      prerequisite_course_ids: drawerForm.prerequisite_course_ids,
      description: drawerForm.description
    }
    if (drawerForm.course_id) {
      await adminApi.updateCourse(drawerForm.course_id, payload)
      ElMessage.success('已更新')
    } else {
      await adminApi.createCourse(payload)
      ElMessage.success('已创建')
    }
    drawerVisible.value = false
    await loadCourses()
  } catch (error) {
    console.error('保存课程失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(row: AdminCourseItem) {
  if (isUnmounted(row)) {
    ElMessage.warning('未挂载方向/等级，需先补全才能上架')
    return
  }
  submitting.value = true
  try {
    await adminApi.updateCourse(row.course_id, { status: row.status === 1 ? 0 : 1 })
    ElMessage.success(row.status === 1 ? '已下架' : '已上架')
    await loadCourses()
  } catch (error) {
    console.error('切换状态失败:', error)
    ElMessage.error('操作失败')
  } finally {
    submitting.value = false
  }
}

function handleAction(cmd: string, row: AdminCourseItem) {
  switch (cmd) {
    case 'edit':
      openDrawer(row)
      break
    case 'toggle':
      toggleStatus(row)
      break
    case 'moveUp':
      moveCourse(row, -1)
      break
    case 'moveDown':
      moveCourse(row, 1)
      break
    case 'delete':
      ElMessageBox.confirm('确定删除该课程？', '提示', {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消'
      }).then(() => handleDeleteCourse(row)).catch(() => {})
      break
  }
}

async function handleDeleteCourse(row: AdminCourseItem) {
  try {
    await adminApi.deleteCourse(row.course_id)
    ElMessage.success('已删除')
    await loadCourses()
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
  }
}

// ===== 章节 =====
const chapterDialogVisible = ref(false)
const chapterFormRef = ref<FormInstance | null>(null)
const chapterForm = reactive<{ chapter_id: number | null; title: string; duration: number; content_url: string }>({
  chapter_id: null,
  title: '',
  duration: 0,
  content_url: ''
})

const chapterRules = {
  title: [{ required: true, message: '请输入章节标题', trigger: 'blur' }]
}

function openChapterDialog(ch?: { chapter_id: number; title: string; duration?: number; content_url?: string }) {
  chapterForm.chapter_id = ch?.chapter_id ?? null
  chapterForm.title = ch?.title ?? ''
  chapterForm.duration = ch?.duration ?? 0
  chapterForm.content_url = ch?.content_url ?? ''
  chapterDialogVisible.value = true
}

async function submitChapter() {
  if (!chapterFormRef.value) return
  await chapterFormRef.value.validate()
  submitting.value = true
  try {
    const payload = {
      title: chapterForm.title,
      duration: chapterForm.duration,
      content_url: chapterForm.content_url || undefined
    }
    if (chapterForm.chapter_id) {
      await adminApi.updateChapter(chapterForm.chapter_id, payload)
      ElMessage.success('已更新')
    } else {
      await adminApi.createChapter(drawerForm.course_id, payload)
      ElMessage.success('已创建')
    }
    chapterDialogVisible.value = false
    await loadDrawerDetail()
  } catch (error) {
    console.error('保存章节失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteChapter(ch: { chapter_id: number }) {
  try {
    await adminApi.deleteChapter(ch.chapter_id)
    ElMessage.success('已删除')
    await loadDrawerDetail()
  } catch (error) {
    console.error('删除章节失败:', error)
    ElMessage.error('删除失败')
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
  submitting.value = true
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
    await loadCertificateTemplates()
  } catch (error) {
    console.error('保存模板失败:', error)
    ElMessage.error('保存失败')
  } finally {
    submitting.value = false
  }
}

async function handleDeleteCertificate(tpl: CertificateTemplate) {
  try {
    // trainingApi 已解包信封：成功即业务成功
    await trainingApi.deleteCertificateTemplate(tpl.id)
    ElMessage.success('已删除')
    await loadCertificateTemplates()
  } catch (error) {
    console.error('删除模板失败:', error)
    ElMessage.error('删除失败')
  }
}

onMounted(() => {
  loadCourses()
  loadCatalog()
  loadCertificateTemplates()
})
</script>

<style scoped>
.cc-layout {
  display: flex;
  gap: var(--space-5);
  align-items: flex-start;
}

/* ===== 左侧导航：三卡片 ===== */
.cc-sidebar {
  width: 210px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.cc-card {
  background: var(--color-bg-card);
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-lg);
  padding: var(--space-3) var(--space-2);
}

.cc-actions-card {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: var(--space-2);
}

.cc-actions-card .el-button {
  width: 100%;
  margin-left: 0;
}

.cc-card-title {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
  padding: var(--space-1) var(--space-3);
  margin-bottom: var(--space-1);
}

.cc-nav-row {
  display: flex;
  align-items: center;
}

.cc-nav-row .cc-nav-item,
.cc-level-row .cc-nav-item {
  flex: 1;
  min-width: 0;
}

.cc-nav-move {
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  line-height: 1.1;
}

.cc-move-icon {
  cursor: pointer;
  font-size: 14px;
  color: var(--color-text-tertiary);
  transition: color var(--duration-fast) var(--ease-default);
}

.cc-move-icon:hover {
  color: var(--color-primary-600);
}

.cc-nav-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: var(--space-2) var(--space-3);
  margin-bottom: 2px;
  border: none;
  border-radius: var(--radius-md);
  background: transparent;
  cursor: pointer;
  font-family: var(--font-body);
  font-size: var(--text-sm);
  color: var(--color-text-primary);
  transition: background var(--duration-fast) var(--ease-default);
}

.cc-nav-item:hover {
  background: var(--color-bg-sidebar-hover);
}

.cc-nav-item.active {
  background: var(--color-primary-50);
  color: var(--color-primary-600);
  font-weight: var(--font-semibold);
}

.cc-nav-warn.active {
  background: var(--color-danger-light);
  color: var(--color-danger);
}

.cc-nav-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cc-nav-count {
  font-size: var(--text-xs);
  color: var(--color-text-tertiary);
}

.cc-level-row {
  display: flex;
  align-items: center;
}

/* ===== 右侧表格 ===== */
.cc-main {
  flex: 1;
  min-width: 0;
}

.cc-toolbar {
  display: flex;
  gap: var(--space-3);
  align-items: center;
  margin-bottom: var(--space-4);
}

.cc-search {
  max-width: 280px;
}

.cc-cell-name {
  margin-left: 4px;
}

.cc-cell-dim {
  color: var(--color-text-secondary);
  font-size: 13px;
  margin-left: 6px;
}

.cc-cell-warn {
  color: var(--color-danger);
  font-size: 13px;
}

.cc-pagination {
  display: flex;
  justify-content: flex-end;
  margin-top: var(--space-4);
}

/* ===== 抽屉 ===== */
.cc-hours {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cc-hours-unit {
  font-size: 12px;
  color: var(--color-text-tertiary);
}

.cc-chapters {
  margin-top: var(--space-4);
  border-top: 1px solid var(--color-border-light);
  padding-top: var(--space-3);
}

.cc-chapters-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: var(--text-sm);
  font-weight: var(--font-semibold);
  margin-bottom: var(--space-2);
}

.cc-chapter-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: var(--space-2) var(--space-1);
  border-bottom: 1px dashed var(--color-border-light);
  font-size: var(--text-sm);
}

.cc-chapter-idx {
  width: 20px;
  color: var(--color-text-tertiary);
}

.cc-chapter-title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cc-chapters-empty {
  color: var(--color-text-tertiary);
  font-size: var(--text-xs);
  padding: var(--space-3) 0;
}

.certificate-header {
  margin-bottom: var(--space-3);
}

@media (max-width: 900px) {
  .cc-layout {
    flex-direction: column;
    /* 纵向后子元素需横向拉伸，否则 .cc-main 宽度=内容宽度（宽表格）导致整页横向溢出 */
    align-items: stretch;
  }

  .cc-sidebar {
    width: 100%;
  }
}

@media screen and (max-width: 768px) {
  .cc-toolbar {
    flex-wrap: wrap;
  }

  .cc-search {
    flex: 1 1 100%;
    max-width: none;
  }

  .cc-hours {
    flex-wrap: wrap;
  }

  .cc-hours :deep(.el-input-number) {
    flex: 1;
    min-width: 90px;
  }
}
</style>
