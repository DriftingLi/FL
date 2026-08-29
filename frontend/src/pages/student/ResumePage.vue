<template>
  <div class="resume-page"><div class="resume-card"><div class="resume-head"><el-button text @click="goBack">返回</el-button><div class="head-actions"><span class="vis-label">公开给招聘方</span><el-switch v-model="form.visibilityOpen" @change="toggleVisibility" /></div></div><el-form label-width="96px" @submit.prevent><el-form-item label="真实姓名"><el-input v-model="form.real_name" maxlength="20" placeholder="填写真实姓名" /></el-form-item><el-form-item label="联系电话"><el-input v-model="form.contact_phone" maxlength="50" placeholder="给招聘方拨打的号码" /></el-form-item><el-form-item label="微信"><el-input v-model="form.wechat" maxlength="50" placeholder="微信号" /></el-form-item><el-form-item label="现居地"><el-input v-model="form.region" maxlength="50" placeholder="现居城市" /></el-form-item><el-form-item label="期望岗位"><el-select v-model="form.expected_specialty_id" clearable placeholder="选择岗位"><el-option v-for="s in specialties" :key="s.specialty_id" :label="s.name" :value="s.specialty_id" /></el-select></el-form-item><el-form-item label="意向地区"><el-select v-model="form.expected_regions" multiple filterable allow-create placeholder="输入地区后回车"><el-option v-for="r in regionOptions" :key="r" :label="r" :value="r" /></el-select></el-form-item><el-form-item label="期望薪资"><div class="salary-row"><el-input-number v-model="form.salary_min" :min="0" placeholder="最低" controls-position="right" /><span class="salary-sep">-</span><el-input-number v-model="form.salary_max" :min="0" placeholder="最高" controls-position="right" /><el-checkbox v-model="form.salary_negotiable" label="面议" class="salary-check" /></div></el-form-item><el-form-item label="到岗时间"><el-select v-model="form.available_in" clearable placeholder="选择"><el-option label="随时" value="immediate" /><el-option label="1周内" value="1w" /><el-option label="2周内" value="2w" /><el-option label="1月内" value="1m" /></el-select></el-form-item><el-form-item label="用工性质"><el-select v-model="form.job_nature" clearable placeholder="选择"><el-option label="全职" value="fulltime" /><el-option label="兼职" value="parttime" /><el-option label="合同" value="contract" /></el-select></el-form-item><el-form-item label="工作年限"><el-input-number v-model="form.experience_years" :min="0" :max="50" /></el-form-item><el-form-item label="自我介绍"><el-input v-model="form.self_intro" type="textarea" :rows="4" maxlength="1000" show-word-limit placeholder="简要介绍" /></el-form-item><el-form-item label="工作经历"><div class="array-block"><div v-for="(exp, idx) in form.resume_experiences" :key="idx" class="array-row"><el-input v-model="exp.company" placeholder="单位" class="row-input" /><el-input v-model="exp.role" placeholder="岗位" class="row-input" /><el-input v-model="exp.start_month" placeholder="开始年月" class="row-input" /><el-input v-model="exp.end_month" placeholder="结束年月" class="row-input" /><el-input v-model="exp.desc" placeholder="描述" class="row-input" /><el-button text type="danger" @click="removeExp(idx)">删除</el-button></div><el-button text type="primary" @click="addExp">新增经历</el-button></div></el-form-item><el-form-item label="持证信息"><div class="array-block"><div v-for="(cert, idx) in form.resume_certifications" :key="idx" class="array-row"><el-select v-model="cert.credential_id" clearable placeholder="证件" class="row-input"><el-option v-for="c in credentials" :key="c.id" :label="c.name" :value="c.id" /></el-select><el-input v-model="cert.cert_no" placeholder="证书编号" class="row-input" /><el-input v-model="cert.expire_date" placeholder="有效期" class="row-input" /><el-button text type="danger" @click="removeCert(idx)">删除</el-button></div><el-button text type="primary" @click="addCert">新增持证</el-button></div></el-form-item><el-form-item label="PDF简历"><div class="upload-block"><el-button @click="triggerPdf">选择 PDF</el-button><span v-if="form.resume_file_url" class="upload-name">{{ form.resume_file_url }}</span><input ref="pdfInput" type="file" accept=".pdf" hidden @change="onPdfChange" /></div></el-form-item><el-form-item label="工作照"><div class="upload-block"><el-button @click="triggerPhoto">选择图片</el-button><span class="photo-count">{{ form.photos.length }}/6</span><input ref="photoInput" type="file" accept="image/*" hidden @change="onPhotoChange" /><div class="photo-list"><span v-for="(p, i) in form.photos" :key="i" class="photo-item"><span class="photo-url">{{ p }}</span><el-button text type="danger" @click="removePhoto(i)">删除</el-button></span></div></div></el-form-item><el-form-item><el-button type="primary" :loading="saving" @click="save">保存</el-button><el-button @click="goBack">取消</el-button></el-form-item></el-form></div></div>
