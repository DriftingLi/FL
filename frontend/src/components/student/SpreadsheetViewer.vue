<template>
  <div class="spreadsheet-viewer">
    <div v-if="isCsv" class="csv-table-wrapper">
      <el-table :data="tableData" border stripe max-height="500" style="width: 100%">
        <el-table-column
          v-for="(col, index) in columns"
          :key="index"
          :prop="col"
          :label="col"
          min-width="120"
        />
      </el-table>
    </div>
    <div v-else class="spreadsheet-download">
      <el-empty description="该表格文件暂不支持在线预览">
        <el-button type="primary" @click="downloadFile">下载文件</el-button>
      </el-empty>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { resolveFileUrl } from '@/utils/fileUrl'

const props = defineProps({
  src: { type: String, required: true },
  fileName: { type: String, default: '' }
})

const resolvedSrc = computed(() => resolveFileUrl(props.src))

const tableData = ref([])
const columns = ref([])

const isCsv = computed(() => {
  const name = props.fileName || props.src
  return name.toLowerCase().endsWith('.csv')
})

onMounted(async () => {
  if (isCsv.value) {
    try {
      const response = await fetch(resolvedSrc.value)
      const text = await response.text()
      parseCsv(text)
    } catch (e) {
      console.error('Failed to load CSV:', e)
    }
  }
})

function parseCsv(text) {
  const lines = text.split('\n').filter(line => line.trim())
  if (lines.length === 0) return

  const headers = lines[0].split(',').map(h => h.trim().replace(/^"|"$/g, ''))
  columns.value = headers

  tableData.value = lines.slice(1).map(line => {
    const values = line.split(',').map(v => v.trim().replace(/^"|"$/g, ''))
    const row = {}
    headers.forEach((h, i) => {
      row[h] = values[i] || ''
    })
    return row
  })
}

function downloadFile() {
  const link = document.createElement('a')
  link.href = resolvedSrc.value
  link.download = props.fileName || ''
  link.click()
}
</script>

<style scoped>
.spreadsheet-viewer {
  width: 100%;
}

.csv-table-wrapper {
  padding: 0;
}

.spreadsheet-download {
  padding: 40px 20px;
}
</style>
