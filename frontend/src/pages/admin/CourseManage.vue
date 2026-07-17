<template>
  <div class="course-manage-page">
    <div class="page-header">
      <h2>课程管理</h2>
      <el-button type="primary" @click="openCreateDialog">
        <el-icon><Plus /></el-icon> 新增课程
      </el-button>
    </div>

    <div class="filter-bar">
      <el-input
        v-model="searchKeyword"
        placeholder="搜索课程名称"
        clearable
        style="width: 240px"
        @clear="handleSearch"
        @keyup.enter="handleSearch"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
      <el-select v-model="filterCategory" placeholder="全部分类" clearable style="width: 160px" @change="handleSearch">
        <el-option label="基础理论" value="CATEGORY_01" />
        <el-option label="安全规范" value="CATEGORY_02" />
        <el-option label="实操技能" value="CATEGORY_03" />
        <el-option label="进阶提升" value="CATEGORY_04" />
      </el-select>
      <el-button @click="handleSearch">搜索</el-button>
    </div>

    <el-table :data="courses" v-loading="loading" stripe border style="width: 100%">
      <el-table-column prop="course_id" label="ID" width="70" align="center" />
      <el-table-column label="封面" width="100" align="center">
        <template #default="{ row }">
          <img
            v-if="row.cover_image"
            :src="row.cover_image"
            class="cover-thumb"
            @error="row.cover_image = ''"
          />
          <span v-else class="cover-placeholder-sm">{{ row.name.charAt(0) }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="课程名称" min-width="180" show-overflow-tooltip />
      <el-table-column label="分类" width="120" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="getCategoryTagType(row.category)">
            {{ getCategoryName(row.category) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="chapter_count" label="章节数" width="90" align="center" />
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
            {{ row.status === 1 ? '上架' : '下架' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="created_at" label="创建时间" width="180" align="center">
        <template #default="{ row }">
          {{ formatDate(row.created_at) }}
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" fixed="right" align="center">
        <template #default="{ row }">
          <el-button type="primary" link size="small" @click="openChapterDrawer(row)">
            章节
          </el-button>
          <el-button
            :type="row.status === 1 ? 'warning' : 'success'"
            link
            size="small"
            @click="toggleStatus(row)"
          >
            {{ row.status === 1 ? '下架' : '上架' }}
          </el-button>
          <el-popconfirm title="确定删除该课程？删除后不可恢复" @confirm="handleDelete(row.course_id)">
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
        @size-change="loadCourses"
        @current-change="loadCourses"
      />
    </div>

    <el-dialog
      v-model="dialogVisible"
      title="新增课程"
      width="560px"
      destroy-on-close
    >
      <el-form ref="formRef" :model="formData" :rules="formRules" label-width="100px">
        <el-form-item label="课程名称" prop="name">
          <el-input v-model="formData.name" placeholder="请输入课程名称" maxlength="50" show-word-limit />
        </el-form-item>
        <el-form-item label="课程分类" prop="category">
          <el-select v-model="formData.category" placeholder="请选择分类" style="width: 100%">
            <el-option label="基础理论 (CATEGORY_01)" value="CATEGORY_01" />
            <el-option label="安全规范 (CATEGORY_02)" value="CATEGORY_02" />
            <el-option label="实操技能 (CATEGORY_03)" value="CATEGORY_03" />
            <el-option label="进阶提升 (CATEGORY_04)" value="CATEGORY_04" />
          </el-select>
        </el-form-item>
        <el-form-item label="课程简介" prop="description">
          <el-input
            v-model="formData.description"
            type="textarea"
            placeholder="请输入课程简介"
            :rows="3"
            maxlength="500"
            show-word-limit
          />
        </el-form-item>
        <el-form-item label="封面图片URL" prop="cover_image">
          <el-input v-model="formData.cover_image" placeholder="请输入图片URL（可选）" />
        </el-form-item>
        <el-form-item label="预计时长(分钟)" prop="duration">
          <el-input-number v-model="formData.duration" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          创建
        </el-button>
      </template>
    </el-dialog>

    <el-drawer
      v-model="drawerVisible"
      :title="`章节管理 - ${currentCourse?.name || ''}`"
      direction="rtl"
      size="520px"
      destroy-on-close
    >
      <template #header>
        <div class="drawer-header">
          <span>章节管理</span>
          <el-button type="primary" size="small" @click="openChapterForm()">
            <el-icon><Plus /></el-icon> 新增章节
          </el-button>
        </div>
      </template>

      <div class="chapter-list" v-loading="chaptersLoading">
        <draggable
          v-model="chapters"
          item-key="chapter_id"
          handle=".drag-handle"
          @end="handleChapterReorder"
        >
          <template #item="{ element, index }">
            <div class="chapter-item">
              <div class="drag-handle"><el-icon><Rank /></el-icon></div>
              <div class="chapter-info">
                <span class="chapter-index">{{ index + 1 }}</span>
                <div class="chapter-detail">
                  <h4>{{ element.title }}</h4>
                  <span class="chapter-duration" v-if="element.duration">{{ element.duration }}分钟</span>
                </div>
              </div>
              <div class="chapter-actions">
                <el-popconfirm title="确定删除该章节？" @confirm="handleDeleteChapter(element.chapter_id)">
                  <template #reference>
                    <el-button type="danger" link size="small">删除</el-button>
                  </template>
                </el-popconfirm>
              </div>
            </div>
          </template>
        </draggable>

        <el-empty v-if="!chaptersLoading && chapters.length === 0" description="暂无章节，点击上方按钮添加" />
      </div>
    </el-drawer>

    <el-dialog
      v-model="chapterDialogVisible"
      title="新增章节"
      width="520px"
      destroy-on-close
    >
      <el-form ref="chapterFormRef" :model="chapterFormData" :rules="chapterFormRules" label-width="100px">
        <el-form-item label="章节标题" prop="title">
          <el-input v-model="chapterFormData.title" placeholder="请输入章节标题" maxlength="100" show-word-limit />
        </el-form-item>
        <el-form-item label="内容链接" prop="content_url">
          <el-input v-model="chapterFormData.content_url" placeholder="外部内容链接（可选）" />
        </el-form-item>
        <el-form-item label="预计时长(分钟)" prop="duration">
          <el-input-number v-model="chapterFormData.duration" :min="0" :max="9999" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="chapterDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="chapterSubmitting" @click="handleChapterSubmit">
          创建
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus, Search, Rank } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import draggable from 'vuedraggable'
import { adminApi } from '@/api/admin'

const loading = ref(false)
const courses = ref([])
const searchKeyword = ref('')
const filterCategory = ref('')
const currentPage = ref(1)
const pageSize = ref(10)
const total = ref(0)

const dialogVisible = ref(false)
const submitting = ref(false)
const formRef = ref(null)
const formData: Record<string, any> = reactive({
  name: '',
  category: '',
  description: '',
  cover_image: '',
  duration: 0
})

const formRules = {
  name: [{ required: true, message: '请输入课程名称', trigger: 'blur' }],
  category: [{ required: true, message: '请选择课程分类', trigger: 'change' }]
}

const drawerVisible = ref(false)
const currentCourse = ref(null)
const chaptersLoading = ref(false)
const chapters = ref([])

const chapterDialogVisible = ref(false)
const chapterSubmitting = ref(false)
const chapterFormRef = ref(null)
const chapterFormData = reactive({
  title: '',
  content_url: '',
  duration: 0
})

const chapterFormRules = {
  title: [{ required: true, message: '请输入章节标题', trigger: 'blur' }]
}

const categoryMap = {
  'CATEGORY_01': '基础理论',
  'CATEGORY_02': '安全规范',
  'CATEGORY_03': '实操技能',
  'CATEGORY_04': '进阶提升'
}

function getCategoryName(category) {
  return categoryMap[category] || '其他'
}

function getCategoryTagType(category) {
  const types = { 'CATEGORY_01': '', 'CATEGORY_02': 'success', 'CATEGORY_03': 'warning', 'CATEGORY_04': 'danger' }
  return types[category] || 'info'
}

function formatDate(dateStr) {
  if (!dateStr) return '-'
  const d = new Date(dateStr)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

async function loadCourses() {
  loading.value = true
  try {
    const params: Record<string, any> = {
      page: currentPage.value,
      page_size: pageSize.value
    }
    if (searchKeyword.value) params.keyword = searchKeyword.value
    if (filterCategory.value) params.category = filterCategory.value

    const res = await adminApi.getCourses(params)
    if (res.code === 200) {
      courses.value = res.data.courses
      total.value = res.data.total
    }
  } catch (error) {
    console.error('加载课程列表失败:', error)
    ElMessage.error('加载课程列表失败')
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  currentPage.value = 1
  loadCourses()
}

function resetForm() {
  formData.name = ''
  formData.category = ''
  formData.description = ''
  formData.cover_image = ''
  formData.duration = 0
}

function openCreateDialog() {
  resetForm()
  dialogVisible.value = true
}

async function handleSubmit() {
  if (!formRef.value) return
  await formRef.value.validate()

  submitting.value = true
  try {
    const res = await adminApi.createCourse(formData)
    if (res.code === 201) {
      ElMessage.success('课程创建成功')
      dialogVisible.value = false
      loadCourses()
    }
  } catch (error) {
    console.error('操作失败:', error)
    ElMessage.error('创建失败')
  } finally {
    submitting.value = false
  }
}

async function handleDelete(courseId) {
  try {
    const res = await adminApi.deleteCourse(courseId)
    if (res.code === 200) {
      ElMessage.success('课程已删除')
      loadCourses()
    }
  } catch (error) {
    console.error('删除失败:', error)
    ElMessage.error('删除失败')
  }
}

async function toggleStatus(row) {
  try {
    const newStatus = row.status === 1 ? 0 : 1
    const res = await adminApi.updateCourse(row.course_id, { status: newStatus })
    if (res.code === 200) {
      ElMessage.success(newStatus === 1 ? '已上架' : '已下架')
      row.status = newStatus
    }
  } catch (error) {
    console.error('状态更新失败:', error)
    ElMessage.error('状态更新失败')
  }
}

async function openChapterDrawer(course) {
  currentCourse.value = course
  drawerVisible.value = true
  chaptersLoading.value = true

  try {
    const res = await adminApi.getCourseDetail(course.course_id)
    if (res.code === 200) {
      chapters.value = res.data.chapters || []
    }
  } catch (error) {
    console.error('加载章节失败:', error)
    ElMessage.error('加载章节失败')
  } finally {
    chaptersLoading.value = false
  }
}

function resetChapterForm() {
  chapterFormData.title = ''
  chapterFormData.content_url = ''
  chapterFormData.duration = 0
}

function openChapterForm() {
  resetChapterForm()
  chapterDialogVisible.value = true
}

async function handleChapterSubmit() {
  if (!chapterFormRef.value) return
  await chapterFormRef.value.validate()

  chapterSubmitting.value = true
  try {
    const res = await adminApi.createChapter(currentCourse.value.course_id, chapterFormData)
    if (res.code === 201) {
      ElMessage.success('章节创建成功')
      chapterDialogVisible.value = false
      openChapterDrawer(currentCourse.value)
    }
  } catch (error) {
    console.error('创建失败:', error)
    ElMessage.error('创建失败')
  } finally {
    chapterSubmitting.value = false
  }
}

async function handleDeleteChapter(chapterId) {
  try {
    const res = await adminApi.deleteChapter(chapterId)
    if (res.code === 200) {
      ElMessage.success('章节已删除')
      openChapterDrawer(currentCourse.value)
    }
  } catch (error) {
    console.error('删除章节失败:', error)
    ElMessage.error('删除章节失败')
  }
}

async function handleChapterReorder() {
  for (let i = 0; i < chapters.value.length; i++) {
    const ch = chapters.value[i]
    if (ch.order_num !== i + 1) {
      try {
        await adminApi.updateChapter(ch.chapter_id, { order_num: i + 1 })
      } catch (e) {
        console.error('排序更新失败:', e)
      }
    }
  }
}

onMounted(() => {
  loadCourses()
})
</script>

<style scoped>
.course-manage-page {
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
  flex-wrap: wrap;
}

.cover-thumb {
  width: 60px;
  height: 40px;
  object-fit: cover;
  border-radius: 4px;
}

.cover-placeholder-sm {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 4px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
  font-weight: bold;
  font-size: 16px;
}

.pagination-wrapper {
  display: flex;
  justify-content: center;
  margin-top: 20px;
}

.drawer-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.chapter-list {
  padding: 0 10px;
}

.chapter-item {
  display: flex;
  align-items: center;
  padding: 12px;
  border-bottom: 1px solid #ebeef5;
  transition: background 0.2s;
}

.chapter-item:hover {
  background: #f5f7fa;
}

.drag-handle {
  cursor: move;
  color: #c0c4cc;
  margin-right: 12px;
  font-size: 18px;
}

.drag-handle:hover {
  color: #409eff;
}

.chapter-info {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 12px;
}

.chapter-index {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #e4e7ed;
  color: #606266;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 600;
}

.chapter-detail h4 {
  font-size: 14px;
  color: #303133;
  margin-bottom: 2px;
}

.chapter-duration {
  font-size: 12px;
  color: #909399;
}

.chapter-actions {
  display: flex;
  gap: 4px;
}

@media screen and (max-width: 768px) {
  .course-manage-page {
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

  .filter-bar .el-input,
  .filter-bar .el-select {
    width: 100% !important;
  }

  .el-table {
    overflow-x: auto;
  }
}
</style>
