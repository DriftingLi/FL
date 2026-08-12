<template>
  <div class="cc-layout">
    <!-- 左侧目录导航 -->
    <aside class="cc-sidebar">
      <!-- 卡片 1：管理动作 -->
      <div class="cc-card cc-actions-card">
        <el-button size="small" type="primary" plain @click="openDirectionDialog()">新增方向</el-button>
        <el-button size="small" plain @click="openLevelDialog()">新增等级</el-button>
        <el-button size="small" plain @click="openCertificateDialog()">证书模板</el-button>
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

    <!-- 课程编辑抽屉（全字段 + 章节管理）与弹窗表单已拆分为子组件 -->
    <CourseCatalogCourseDrawer
      ref="courseDrawerRef"
      :directions="directions"
      :levels="levels"
      :certificate-templates="certificateTemplates"
      :course-options="courseOptions"
      :default-specialty-id="filterSpecialty"
      :submitting="submitting"
      @saved="loadCourses"
    />
    <CourseCatalogDialogs
      ref="catalogDialogsRef"
      :certificate-templates="certificateTemplates"
      :submitting="submitting"
      @catalog-changed="loadCatalog"
      @certificates-changed="loadCertificateTemplates"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Search, ArrowDown, CaretTop, CaretBottom } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { trainingApi, type CatalogDirectionNode, type CatalogLevel, type CertificateTemplate } from '@/api/training'
import { adminApi, type AdminCourseItem } from '@/api/admin'
import { levelTagType } from '@/constants/level'
import CourseCatalogCourseDrawer from '@/components/admin/CourseCatalogCourseDrawer.vue'
import CourseCatalogDialogs from '@/components/admin/CourseCatalogDialogs.vue'

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

// ===== 子组件（抽屉/弹窗）入口 =====
const courseDrawerRef = ref<InstanceType<typeof CourseCatalogCourseDrawer> | null>(null)
const catalogDialogsRef = ref<InstanceType<typeof CourseCatalogDialogs> | null>(null)

function openDrawer(course?: AdminCourseItem | null) {
  courseDrawerRef.value?.open(course ?? null)
}

function openDirectionDialog(d?: CatalogDirectionNode | null) {
  catalogDialogsRef.value?.openDirectionDialog(d ?? null)
}

function openLevelDialog(l?: CatalogLevel | null) {
  catalogDialogsRef.value?.openLevelDialog(l ?? null)
}

function openCertificateDialog() {
  catalogDialogsRef.value?.openCertificateDialog()
}

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
    /* 错误已由拦截器提示 */
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
    /* 错误已由拦截器提示 */
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
    /* 错误已由拦截器提示 */
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
    /* 错误已由拦截器提示 */
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
    /* 错误已由拦截器提示 */
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
    /* 错误已由拦截器提示 */
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
}
</style>
