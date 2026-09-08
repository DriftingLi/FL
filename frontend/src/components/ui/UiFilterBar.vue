<template>
  <!--
    列表筛选栏：只统一外层容器（间距、换行、响应式堆叠）与左右分区，
    字段经 #filters / #actions 插槽由各页自写（ADR-0035 控件收敛）。
    不做字段 prop 化 —— 各页字段数与按钮语义不一致，prop 化会让组件持续膨胀。

    默认形态 = 现状最常见的 `mb-3 flex flex-wrap items-center gap-2`；
    右侧操作区 ml-auto 顶到最右。替代散落各页的 .filter-bar / .cc-toolbar / .toolbar 语义 class。
  -->
  <div class="mb-3 flex flex-wrap items-center gap-2">
    <div class="flex min-w-0 flex-wrap items-center gap-2">
      <slot name="filters" />
    </div>
    <div class="ml-auto flex items-center gap-2">
      <slot name="actions" />
    </div>
  </div>
</template>
