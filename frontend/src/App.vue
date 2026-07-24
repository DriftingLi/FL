<template>
  <router-view v-slot="{ Component, route }">
    <transition name="fade-slide" mode="out-in">
      <!-- key 用最顶层匹配路由的路径，而非完整 route.path：
           同一 Layout 下切换子页面不会重新挂载整个 Layout，
           避免侧栏/顶栏等框架元素被反复销毁重建带来的闪烁与状态丢失 -->
      <component :is="Component" :key="route.matched[0]?.path || route.path" />
    </transition>
  </router-view>
</template>

<script setup lang="ts">
</script>

<style>
#app {
  width: 100%;
  min-height: 100vh;
}

.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1),
              transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
</style>
