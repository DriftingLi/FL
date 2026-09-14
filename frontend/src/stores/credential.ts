import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { credentialApi, type CredentialDict, type GroupedCredentials } from '@/api/credential'

export const useCredentialStore = defineStore('credential', () => {
  const current: Ref<CredentialDict | null> = ref(null)
  const grouped: Ref<GroupedCredentials> = ref({ special_operation: [], skill_level: [] })
  const flatList: Ref<CredentialDict[]> = ref([])
  const loading: Ref<boolean> = ref(false)
  const initialized: Ref<boolean> = ref(false)

  async function loadGrouped() {
    loading.value = true
    try {
      const data = await credentialApi.listGrouped()
      grouped.value = data
      flatList.value = [...(data.special_operation || []), ...(data.skill_level || [])]
    } finally {
      loading.value = false
    }
  }

  async function loadFlat() {
    const data = await credentialApi.listCredentials()
    flatList.value = data.credentials || []
    // also populate grouped from flat if needed
    grouped.value = {
      special_operation: flatList.value.filter(c => c.category === 'special_operation'),
      skill_level: flatList.value.filter(c => c.category === 'skill_level')
    }
  }

  async function loadCurrent(): Promise<CredentialDict | null> {
    try {
      const data = await credentialApi.getCurrent()
      current.value = data.credential || null
      initialized.value = true
      return current.value
    } catch {
      current.value = null
      initialized.value = true
      return null
    }
  }

  async function ensureInitialized(): Promise<void> {
    if (initialized.value) return
    await loadCurrent()
  }

  async function switchTo(credentialId: number): Promise<CredentialDict> {
    const data = await credentialApi.setCurrent(credentialId)
    // PATCH /me/credential 成功必回字典（service 侧查不到证件即 400）；生成形状沿用与 GET 共用的
    // CurrentCredentialDTO（credential 可空），此处按端点事实收口。
    const dict = data.credential
    if (!dict) throw new Error('切换证件失败：响应未返回证件')
    current.value = dict
    // 同步到 auth 的 localStorage userInfo（如有）
    try {
      const raw = localStorage.getItem('userInfo')
      if (raw) {
        const info = JSON.parse(raw)
        info.current_credential_id = credentialId
        info.current_credential = dict
        localStorage.setItem('userInfo', JSON.stringify(info))
      }
    } catch {}
    return dict
  }

  async function initialize(): Promise<void> {
    await Promise.all([loadGrouped().catch(() => {}), loadCurrent().catch(() => {})])
  }

  return {
    current,
    grouped,
    flatList,
    loading,
    initialized,
    loadGrouped,
    loadFlat,
    loadCurrent,
    ensureInitialized,
    switchTo,
    initialize
  }
})
