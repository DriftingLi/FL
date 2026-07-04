<template>
  <div class="ai-assistant-page">
    <div v-if="loading" class="loading-overlay">
      <div class="loading-spinner"></div>
      <span class="loading-text">AI 助手加载中</span>
    </div>

    <div v-if="errorMsg" class="error-overlay">
      <div class="error-icon">!</div>
      <p class="error-msg">{{ errorMsg }}</p>
      <button class="retry-btn" @click="initSDK">重新加载</button>
    </div>

    <div
      v-show="!loading && !errorMsg"
      ref="sdkContainer"
      class="sdk-container"
    ></div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { cozeWebSDK } from '@coze/web-sdk/js'
import { aiApi } from '@/api/ai'

const sdkContainer = ref(null)
const loading = ref(true)
const errorMsg = ref('')

function cleanupSDK() {
  try {
    cozeWebSDK.destroy()
  } catch (e) {
    // ignore
  }
  if (sdkContainer.value) {
    sdkContainer.value.innerHTML = ''
  }
}

async function initSDK() {
  loading.value = true
  errorMsg.value = ''

  cleanupSDK()

  try {
    const res = await aiApi.getCozeToken()
    if (res.code !== 200) {
      errorMsg.value = res.message || '获取智能体配置失败'
      loading.value = false
      return
    }

    const { project_id } = res.data

    await nextTick()

    cozeWebSDK.init({
      projectId: project_id,
      container: sdkContainer.value,
      refreshToken: async () => {
        const tokenRes = await aiApi.getCozeToken()
        if (tokenRes.code === 200) {
          return tokenRes.data.token
        }
        return ''
      },
      style: 'width: 100%; height: 100%; border: none;',
      theme: 'light',
      onIframeReady: () => {
        loading.value = false
      },
      onTokenExpired: () => {
        console.warn('Coze SDK token expired')
      },
      onTokenInvalid: () => {
        errorMsg.value = 'Token 验证失败，请检查扣子智能体配置'
        loading.value = false
      },
      onNetworkError: () => {
        errorMsg.value = '网络连接失败，请检查网络后重试'
        loading.value = false
      }
    })
  } catch (error) {
    console.error('Failed to init Coze SDK:', error)
    errorMsg.value = 'AI 助手加载失败，请稍后重试'
    loading.value = false
  }
}

onMounted(() => {
  initSDK()
})

onUnmounted(() => {
  cleanupSDK()
})
</script>

<style scoped>
.ai-assistant-page {
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
  background: #f0f2f5;
}

.sdk-container {
  width: 100%;
  height: 100%;
}

.loading-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 20px;
  background: #f0f2f5;
  z-index: 10;
}

.loading-spinner {
  width: 36px;
  height: 36px;
  border: 3px solid #e0e5ec;
  border-top-color: #4f6ef7;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.loading-text {
  font-size: 14px;
  color: #8c95a6;
  letter-spacing: 0.5px;
}

.error-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 16px;
  background: #f0f2f5;
  z-index: 10;
}

.error-icon {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: #fee2e2;
  color: #ef4444;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 22px;
  font-weight: 700;
}

.error-msg {
  font-size: 14px;
  color: #6b7280;
  margin: 0;
  max-width: 320px;
  text-align: center;
  line-height: 1.6;
}

.retry-btn {
  padding: 8px 24px;
  border: none;
  border-radius: 8px;
  background: #4f6ef7;
  color: #fff;
  font-size: 14px;
  cursor: pointer;
  transition: background 0.2s;
}

.retry-btn:hover {
  background: #3b5de7;
}

@media screen and (max-width: 768px) {
  .ai-assistant-page {
    width: 100%;
    height: 100%;
  }
}
</style>
