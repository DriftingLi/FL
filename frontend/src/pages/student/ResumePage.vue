<template>
  <div class="mx-auto max-w-[560px] p-4">
    <UiCard padding="lg" class="resume-card">
      <div class="mb-4 flex items-center justify-between">
        <UiButton variant="text" @click="goBack">返回</UiButton>
        <div class="flex items-center gap-2">
          <span class="text-[13px] text-ink-2">公开给招聘方</span>
          <el-switch v-model="form.visibilityOpen" @change="toggleVisibility" />
        </div>
      </div>
      <div v-if="viewCount > 0" class="view-stats rounded-card border border-line bg-ui-50 px-4 py-3 mb-4 text-sm text-ink">
        近 7 天 {{ viewCount }} 家企业查看过你的简历
      </div>
      <div v-if="contactRequests.length > 0" class="rounded-card border border-line bg-panel p-4 mb-4">
        <div class="text-sm font-semibold text-ink mb-2">收到的简历查看申请</div>
        <div v-for="req in contactRequests" :key="req.id" class="border-t border-line py-2">
          <div class="flex items-center justify-between gap-2">
            <div class="text-sm text-ink">
              <div>{{ req.company_name }} · {{ req.contact_name }}</div>
              <div class="text-xs text-ink-3">附言：{{ req.message }}</div>
              <div class="text-xs text-ink-3">{{ statusLabel(req.status) }} · {{ req.created_at }}</div>
            </div>
            <div class="flex gap-1">
              <UiButton variant="primary" v-if="req.status === 'pending'" size="small" @click="approveReq(req.id)">同意</UiButton>
              <UiButton v-if="req.status === 'pending'" size="small" @click="rejectReq(req.id)">拒绝</UiButton>
              <UiButton variant="danger" v-if="req.status === 'approved'" size="small" @click="revokeReq(req.id)">撤回</UiButton>
            </div>
          </div>
          <!-- #487：已同意条目就地展开企业联系方式（含电话一键复制） -->
          <CompanyContactInfo
            v-if="req.status === 'approved'"
            :approved="true"
            :company-name="req.company_name"
            :contact-name="req.contact_name"
            :phone="req.contact_phone"
            :email="req.contact_email"
            :wechat="req.wechat"
          />
        </div>
      </div>
      <!-- #415：未建简历（契约内 404）呈空态引导；真实故障走可重试错误态 -->
      <UiEmptyState
        v-if="resumeMissing"
        title="简历尚未创建"
        description="完善后可被招聘企业看到"
        action-text="去完善"
        @action="resumeMissing = false"
      />
      <UiErrorState
        v-else-if="resumeError"
        title="简历加载失败"
        description="网络或服务端异常，可重试"
        :retrying="false"
        @retry="load()"
      />
      <el-form v-else label-width="96px" @submit.prevent>
        <el-form-item label="真实姓名">
          <el-input v-model="form.real_name" maxlength="20" placeholder="填写真实姓名" />
        </el-form-item>
        <el-form-item label="联系电话">
          <el-input v-model="form.contact_phone" maxlength="50" placeholder="给招聘方拨打的号码" />
        </el-form-item>
        <el-form-item label="微信">
          <el-input v-model="form.wechat" maxlength="50" placeholder="微信号" />
        </el-form-item>
        <el-form-item label="现居地">
          <el-cascader
            v-model="regionCascader"
            :options="regionOptions"
            :props="{ value: 'label', label: 'label', children: 'children' }"
            placeholder="选择省/市"
            clearable
            class="!w-full"
            @change="onRegionChange"
          />
        </el-form-item>
        <el-form-item label="期望岗位">
          <el-select v-model="form.expected_position_id" clearable placeholder="选择岗位">
            <el-option v-for="p in positions" :key="p.position_id" :label="p.name" :value="p.position_id" />
          </el-select>
        </el-form-item>
        <el-form-item label="意向地区">
          <el-cascader
            v-model="expectedRegionsCascader"
            :options="regionOptions"
            :props="{ value: 'label', label: 'label', children: 'children', multiple: true }"
            placeholder="选择省/市（可多选）"
            clearable
            class="!w-full"
            @change="onRegionsChange"
          />
        </el-form-item>
        <el-form-item label="期望薪资">
          <div class="flex items-center gap-2">
            <el-input-number v-model="form.salary_min" :min="0" placeholder="最低" controls-position="right" />
            <span class="text-ink-3">-</span>
            <el-input-number v-model="form.salary_max" :min="0" placeholder="最高" controls-position="right" />
            <el-checkbox v-model="form.salary_negotiable" label="面议" class="ml-2" />
          </div>
        </el-form-item>
        <el-form-item label="到岗时间">
          <el-select v-model="form.available_in" clearable placeholder="选择">
            <el-option label="随时" value="immediate" />
            <el-option label="1周内" value="1w" />
            <el-option label="2周内" value="2w" />
            <el-option label="1月内" value="1m" />
          </el-select>
        </el-form-item>
        <el-form-item label="用工性质">
          <el-select v-model="form.job_nature" clearable placeholder="选择">
            <el-option label="全职" value="fulltime" />
            <el-option label="兼职" value="parttime" />
            <el-option label="合同" value="contract" />
          </el-select>
        </el-form-item>
        <el-form-item label="工作年限">
          <el-input-number v-model="form.experience_years" :min="0" :max="50" />
        </el-form-item>
        <el-form-item label="自我介绍">
          <el-input v-model="form.self_intro" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="简要介绍" />
        </el-form-item>
        <el-form-item label="工作经历">
          <div class="flex w-full flex-col gap-2">
            <div v-for="(exp, idx) in form.resume_experiences" :key="idx" class="flex flex-wrap items-center gap-2">
              <el-input v-model="exp.company" placeholder="单位" class="min-w-[120px] flex-1" />
              <el-input v-model="exp.role" placeholder="岗位" class="min-w-[120px] flex-1" />
              <el-input v-model="exp.start_month" placeholder="开始年月" class="min-w-[120px] flex-1" />
              <el-input v-model="exp.end_month" placeholder="结束年月" class="min-w-[120px] flex-1" />
              <el-input v-model="exp.desc" placeholder="描述" class="min-w-[120px] flex-1" />
              <UiButton variant="text" @click="removeExp(idx)" class="text-bad">删除</UiButton>
            </div>
            <UiButton variant="primary" text @click="addExp">新增经历</UiButton>
          </div>
        </el-form-item>
        <el-form-item label="持证信息">
          <div class="flex w-full flex-col gap-2">
            <div v-for="(cert, idx) in form.resume_certifications" :key="idx" class="flex flex-wrap items-center gap-2">
              <el-select v-model="cert.credential_id" clearable placeholder="证件" class="min-w-[120px] flex-1">
                <el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" />
              </el-select>
              <el-input v-model="cert.cert_no" placeholder="证书编号" class="min-w-[120px] flex-1" />
              <el-input v-model="cert.expire_date" placeholder="有效期" class="min-w-[120px] flex-1" />
              <UiButton variant="text" @click="removeCert(idx)" class="text-bad">删除</UiButton>
            </div>
            <UiButton variant="primary" text @click="addCert">新增持证</UiButton>
          </div>
        </el-form-item>
        <el-form-item label="PDF简历">
          <div class="flex flex-wrap items-center gap-2">
            <UiButton @click="triggerPdf">选择 PDF</UiButton>
            <span v-if="form.resume_file_url" class="max-w-[200px] truncate text-xs text-ink-2">{{ form.resume_file_url }}</span>
            <input ref="pdfInput" type="file" accept=".pdf" hidden @change="onPdfChange" />
          </div>
        </el-form-item>
        <el-form-item label="工作照">
          <div class="flex flex-wrap items-center gap-2">
            <UiButton @click="triggerPhoto">选择图片</UiButton>
            <span class="text-xs text-ink-3">{{ form.photos.length }}/6</span>
            <input ref="photoInput" type="file" accept="image/*" hidden @change="onPhotoChange" />
            <div class="mt-2 flex w-full flex-col gap-1">
              <span v-for="(p, i) in form.photos" :key="i" class="flex items-center gap-2 text-xs">
                <span class="max-w-[200px] truncate text-ink-2">{{ p }}</span>
                <UiButton variant="text" @click="removePhoto(i)" class="text-bad">删除</UiButton>
              </span>
            </div>
          </div>
        </el-form-item>
        <el-form-item>
          <UiButton variant="primary" :loading="saving" @click="save">保存</UiButton>
          <UiButton @click="goBack">取消</UiButton>
        </el-form-item>
      </el-form>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { resumeApi } from '@/api/resume'
