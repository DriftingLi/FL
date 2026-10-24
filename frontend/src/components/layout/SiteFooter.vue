<template>
  <!-- 站点页脚（#1756 第一批）：内容列底部细条，展示运营主体与备案号。
       形态取参照图的窄条布局：左侧品牌标识、右侧主体信息用竖线分隔，不做多列大页脚。
       落点由外壳决定（SidebarLayout 挂在内容列末尾、AuthPageShell 挂在整个认证页末尾），
       组件本身不带定位样式 —— 它只负责「说什么」，不负责「放哪里」。

       上边框的**线色**是个例外（#1811）：它和侧栏「收起侧栏」上方那条是**同一条外壳底缝**，
       必须走 --color-border-seam。此前用 border-line（= --color-border-light，#F1F5F9），
       与 --color-bg-page（#F8FAFC）几乎同值，浅色主题下这条线等于没画。 -->
  <footer class="site-footer mt-10 border-t border-[var(--color-border-seam)] px-6 py-4">
    <div
      class="site-footer-inner mx-auto flex max-w-[1280px] flex-wrap items-center justify-between gap-x-6 gap-y-2 text-xs text-ink-3"
    >
      <span class="flex items-center gap-2">
        <img :src="logoSrc" alt="HRWAI" class="h-5 w-5 rounded-ctl object-cover" />
        <span class="font-semibold tracking-[0.02em] text-ink-3">HRWAI</span>
      </span>
      <span class="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span>{{ company }}</span>
        <span aria-hidden="true" class="opacity-40">|</span>
        <a
          :href="icpLink"
          target="_blank"
          rel="noopener noreferrer"
          class="hover:text-ink-2"
        >{{ icp }}</a>
      </span>
    </div>
  </footer>
</template>

<script setup lang="ts">
// 主体与备案号是**部署事实**，不是业务配置：易道站挂 -3（主体备案号下的第 3 个站点），
// 和润的残值与门户各自挂自己的号。三处不同号是故意的，别来「统一」它们（见 ADR 记录）。
// logo 走常量而不是模板里的字面量 src：Vue 的 transformAssetUrls 会把 `src="/images/…"`
// 当成模块导入，测试环境（happy-dom）里解析不到 /images 会直接炸掉整个套件。
// 换 logo 时只改这一行（第二批会替换成新标识）。
const logoSrc = '/images/HRWAIlogo.jpg'
const company = '易道网络科技（广东）有限公司'
const icp = '粤ICP备2026005936号-3'
const icpLink = 'https://beian.miit.gov.cn/#/Integrated/index'
</script>
