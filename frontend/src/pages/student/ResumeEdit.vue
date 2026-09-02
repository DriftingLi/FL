<template>
  <div class="mx-auto max-w-[560px] p-4">
    <UiCard padding="lg" class="resume-card">
      <div class="mb-4 flex items-center justify-between">
        <UiButton variant="text" @click="goBack">返回预览</UiButton>
        <div class="text-sm font-semibold text-ink">编辑简历</div>
      </div>
      <UiErrorState
        v-if="loadError"
        title="简历加载失败"
        description="网络或服务端异常，可重试"
        :retrying="false"
        @retry="load()"
      />
      <el-form v-else v-loading="loading" label-width="96px" @submit.prevent>
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
        <el-form-item label="PDF附件">
          <div class="text-xs text-ink-3">
            <template v-if="form.resume_file_url">
              <a :href="form.resume_file_url" target="_blank" class="text-ui-600">查看当前附件</a>
              <span>（上传/更换/删除请在预览页操作区进行）</span>
            </template>
            <span v-else>未上传附件</span>
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
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { resumeApi } from '@/api/resume'
import { unwrappedRequest } from '@/api/request'
import { buildCityLevelRegionOptions, splitRegionPath, regionElementsToPaths, cascaderToRegionStrings } from '@/utils/region'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiErrorState from '@/components/ui/UiErrorState.vue'

const router = useRouter()
const saving = ref(false)
const loading = ref(false)
const loadError = ref(false)
const photoInput = ref<HTMLInputElement | null>(null)
const regionOptions = buildCityLevelRegionOptions()
const regionCascader = ref<string[]>([])
const expectedRegionsCascader = ref<string[][]>([])

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
  photos: []
})

const positions = ref<any[]>([])
const credentials = ref<any[]>([])

function goBack() {
  router.push({ name: 'StudentResume' })
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
function triggerPhoto() {
  photoInput.value?.click()
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

async function load() {
  loading.value = true
  loadError.value = false
  try {
    const data: any = await resumeApi.get()
    if (!data) return
    form.real_name = data.real_name || ''
    form.contact_phone = data.contact_phone || ''
    form.wechat = data.wechat || ''
    form.region = data.region || ''
    form.expected_position_id = data.expected_position_id ?? null
    form.expected_regions = data.expected_regions || []
    // #486 回显：按 / 拆路径（直辖市一段）
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
  } catch (e: any) {
    if (String(e?.message || '').includes('不存在')) {
      // 未建简历：空表单，保存即创建
    } else {
      loadError.value = true
    }
  } finally {
    loading.value = false
  }
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
    router.push({ name: 'StudentResume' })
  } catch {} finally {
    saving.value = false
  }
}

// #486：现居地单选（存「省/市」串）；意向地区多选（每项「省/市」串）
function onRegionChange(val: any) {
  form.region = Array.isArray(val) && val.length ? val.join('/') : ''
}
function onRegionsChange(val: any) {
  form.expected_regions = cascaderToRegionStrings(val || [])
}

onMounted(async () => {
  try {
    const res: any = await unwrappedRequest.get('/positions', { headers: { 'X-Silent': '1' } })
    positions.value = res?.positions || []
  } catch {}
  try {
    const res: any = await unwrappedRequest.get('/credentials')
    credentials.value = res?.credentials || []
  } catch {}
  load()
})
</script>
