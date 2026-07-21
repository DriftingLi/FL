<template>
  <div class="content-detail-page">
    <div class="container">
      <!-- 加载中 -->
      <div v-if="loading" v-loading="true" class="loading-placeholder"></div>

      <!-- 未找到 -->
      <div v-else-if="notFound" class="not-found">
        <el-empty description="文章不存在或已下架">
          <el-button type="primary" @click="goHome">返回首页</el-button>
        </el-empty>
      </div>

      <!-- 详情内容 -->
      <template v-else-if="detail">
        <!-- 文章头部 -->
        <header class="article-header">
          <span class="article-tag">{{ detail.category_label || categoryLabel(detail.category) }}</span>
          <h1 class="article-title">{{ detail.title }}</h1>
          <div class="article-meta">
            <span v-if="detail.source" class="meta-item">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/>
                <polyline points="22,6 12,13 2,6"/>
              </svg>
              来源：{{ detail.source }}
            </span>
            <span v-if="detail.published_at" class="meta-item">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10"/>
                <polyline points="12 6 12 12 16 14"/>
              </svg>
              {{ formatDate(detail.published_at) }}
            </span>
            <span class="meta-item">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                <circle cx="12" cy="12" r="3"/>
              </svg>
              阅读 {{ detail.view_count || 0 }}
            </span>
          </div>
        </header>

        <!-- 主体网格 -->
        <div class="article-layout">
          <article class="article-main">
            <div class="markdown-body" v-html="renderedContent"></div>

            <!-- 上一篇 / 下一篇 -->
            <nav class="article-nav">
              <div class="nav-item nav-prev">
                <router-link v-if="detail.prev" :to="`/content/${detail.prev.content_id}`" class="nav-link">
                  <span class="nav-label">
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="15 18 9 12 15 6"/></svg>
                    上一篇
                  </span>
                  <span class="nav-title">{{ detail.prev.title }}</span>
                </router-link>
                <div v-else class="nav-empty">
                  <span class="nav-label">上一篇</span>
                  <span class="nav-title-empty">没有更新的文章了</span>
                </div>
              </div>
              <div class="nav-item nav-next">
                <router-link v-if="detail.next" :to="`/content/${detail.next.content_id}`" class="nav-link">
                  <span class="nav-label">
                    下一篇
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 18 15 12 9 6"/></svg>
                  </span>
                  <span class="nav-title">{{ detail.next.title }}</span>
                </router-link>
                <div v-else class="nav-empty">
                  <span class="nav-label">下一篇</span>
                  <span class="nav-title-empty">没有更早的文章了</span>
                </div>
              </div>
            </nav>
          </article>

          <!-- 侧边栏：相关资讯 -->
          <aside class="article-sidebar">
            <h3 class="sidebar-title">
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
                <line x1="16" y1="13" x2="8" y2="13"/>
                <line x1="16" y1="17" x2="8" y2="17"/>
                <polyline points="10 9 9 9 8 9"/>
              </svg>
              相关资讯
            </h3>
            <ul class="related-list" v-if="detail.related && detail.related.length">
              <li v-for="item in detail.related" :key="item.content_id">
                <router-link :to="`/content/${item.content_id}`" class="related-item">
                  <div class="related-cover-wrap" v-if="item.cover_image">
                    <img :src="resolveFileUrl(item.cover_image)" :alt="item.title" class="related-cover" />
                  </div>
                  <div class="related-cover-wrap related-cover-placeholder" v-else>
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                      <rect x="3" y="3" width="18" height="18" rx="2" ry="2"/>
                      <circle cx="8.5" cy="8.5" r="1.5"/>
                      <polyline points="21 15 16 10 5 21"/>
                    </svg>
                  </div>
                  <div class="related-info">
                    <span class="related-title">{{ item.title }}</span>
                    <span class="related-date" v-if="item.published_at">{{ formatDate(item.published_at) }}</span>
                  </div>
                </router-link>
              </li>
            </ul>
            <div v-else class="related-empty">暂无相关资讯</div>
          </aside>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { marked } from 'marked'
import { featuredApi, categoryLabel } from '@/api/featured'
import { resolveFileUrl } from '@/utils/fileUrl'
import '@/assets/styles/markdown.css'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const notFound = ref(false)
const detail = ref<any>(null)

const renderedContent = computed(() => {
  if (!detail.value?.content) return ''
  try {
    return marked.parse(detail.value.content) as string
  } catch (e) {
    return detail.value.content
  }
})

function getIdFromRoute(): number {
  const id = Number(route.params.id)
  return isNaN(id) ? 0 : id
}

async function loadDetail() {
  const id = getIdFromRoute()
  if (!id) {
    notFound.value = true
    return
  }
  loading.value = true
  notFound.value = false
  detail.value = null
  try {
    const res = await featuredApi.getPublicDetail(id)
    if (res.code === 200 && res.data) {
      detail.value = res.data
      await nextTick()
      window.scrollTo(0, 0)
    } else {
      notFound.value = true
    }
  } catch (e: any) {
    notFound.value = true
  } finally {
    loading.value = false
  }
}

function formatDate(dateStr: string): string {
  if (!dateStr) return ''
  try {
    const d = new Date(dateStr)
    if (isNaN(d.getTime())) return ''
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
  } catch {
    return ''
  }
}

function goHome() {
  router.push('/')
}

onMounted(() => {
  loadDetail()
})

