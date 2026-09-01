<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between">
      <h1 class="text-xl font-bold text-ink">职位管理</h1>
      <UiButton variant="primary" size="small" @click="openCreate">发布职位</UiButton>
    </div>

    <UiErrorState
      v-if="loadError"
      title="职位加载失败"
      description="网络或服务端异常，可重试"
      :retrying="retrying"
      @retry="handleRetry"
    />
    <UiSkeleton v-else-if="loading" variant="list" :count="4" />
    <div v-else-if="items.length === 0" class="rounded-card border border-line bg-panel p-8 text-center text-ink-3">
      暂无职位，点击右上角「发布职位」开始招聘
    </div>
    <div v-else class="grid gap-3">
      <div
        v-for="item in items"
        :key="String(item.id)"
        class="rounded-card border border-line bg-panel p-4"
      >
        <div class="flex items-start justify-between gap-3">
          <div class="min-w-0 flex-1">
            <div class="flex items-center gap-2">
              <span class="text-sm font-semibold text-ink">{{ item.title }}</span>
              <el-tag v-if="item.forced_offline" type="danger" size="small">已强制下架</el-tag>
              <el-tag v-else :type="item.status === 'open' ? 'success' : 'info'" size="small">{{ item.status === 'open' ? '招聘中' : '已下架' }}</el-tag>
            </div>
            <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-ink-3">
              <span v-if="item.specialty_name">{{ item.specialty_name }}</span>
              <span v-if="item.region">{{ item.region }}</span>
              <span v-if="item.salary_text">{{ item.salary_text }}</span>
              <span v-if="item.experience_req">经验：{{ item.experience_req }}</span>
              <span>发布于 {{ item.published_at }}</span>
            </div>
            <div v-if="item.offline_reason" class="mt-1 text-xs text-red-500">下架原因：{{ item.offline_reason }}</div>
          </div>
          <div class="flex shrink-0 items-center gap-2">
            <router-link :to="`/recruit/jobs/${item.id}/applications`" class="text-xs font-medium text-ui-600 hover:text-ui-700">投递列表</router-link>
            <UiButton size="small" @click="openEdit(item)">编辑</UiButton>
            <UiButton
              v-if="!item.forced_offline"
              size="small"
              :variant="item.status === 'open' ? 'info' : 'primary'"
              @click="toggleStatus(item)"
            >
              {{ item.status === 'open' ? '下架' : '上架' }}
            </UiButton>
          </div>
        </div>
      </div>
    </div>
    <div v-if="total > 0" class="flex justify-center">
      <el-pagination
        v-model:current-page="page"
        :page-size="pageSize"
        :total="total"
        layout="prev, pager, next"
        @current-change="handlePageChange"
      />
    </div>

    <el-dialog v-model="dialogVisible" :title="editing ? '编辑职位' : '发布职位'" width="560px" destroy-on-close>
      <el-form label-width="90px">
        <el-form-item label="职位名" required>
          <el-input v-model="form.title" maxlength="100" placeholder="如：叉车维修技师" />
        </el-form-item>
        <el-form-item label="专业方向" required>
          <el-select v-model="form.specialty_id" placeholder="选择专业方向" class="!w-full">
            <el-option v-for="s in specialties" :key="s.specialty_id" :label="s.name" :value="s.specialty_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="地区">
          <el-input v-model="form.region" maxlength="100" placeholder="如：江苏苏州" />
        </el-form-item>
        <el-form-item label="薪资">
          <div class="flex items-center gap-2">
            <el-input v-model.number="form.salary_min" type="number" placeholder="下限" class="!w-24" />
            <span>-</span>
            <el-input v-model.number="form.salary_max" type="number" placeholder="上限" class="!w-24" />
            <el-input v-model="form.salary_text" placeholder="如：6-9K / 面议" class="!w-32" />
          </div>
        </el-form-item>
        <el-form-item label="经验要求">
          <el-input v-model="form.experience_req" maxlength="100" placeholder="如：2年" />
        </el-form-item>
        <el-form-item label="职位描述">
          <el-input v-model="form.description" type="textarea" :rows="4" maxlength="5000" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <UiButton @click="dialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="submitting" @click="submit">保存</UiButton>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { jobApi, type JobPosting, type JobPostingInput } from '@/api/job'
import { trainingApi, type CatalogDirection } from '@/api/training'
import { useAsyncPage } from '@/composables/useAsyncPage'
import UiButton from '@/components/ui/UiButton.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiSkeleton from '@/components/ui/UiSkeleton.vue'

const items = ref<JobPosting[]>([])
const specialties = ref<CatalogDirection[]>([])
const dialogVisible = ref(false)
const editing = ref(false)
const submitting = ref(false)
const editingId = ref(0)

const form = reactive<JobPostingInput>({
  title: '',
  specialty_id: null,
  region: '',
  salary_min: null,
  salary_max: null,
  salary_text: '',
  experience_req: '',
  description: ''
})

const {
  loading,
  loadError,
  retrying,
  retry: handleRetry,
  total,
  page,
  pageSize,
  run: load,
  handlePageChange
} = useAsyncPage(async () => {
  const res = await jobApi.listMyJobs({ page: page.value, page_size: pageSize.value })
  items.value = res?.items || []
  total.value = res?.total || 0
})

function openCreate() {
  editing.value = false
  editingId.value = 0
  Object.assign(form, { title: '', specialty_id: null, region: '', salary_min: null, salary_max: null, salary_text: '', experience_req: '', description: '' })
  dialogVisible.value = true
}

function openEdit(item: JobPosting) {
  editing.value = true
  editingId.value = item.id
  Object.assign(form, {
    title: item.title,
    specialty_id: item.specialty_id ?? null,
    region: item.region,
    salary_min: item.salary_min ?? null,
    salary_max: item.salary_max ?? null,
    salary_text: item.salary_text,
    experience_req: item.experience_req,
    description: item.description
  })
  dialogVisible.value = true
}

async function submit() {
  if (!form.title.trim()) {
    ElMessage.warning('职位名不能为空')
    return
  }
  if (!form.specialty_id) {
    ElMessage.warning('请选择专业方向')
    return
  }
  submitting.value = true
  try {
    if (editing.value) {
      await jobApi.updateJob(editingId.value, { ...form })
      ElMessage.success('职位已更新')
    } else {
      await jobApi.createJob({ ...form })
      ElMessage.success('职位发布成功')
    }
    dialogVisible.value = false
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '保存失败')
  } finally {
    submitting.value = false
  }
}

async function toggleStatus(item: JobPosting) {
  try {
    const res = await jobApi.toggleJobStatus(item.id)
    ElMessage.success(res?.status === 'open' ? '职位已上架' : '职位已下架')
    load()
  } catch (e: any) {
    ElMessage.error(e?.message || '操作失败')
  }
}

onMounted(async () => {
  try {
    const tree: any = await trainingApi.getCatalogTree()
    specialties.value = tree?.specialties || []
  } catch {}
  load()
})
</script>