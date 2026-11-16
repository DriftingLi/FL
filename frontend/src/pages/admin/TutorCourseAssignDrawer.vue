<!--
  讲师课程分配抽屉（#2597 / 规格 D4）。

  为什么是抽屉而不是独立页：这是**讲师行上的一个动作**（「给这个人分配哪些课」），
  离开讲师列表就丢了上下文。抽屉里同时要「看已分配」与「改已分配」，两种交互在
  一个窄面板里是自然的（独立页要两次跳转）。

  交互口径（D4）：
  - 勾选 = 想让他有；取消勾选 = 想让他没有。一次「确定」把**差集**同时提交：
    新增的走 assign，移除的走 remove。两条请求并发发（互不依赖），任一失败则
    整体算失败并保留抽屉——不让用户在半失败状态下猜哪一半生效了。
  - 已分配的行显示来源（「管理员分配」/「他自己认领」）：审计上这是两件事，
    管理员要知道自己能不能改（自助认领的可以覆盖成正式分配）。
  - 移除确认文案写明**不回溯**（B6）：已写章节的归属与在审内容不受影响。
-->
<template>
  <UiDialog
    v-model="visible"
    :title="`分配课程 · ${tutorName}`"
    width="620px"
    destroy-on-close
    confirm-text="保存"
    :confirm-loading="submitting"
    :confirm-disabled="loading"
    @confirm="handleConfirm"
  >
    <div class="assign-body">
      <div class="assign-toolbar">
        <UiInput
          v-model="keyword"
          placeholder="搜索课程名称"
          clearable
          style="width: 240px"
        >
          <template #prefix><UiIcon name="search" /></template>
        </UiInput>
        <span class="text-sm text-ink-soft">
          已选 {{ selectedIds.length }} 门 · 共 {{ courses.length }} 门
        </span>
      </div>

      <UiAsyncSection
        :loading="loading"
        :error="loadError !== null"
        :empty="isEmptyCourses"
        empty-text="没有可分配的课程"
        @retry="loadAll"
      >
        <div class="course-list">
          <UiCheckboxGroup v-model="selectedIds">
            <UiCheckbox
              v-for="course in filteredCourses"
              :key="course.course_id"
              :value="course.course_id"
            >
              <!-- 组内模式：`value` 是勾选值，显示文案走默认插槽。
                   不传 `label`——EP 的 `label` 在组内模式下历史上兼作 value，
                   两处都传会让「值是 id 还是名字」变得要靠 EP 版本猜。 -->
              <span class="course-name">{{ course.name }}</span>
              <UiTag v-if="assignedSource(course.course_id)" size="small" :tone="sourceTone(course.course_id)">
                {{ sourceLabel(course.course_id) }}
              </UiTag>
            </UiCheckbox>
          </UiCheckboxGroup>
        </div>
      </UiAsyncSection>
    </div>
  </UiDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { adminApi, type AdminCourseItem, type AdminTutorCourseAssignment } from '@/api/admin'
import { useConfirm } from '@/composables/useConfirm'
import UiAsyncSection from '@/components/ui/UiAsyncSection.vue'
import UiCheckbox from '@/components/ui/UiCheckbox.vue'
import UiCheckboxGroup from '@/components/ui/UiCheckboxGroup.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiIcon from '@/components/ui/UiIcon.vue'
import UiInput from '@/components/ui/UiInput.vue'
import UiTag from '@/components/ui/UiTag.vue'

const props = defineProps<{
  tutorId: number
  tutorName: string
}>()

const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{ saved: [] }>()

const loading = ref(false)
const submitting = ref(false)
const loadError = ref<Error | null>(null)
const keyword = ref('')

/** 全量课程（候选池；后端按 page_size=200 一次取，管理端课程规模远小于此） */
const courses = ref<AdminCourseItem[]>([])
/** 已分配认领行：course_id → 来源（self/admin）。未在表里 = 未分配 */
const assignments = ref<Map<number, AdminTutorCourseAssignment>>(new Map())
/** 当前勾选的课程 id（打开时初始化为「已分配的」） */
const selectedIds = ref<number[]>([])