import { buildCityLevelRegionOptions, splitRegionPath, regionElementsToPaths, cascaderToRegionStrings } from '@/utils/region'
import { unwrappedRequest } from '@/api/request'
import UiEmptyState from '@/components/ui/UiEmptyState.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import CompanyContactInfo from '@/components/recruit/CompanyContactInfo.vue'

const router = useRouter()
const saving = ref(false)
// #415：未建简历（契约内 404）与真实故障分离——空态有引导，故障可重试。
const resumeMissing = ref(false)
const resumeError = ref(false)
const pdfInput = ref<HTMLInputElement | null>(null)
const photoInput = ref<HTMLInputElement | null>(null)

const form = reactive<any>({
  real_name: '',
  contact_phone: '',
  wechat: '',
  region: '',
  expected_position_id: null,
  expected_regions: [],
  salary_min: null,
  salary_max: null,
  salary_negotiable: false,
  available_in: '',
  job_nature: '',
  experience_years: 0,
  self_intro: '',
  resume_experiences: [],
  resume_certifications: [],
  resume_file_url: '',
  photos: [],
  visibilityOpen: false
})

const positions = ref<any[]>([])
const credentials = ref<any[]>([])
const regionOptions = buildCityLevelRegionOptions()
const regionCascader = ref<string[]>([])
const expectedRegionsCascader = ref<string[][]>([])
const viewCount = ref(0)

