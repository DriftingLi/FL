<script setup lang="ts">
import { computed } from 'vue'
import type { Eligibility, Reason } from '@/api/generated/course'
import UiTag from '@/components/ui/UiTag.vue'
import { describePublicEligibility, describePublicReason } from '@/utils/publicEligibility'

const props = withDefaults(defineProps<{
  eligibility?: Eligibility
  showReasons?: boolean
  reasonLimit?: number
}>(), {
  showReasons: true
})

const summary = computed(() => describePublicEligibility(props.eligibility))
const reasons = computed(() => props.eligibility?.reasons.slice(0, props.reasonLimit).map((reason: Reason) => ({
  code: reason,
  ...describePublicReason(reason)
})) ?? [])
</script>

<template>
  <div v-if="eligibility" class="flex flex-wrap items-center gap-x-2 gap-y-1">
    <UiTag :tone="summary.tone" size="small" effect="plain">{{ summary.label }}</UiTag>
    <template v-if="showReasons">
      <span v-for="(reason, index) in reasons" :key="reason.code" class="text-xs text-ink-3">
        {{ reason.label }}<span v-if="index < reasons.length - 1">；</span>
      </span>
    </template>
  </div>
  <span v-else class="text-xs text-ink-3">—</span>
</template>