// 切换 id 时重新加载
watch(() => route.params.id, (newId) => {
  if (newId) loadDetail()
})
</script>

<style scoped>
.content-detail-page {
  padding: 120px 0 80px;
  background: var(--color-bg-page, #f8fafc);
  min-height: 100vh;
}

.content-detail-page .container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 var(--space-6, 24px);
}

.loading-placeholder {
  height: 400px;
  background: #fff;
  border-radius: var(--radius-lg, 12px);
}

.not-found {
  background: #fff;
  border-radius: var(--radius-lg, 12px);
  padding: 80px 24px;
  text-align: center;
}

/* 文章头部 */
.article-header {
  background: #fff;
  border-radius: var(--radius-lg, 12px);
  padding: 40px 48px;
  margin-bottom: 32px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
}

.article-tag {
  display: inline-block;
  background: var(--gradient-brand, linear-gradient(135deg, #2563eb, #7c3aed));
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  padding: 6px 14px;
  border-radius: var(--radius-full, 999px);
  margin-bottom: 16px;
  letter-spacing: 0.5px;
}

.article-title {
  font-family: var(--font-display, 'PingFang SC', sans-serif);
  font-size: 32px;
  font-weight: var(--font-bold, 700);
  color: var(--color-text-primary, #0f172a);
  line-height: 1.3;
  margin: 0 0 20px;
}

.article-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  font-size: 14px;
  color: var(--color-text-tertiary, #64748b);
}

.meta-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

/* 主体网格 */
.article-layout {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 32px;
  align-items: start;
}

.article-main {
  background: #fff;
  border-radius: var(--radius-lg, 12px);
  padding: 40px 48px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  min-width: 0;
}

/* 上一篇/下一篇 */
.article-nav {
  display: flex;
  justify-content: space-between;
  gap: 24px;
  margin-top: 48px;
  padding-top: 32px;
  border-top: 1px solid var(--color-border-light, #e2e8f0);
}

.nav-item {
  flex: 1;
  min-width: 0;
}

.nav-next {
  text-align: right;
}

.nav-link {
  display: inline-block;
  text-decoration: none;
  color: var(--color-text-primary, #0f172a);
  max-width: 100%;
  transition: color 0.2s;
}

.nav-link:hover {
  color: var(--color-primary-500, #2563eb);
}

.nav-label {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  color: var(--color-text-tertiary, #64748b);
  margin-bottom: 8px;
}

.nav-title {
  display: block;
  font-size: 15px;
  font-weight: 600;
  line-height: 1.5;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.nav-empty {
  color: var(--color-text-tertiary, #94a3b8);
}

.nav-title-empty {
  display: block;
  font-size: 14px;
  margin-top: 4px;
}

/* 侧边栏 */
.article-sidebar {
  background: #fff;
  border-radius: var(--radius-lg, 12px);
  padding: 24px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.06);
  position: sticky;
  top: 100px;
}

.sidebar-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-family: var(--font-display, 'PingFang SC', sans-serif);
  font-size: 18px;
  font-weight: 600;
  color: var(--color-text-primary, #0f172a);
  margin: 0 0 20px;
  padding-bottom: 16px;
  border-bottom: 2px solid var(--color-primary-500, #2563eb);
}

.related-list {
  list-style: none;
  padding: 0;
  margin: 0;
}

.related-list li {
  margin-bottom: 16px;
}

.related-list li:last-child {
  margin-bottom: 0;
}

.related-item {
  display: flex;
  gap: 12px;
  text-decoration: none;
  color: inherit;
  padding: 8px;
  margin: -8px;
  border-radius: var(--radius-md, 8px);
  transition: background 0.2s;
}

.related-item:hover {
  background: var(--color-bg-page, #f8fafc);
}

.related-cover-wrap {
  width: 80px;
  height: 60px;
  border-radius: var(--radius-md, 8px);
  overflow: hidden;
  flex-shrink: 0;
  background: var(--color-bg-page, #f1f5f9);
}

.related-cover {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.related-cover-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-text-tertiary, #94a3b8);
}

.related-info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
}

.related-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--color-text-primary, #0f172a);
  line-height: 1.4;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  transition: color 0.2s;
}

.related-item:hover .related-title {
  color: var(--color-primary-500, #2563eb);
}

.related-date {
  font-size: 12px;
  color: var(--color-text-tertiary, #94a3b8);
}

.related-empty {
  font-size: 14px;
  color: var(--color-text-tertiary, #94a3b8);
  text-align: center;
  padding: 32px 0;
}

/* 响应式 */
@media (max-width: 1024px) {
  .article-header,
  .article-main {
    padding: 32px 28px;
  }
  .article-title {
    font-size: 26px;
  }
}

@media (max-width: 768px) {
  .content-detail-page {
    padding: 90px 0 60px;
  }

  .article-layout {
    grid-template-columns: 1fr;
  }

  .article-sidebar {
    position: static;
    order: 2;
  }

  .article-main {
    order: 1;
    padding: 24px 20px;
  }

  .article-header {
    padding: 24px 20px;
    margin-bottom: 20px;
  }

  .article-title {
    font-size: 22px;
  }

  .article-meta {
    gap: 16px;
    font-size: 13px;
  }

  .article-nav {
    flex-direction: column;
    gap: 16px;
  }

  .nav-next {
    text-align: left;
  }
}
</style>