function goBack() {
  router.push({ name: 'StudentProfile' })
}

function statusLabel(s: string) {
  const m: Record<string, string> = { pending: '待处理', approved: '已同意', rejected: '已拒绝', revoked: '已撤回', expired: '已过期' }
  return m[s] || s
}

function addExp() {
  form.resume_experiences.push({ company: '', role: '', start_month: '', end_month: '', desc: '' })
}
function removeExp(idx: number) {
  form.resume_experiences.splice(idx, 1)
}
function addCert() {
  form.resume_certifications.push({ credential_id: null, cert_no: '', expire_date: '', image_urls: [] })
}
function removeCert(idx: number) {
  form.resume_certifications.splice(idx, 1)
}
function removePhoto(idx: number) {
  form.photos.splice(idx, 1)
}
function triggerPdf() {
  pdfInput.value?.click()
}
function triggerPhoto() {
  photoInput.value?.click()
}

async function onPdfChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (file.type !== 'application/pdf' && !file.name.toLowerCase().endsWith('.pdf')) {
    ElMessage.error('仅支持 PDF 文件')
    input.value = ''
    return
  }
  if (file.size > 50 * 1024 * 1024) {
    ElMessage.error('文件大小超出限制，最大允许50MB')
    input.value = ''
    return
  }
  const fd = new FormData()
  fd.append('file', file)
  try {
    const res: any = await resumeApi.uploadPdf(fd)
    form.resume_file_url = res.url || res.data?.url || ''
    ElMessage.success('上传成功')
  } catch {}
  input.value = ''
}

async function onPhotoChange(e: Event) {
  const input = e.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (form.photos.length >= 6) {
    ElMessage.warning('工作照最多 6 张')
    input.value = ''
    return
  }
  const fd = new FormData()
  fd.append('file', file)
  try {
    const res: any = await resumeApi.uploadImage(fd)
    const url = res.url || res.data?.url || ''
    if (url) form.photos.push(url)
    ElMessage.success('上传成功')
  } catch {}
  input.value = ''
}

async function toggleVisibility(val: boolean) {
  const vis = val ? 'open' : 'hidden'
  try {
    await resumeApi.updateVisibility(vis as any)
    ElMessage.success(val ? '已公开' : '已设为不公开')
  } catch {
    form.visibilityOpen = !val
  }
}