const filteredCourses = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return courses.value
  return courses.value.filter(c => c.name.toLowerCase().includes(kw))
})

// 空态判据必须具名（守卫 #1101 只认脚本里声明过的标识符，模板内联的
// `filteredCourses.length === 0` 会被 scripts/check-async-section.mjs 判红）。
// 这里的「空」是**筛选后为空**（有课程但搜索无匹配），不是「后端没数据」——
// 后者由 loadError/首次加载覆盖，所以不复用 useAsyncPage 的 isEmpty。
const isEmptyCourses = computed(() => filteredCourses.value.length === 0)

function assignedSource(courseId: number): string | null {
  return assignments.value.get(courseId)?.source ?? null
}

function sourceTone(courseId: number): 'success' | 'info' {
  return assignedSource(courseId) === 'admin' ? 'success' : 'info'
}

function sourceLabel(courseId: number): string {
  return assignedSource(courseId) === 'admin' ? '管理员分配' : '他自己认领'
}

// 用 watch(visible) 而不是 el-dialog 的 @open：抽屉的数据加载是「打开就拉」，
// 与对话框的动画时序无关。挂 @open 会让「先点开、动画没跑完就保存」拿到空候选池，
// 也让测试必须模拟 EP 内部事件（我们不该测组件库的时序）。
// immediate：抽屉被父组件以「打开状态挂载」时（visible 初值即 true）也要拉一次。
watch(
  visible,
  opened => {
    if (opened) {
      keyword.value = ''
      void loadAll()
    }
  },
  { immediate: true }
)

async function loadAll() {
  loading.value = true
  loadError.value = null
  try {
    const [coursePage, assignmentList] = await Promise.all([
      adminApi.getCourses({ page: 1, page_size: 200, filter: 'all' }),
      adminApi.getTutorCourses(props.tutorId)
    ])
    courses.value = coursePage.courses ?? []
    assignments.value = new Map(
      (assignmentList.items ?? []).map(item => [item.course_id, item])
    )
    // 初始勾选 = 当前已分配（差集才是「这次改了什么」）
    selectedIds.value = [...(assignmentList.course_ids ?? [])]
  } catch (error) {
    loadError.value = error as Error
  } finally {
    loading.value = false
  }
}

async function handleConfirm() {
  const before = new Set(assignments.value.keys())
  const after = new Set(selectedIds.value)
  const toAdd = [...after].filter(id => !before.has(id))
  const toRemove = [...before].filter(id => !after.has(id))

  if (toAdd.length === 0 && toRemove.length === 0) {
    ElMessage.info('没有改动')
    visible.value = false
    return
  }

  // 移除是不可逆动作（断编辑权），单独确认一次；新增是安全动作，不打扰。
  // 两者都有时先问移除——用户取消就整体不动，避免「加了一半又不想删」。
  if (toRemove.length > 0) {
    try {
      await useConfirm().confirmDanger(
        `将移除 ${toRemove.length} 门课程。移除后该讲师不能再编辑本课程；` +
          '已写章节的归属与在审内容不受影响。',
        '确认移除课程'
      )
    } catch {
      return // 用户取消：抽屉保持打开，勾选状态不动
    }
  }

  submitting.value = true
  try {
    // 两条请求互不依赖，并发发；任一失败都算整体失败（不做半成功静默）
    await Promise.all([
      toAdd.length > 0 ? adminApi.assignTutorCourses(props.tutorId, toAdd) : Promise.resolve(),
      toRemove.length > 0 ? adminApi.removeTutorCourses(props.tutorId, toRemove) : Promise.resolve()
    ])
    ElMessage.success(
      toAdd.length > 0 || toRemove.length > 0
        ? `已更新：新增 ${toAdd.length} 门，移除 ${toRemove.length} 门`
        : '没有改动'
    )
    visible.value = false
    emit('saved')
  } catch (error) {
    console.error('分配课程失败:', error)
    /* 错误已由拦截器提示；抽屉保留，用户可重试 */
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.assign-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.assign-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.course-list {
  max-height: 380px;
  overflow-y: auto;
  padding-right: 4px;
}

.course-name {
  margin-right: 8px;
}
</style>