</template>
<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { resumeApi } from '@/api/resume'
import { unwrappedRequest } from '@/api/request'
const router = useRouter()
const saving = ref(false)
const pdfInput = ref<HTMLInputElement | null>(null)
const photoInput = ref<HTMLInputElement | null>(null)
const form = reactive<any>({ real_name: '', contact_phone: '', wechat: '', region: '', expected_specialty_id: null, expected_regions: [], salary_min: null, salary_max: null, salary_negotiable: false, available_in: '', job_nature: '', experience_years: 0, self_intro: '', resume_experiences: [], resume_certifications: [], resume_file_url: '', photos: [], visibilityOpen: false })
const specialties = ref<any[]>([])
const credentials = ref<any[]>([])
const regionOptions = ref<string[]>([])
function goBack() { router.push({ name: 'StudentProfile' }) }
function addExp() { form.resume_experiences.push({ company: '', role: '', start_month: '', end_month: '', desc: '' }) }
function removeExp(idx: number) { form.resume_experiences.splice(idx, 1) }
function addCert() { form.resume_certifications.push({ credential_id: null, cert_no: '', expire_date: '', image_urls: [] }) }
function removeCert(idx: number) { form.resume_certifications.splice(idx, 1) }
function removePhoto(idx: number) { form.photos.splice(idx, 1) }
function triggerPdf() { pdfInput.value?.click() }
function triggerPhoto() { photoInput.value?.click() }
async function onPdfChange(e: Event) { const input = e.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; if (file.type !== 'application/pdf' && !file.name.toLowerCase().endsWith('.pdf')) { ElMessage.error('仅支持 PDF 文件'); input.value = ''; return } if (file.size > 50 * 1024 * 1024) { ElMessage.error('文件大小超出限制，最大允许50MB'); input.value = ''; return } const fd = new FormData(); fd.append('file', file); try { const res: any = await resumeApi.uploadPdf(fd); form.resume_file_url = res.url || res.data?.url || ''; ElMessage.success('上传成功') } catch {} input.value = '' }
async function onPhotoChange(e: Event) { const input = e.target as HTMLInputElement; const file = input.files?.[0]; if (!file) return; if (form.photos.length >= 6) { ElMessage.warning('工作照最多 6 张'); input.value = ''; return } const fd = new FormData(); fd.append('file', file); try { const res: any = await resumeApi.uploadImage(fd); const url = res.url || res.data?.url || ''; if (url) form.photos.push(url); ElMessage.success('上传成功') } catch {} input.value = '' }
async function toggleVisibility(val: boolean) { const vis = val ? 'open' : 'hidden'; try { await resumeApi.updateVisibility(vis as any); ElMessage.success(val ? '已公开' : '已设为不公开') } catch { form.visibilityOpen = !val } }
async function load() {
  try { const data: any = await resumeApi.get(); if (!data) return; form.real_name = data.real_name || ''; form.contact_phone = data.contact_phone || ''; form.wechat = data.wechat || ''; form.region = data.region || ''; form.expected_specialty_id = data.expected_specialty_id ?? null; form.expected_regions = data.expected_regions || []; form.salary_min = data.salary_min ?? null; form.salary_max = data.salary_max ?? null; form.salary_negotiable = !!data.salary_negotiable; form.available_in = data.available_in || ''; form.job_nature = data.job_nature || ''; form.experience_years = data.experience_years || 0; form.self_intro = data.self_intro || ''; form.resume_experiences = data.resume_experiences || []; form.resume_certifications = data.resume_certifications || []; form.resume_file_url = data.resume_file_url || ''; form.photos = data.photos || []; form.visibilityOpen = data.visibility === 'open' } catch {}
  try { const res: any = await unwrappedRequest.get('/catalog/tree'); if (res?.specialties) specialties.value = res.specialties } catch {}
  try { const res: any = await unwrappedRequest.get('/credentials'); if (res?.credentials) credentials.value = res.credentials } catch {}
}
async function save() { saving.value = true; try { const payload: any = { real_name: form.real_name, contact_phone: form.contact_phone, wechat: form.wechat, region: form.region, expected_specialty_id: form.expected_specialty_id, expected_regions: form.expected_regions, salary_min: form.salary_min, salary_max: form.salary_max, salary_negotiable: form.salary_negotiable, available_in: form.available_in, job_nature: form.job_nature, experience_years: form.experience_years, self_intro: form.self_intro, resume_experiences: form.resume_experiences, resume_certifications: form.resume_certifications, resume_file_url: form.resume_file_url, photos: form.photos }; await resumeApi.save(payload); ElMessage.success('保存成功') } catch {} finally { saving.value = false } }
onMounted(load)
</script>
<style scoped>
.resume-page { max-width: 560px; margin: 0 auto; padding: 16px; }
.resume-card { background: var(--color-bg-card); border-radius: 12px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.06); padding: 24px; }
.resume-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px; }
.head-actions { display: flex; align-items: center; gap: 8px; }
.vis-label { font-size: 13px; color: var(--color-text-secondary); }
.salary-row { display: flex; align-items: center; gap: 8px; }
.salary-sep { color: var(--color-text-tertiary); }
.salary-check { margin-left: 8px; }
.array-block { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.array-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.row-input { flex: 1; min-width: 120px; }
.upload-block { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.upload-name { font-size: 12px; color: var(--color-text-secondary); max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.photo-count { font-size: 12px; color: var(--color-text-tertiary); }
.photo-list { display: flex; flex-direction: column; gap: 4px; width: 100%; margin-top: 8px; }
.photo-item { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.photo-url { max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--color-text-secondary); }
</style>
