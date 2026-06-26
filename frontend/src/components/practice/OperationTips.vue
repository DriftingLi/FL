<template>
  <div class="operation-tips">
    <el-tag type="info" size="small" effect="dark">
      🖱️ 左键旋转 | 右键平移 | 滚轮缩放 | 点击部件查看信息
    </el-tag>
  </div>
</template>

<style scoped>
.operation-tips {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
}

@media screen and (max-width: 768px) {
  .operation-tips {
    bottom: 8px;
  }

  .operation-tips :deep(.el-tag) {
    font-size: 11px;
  }
}
</style>