async function load() {
  resumeMissing.value = false
  resumeError.value = false
  try {
    const data: any = await resumeApi.get()
    if (!data) return
    form.real_name = data.real_name || ''
    form.contact_phone = data.contact_phone || ''
    form.wechat = data.wechat || ''
    form.region = data.region || ''
    form.expected_position_id = data.expected_position_id ?? null
    form.expected_regions = data.expected_regions || []
    // 回填地区级联（#486）：现居地按 / 拆路径（直辖市一段）；意向地区每个元素拆成路径
    regionCascader.value = data.region ? splitRegionPath(String(data.region)) : []
    expectedRegionsCascader.value = regionElementsToPaths(data.expected_regions || [])
    form.salary_min = data.salary_min ?? null
    form.salary_max = data.salary_max ?? null
    form.salary_negotiable = !!data.salary_negotiable
    form.available_in = data.available_in || ''
    form.job_nature = data.job_nature || ''
    form.experience_years = data.experience_years || 0
    form.self_intro = data.self_intro || ''
    form.resume_experiences = data.resume_experiences || []
    form.resume_certifications = data.resume_certifications || []
    form.resume_file_url = data.resume_file_url || ''
    form.photos = data.photos || []
    form.visibilityOpen = data.visibility === 'open'
  } catch (e) {
    // #415：契约内空态（404）显式表达「简历尚未创建」；其余故障进可重试错误态。
    const kind = (e as { kind?: string }).kind
    if (kind === 'notfound') {
      resumeMissing.value = true
    } else {
      resumeError.value = true
    }
  }
  try {
    const res: any = await unwrappedRequest.get('/positions', { headers: { 'X-Silent': '1' } })
    if (res?.positions) positions.value = res.positions
  } catch {}
  try {
    const res: any = await unwrappedRequest.get('/credentials')
    if (res?.credentials) credentials.value = res.credentials
  } catch {}
  try {
    const stats: any = await resumeApi.getViewStats()
    viewCount.value = stats?.count || 0
  } catch {}
}

async function save() {
  saving.value = true
  try {
    const payload: any = {
      real_name: form.real_name,
      contact_phone: form.contact_phone,
      wechat: form.wechat,
      region: form.region,
      expected_position_id: form.expected_position_id,
      expected_regions: form.expected_regions,
      salary_min: form.salary_min,
      salary_max: form.salary_max,
      salary_negotiable: form.salary_negotiable,
      available_in: form.available_in,
      job_nature: form.job_nature,
      experience_years: form.experience_years,
      self_intro: form.self_intro,
      resume_experiences: form.resume_experiences,
      resume_certifications: form.resume_certifications,
      resume_file_url: form.resume_file_url,
      photos: form.photos
    }
    await resumeApi.save(payload)
    ElMessage.success('保存成功')
  } catch {} finally {
    saving.value = false
  }
}

const contactRequests = ref<any[]>([])
const contactLoading = ref(false)

async function loadContactRequests() {
  contactLoading.value = true
  try {
    const res: any = await resumeApi.listContactRequests({ page: 1, page_size: 20 })
    contactRequests.value = res?.items || []
  } catch {}
  contactLoading.value = false
}
async function approveReq(id: number) {
  try { await resumeApi.approveContactRequest(id); ElMessage.success('已同意'); loadContactRequests() } catch (e: any) { ElMessage.error(e?.message || '操作失败') }
}
async function rejectReq(id: number) {
  try { await resumeApi.rejectContactRequest(id); ElMessage.success('已拒绝'); loadContactRequests() } catch (e: any) { ElMessage.error(e?.message || '操作失败') }
}
async function revokeReq(id: number) {
  try { await resumeApi.revokeContactRequest(id); ElMessage.success('已撤回'); loadContactRequests() } catch (e: any) { ElMessage.error(e?.message || '操作失败') }
}


// 地区选择器联动：现居地单选（存省/市字符串，兼容旧字段）
function onRegionChange(val: any) {
  if (Array.isArray(val) && val.length) {
    form.region = val.join('/')
  } else {
    form.region = ''
  }
}

// 意向地区多选：每项为 [省, 市] 数组，存「省/市」字符串数组（兼容 expected_regions 既有契约）
function onRegionsChange(val: any) {
  // #486：级联选择值为路径数组（如 ['江苏省','苏州市']），按 / 拼回存储串；直辖市为单段
  form.expected_regions = cascaderToRegionStrings(val || [])
}

onMounted(async () => {
  try {
    const res: any = await unwrappedRequest.get('/positions', { headers: { 'X-Silent': '1' } })
    positions.value = res?.positions || []
  } catch {}
  load()
  loadContactRequests()
})
</script>

