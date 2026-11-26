import type { UiSelectOption } from '@/components/ui/UiSelect.vue'

export const LEGACY_CATEGORY_LABEL = '未分类/分类失效'

export function trainingCategoryOptions(
  categories: ReadonlyArray<{ id: number; name: string; status?: number | null }>,
  allLabel: string
): UiSelectOption[] {
  return [
    { label: allLabel, value: 'all' },
    ...categories
      .filter(category => (category.status ?? 1) !== 0)
      .map(category => ({ label: category.name, value: category.id })),
    { label: LEGACY_CATEGORY_LABEL, value: 'legacy' }
  ]
}

export function trainingCategoryName(
  category?: { name: string; status?: number | null } | null
): string {
  if (!category) return LEGACY_CATEGORY_LABEL
  return category.status === 0 ? `${category.name}（已停用）` : category.name
}
