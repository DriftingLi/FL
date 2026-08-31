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
        <UiButton variant="primary" @click="downloadFile">下载文件</UiButton>
      </el-empty>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { resolveFileUrl } from '@/utils/fileUrl'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps({
  src: { type: String, required: true },
  fileName: { type: String, default: '' }
})

const resolvedSrc = computed(() => resolveFileUrl(props.src))

const tableData = ref<Record<string, string>[]>([])
const columns = ref<string[]>([])

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

function parseCsv(text: string) {
  const lines = text.split('\n').filter((line: string) => line.trim())
  if (lines.length === 0) return

  const headers = lines[0].split(',').map((h: string) => h.trim().replace(/^"|"$/g, ''))
  columns.value = headers

  tableData.value = lines.slice(1).map((line: string) => {
    const values = line.split(',').map((v: string) => v.trim().replace(/^"|"$/g, ''))
    const row: Record<string, string> = {}
    headers.forEach((h: string, i: number) => {
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
