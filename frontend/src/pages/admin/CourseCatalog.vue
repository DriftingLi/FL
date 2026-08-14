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
      <FacetCard title="专业方向">
        <FacetItem
          :active="specialtyId === null"
          name="全部课程"
          :count="totalAll"
          @select="selectDirection(null)"
        />

        <div v-for="d in directions" :key="d.specialty_id" class="cc-nav-row">
          <FacetItem
            :active="specialtyId === d.specialty_id"
            :name="d.name"
            :count="countOfDirection(d.specialty_id)"
            @select="selectDirection(d.specialty_id)"
          />
          <span class="cc-nav-move">
            <el-icon class="cc-move-icon" @click.stop="moveDirection(d, -1)"><CaretTop /></el-icon>
            <el-icon class="cc-move-icon" @click.stop="moveDirection(d, 1)"><CaretBottom /></el-icon>
          </span>
        </div>

        <FacetItem
          v-if="unmountedCount > 0"
          warn
          :active="specialtyId === UNMOUNTED_SPECIALTY_ID"
          name="未挂载课程"
          :count="unmountedCount"
          @select="selectDirection(UNMOUNTED_SPECIALTY_ID)"
        />
      </FacetCard>

      <!-- 卡片 3：课程等级 -->
      <FacetCard title="课程等级">
        <FacetItem
          :active="levelId === null"
          name="全部等级"
          :count="scopedTotal"
          @select="selectLevel(null)"
        />

        <div v-for="l in levels" :key="l.level_id" class="cc-level-row">
          <FacetItem
            :active="levelId === l.level_id"
            :name="l.name"
            :count="countOfLevel(l.level_id)"
            @select="selectLevel(l.level_id)"
          />
          <span class="cc-nav-move">
            <el-icon class="cc-move-icon" @click.stop="moveLevel(l, -1)"><CaretTop /></el-icon>
            <el-icon class="cc-move-icon" @click.stop="moveLevel(l, 1)"><CaretBottom /></el-icon>
          </span>
        </div>
      </FacetCard>
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
      :default-specialty-id="specialtyId"
      :submitting="submitting"
      @saved="refreshCatalog"
    />
    <CourseCatalogDialogs
      ref="catalogDialogsRef"
      :certificate-templates="certificateTemplates"
      :submitting="submitting"
      @catalog-changed="refreshCatalog"
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
import { useCourseCatalog, UNMOUNTED_SPECIALTY_ID } from '@/composables/useCourseCatalog'
import FacetCard from '@/components/catalog/FacetCard.vue'
import FacetItem from '@/components/catalog/FacetItem.vue'
import CourseCatalogCourseDrawer from '@/components/admin/CourseCatalogCourseDrawer.vue'
import CourseCatalogDialogs from '@/components/admin/CourseCatalogDialogs.vue'

const loading = ref(false)
const submitting = ref(false)

// ===== 数据源：管理端课程列表（客户端过滤/分页，课程规模小） =====
const allCourses = ref<AdminCourseItem[]>([])
const certificateTemplates = ref<CertificateTemplate[]>([])

function isUnmounted(c: AdminCourseItem): boolean {
  return c.specialty_id === null || c.specialty_id === undefined || c.level_id === null || c.level_id === undefined
}

const {
  directions,
  levels,
  specialtyId,
  levelId,
  totalAll,
  scopedTotal,
  countOfDirection,
  countOfLevel,
  unmountedCount,
  selectDirection,
  selectLevel,
  fetchCatalog,
  levelNameOf,
  specialtyNameOf
} = useCourseCatalog({
  adapter: {
    async load() {
      const [treeData, levelsData, coursesData] = await Promise.all([
        trainingApi.getAdminCatalogTree(),
        trainingApi.getLevels(),
        adminApi.getCourses({ page: 1, page_size: 500 })
      ])
      allCourses.value = coursesData.courses || []
      return {
        directions: treeData.specialties || [],
        levels: levelsData.levels || [],
        items: (coursesData.courses || []).map(c => ({
          specialty_id: isUnmounted(c) ? null : (c.specialty_id ?? null),
          level_id: isUnmounted(c) ? null : (c.level_id ?? null),
          count: 1
        }))
      }
    }
  },
  bidirectional: false,
  onSelect: () => {
    currentPage.value = 1
  }
})

const mountedCourses = computed(() => allCourses.value.filter(c => !isUnmounted(c)))

const filterStatus = ref<number | null>(null)
const keyword = ref('')
const currentPage = ref(1)
const pageSize = ref(10)

const filteredCourses = computed(() => {
  const k = keyword.value.trim().toLowerCase()
  return allCourses.value.filter(c => {
    if (specialtyId.value === UNMOUNTED_SPECIALTY_ID) {
      if (!isUnmounted(c)) return false
    } else if (specialtyId.value !== null && c.specialty_id !== specialtyId.value) {
      return false
    }
    if (specialtyId.value !== UNMOUNTED_SPECIALTY_ID && levelId.value !== null && c.level_id !== levelId.value) {
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

function certificateNameOf(id?: number | null) {
  if (!id) return ''
  return certificateTemplates.value.find(t => t.id === id)?.name || ''
}

// 课程排序仅在同时选中具体方向 + 具体等级时可用（全部/未挂载不提供）
const canSortCourses = computed(
  () =>
    specialtyId.value !== null &&
    specialtyId.value !== UNMOUNTED_SPECIALTY_ID &&
    levelId.value !== null
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
    await refreshCatalog()
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
    await refreshCatalog()
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
    await refreshCatalog()
  } catch (error) {
    console.error('排序失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    submitting.value = false
  }
}

// ===== 加载 =====
// adapter 回填页面级 allCourses（表格/抽屉共用），计数组 items 由同一数据派生
async function refreshCatalog() {
  loading.value = true
  try {
    await fetchCatalog()
  } catch (error) {
    console.error('加载课程失败:', error)
    /* 错误已由拦截器提示 */
  } finally {
    loading.value = false
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
    await refreshCatalog()
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
    await refreshCatalog()
  } catch (error) {
    console.error('删除失败:', error)
    /* 错误已由拦截器提示 */
  }
}

onMounted(() => {
  refreshCatalog()
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
