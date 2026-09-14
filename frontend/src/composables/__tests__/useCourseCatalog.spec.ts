// useCourseCatalog：课程目录筛选 module 的接口级测试（计数语义的唯一事实源）。
// seam：composable 接口——data-source adapter 用内存 fixture，不触达 API 层。
import { describe, it, expect } from 'vitest'
import { useCourseCatalog, treeCatalogAdapter } from '@/composables/useCourseCatalog'
import type { CatalogDirectionNode, CatalogLevelNode, LevelDict } from '@/api/training'
import type { CourseDTO } from '@/api/course'

// 生成 DTO 的最小测试夹具：只填测试关心的字段，其余取 DTO 零值
// （注解成为唯一事实源后，手写时代「只给两个字段」的 fixture 不再合法）。
function courseOf(courseId: number, name: string, over: Partial<CourseDTO> = {}): CourseDTO {
  return {
    certificate_name: '',
    certificate_template_id: null,
    course_id: courseId,
    cover_image: '',
    created_at: '',
    credential_id: null,
    description: '',
    duration: 0,
    is_featured: false,
    is_hot: false,
    level_id: null,
    name,
    practice_hours: 0,
    sort_order: 0,
    specialty_id: null,
    status: 1,
    theory_hours: 0,
    ...over
  }
}
function levelDictOf(levelId: number, name: string, over: Partial<LevelDict> = {}): LevelDict {
  return { code: '', created_at: '', description: '', level_id: levelId, name, sort_order: 0, status: 1, ...over }
}
function levelNodeOf(levelId: number, name: string, sortOrder: number, courses: CourseDTO[]): CatalogLevelNode {
  return { ...levelDictOf(levelId, name), sort_order: sortOrder, courses }
}
function specialtyOf(specialtyId: number, name: string, levels: CatalogLevelNode[]): CatalogDirectionNode {
  return { code: '', created_at: '', description: '', levels, name, sort_order: 0, specialty_id: specialtyId, status: 1 }
}

const tree: CatalogDirectionNode[] = [
  specialtyOf(2, '维修', [
    levelNodeOf(1, '入门', 1, [courseOf(1, 'A')]),
    levelNodeOf(2, '进阶', 2, [courseOf(2, 'B'), courseOf(3, 'C')])
  ]),
  specialtyOf(3, '安全', [levelNodeOf(1, '入门', 1, [courseOf(4, 'D')])])
]

function mountTree() {
  const catalog = useCourseCatalog({
    adapter: treeCatalogAdapter(async () => ({ specialties: tree }))
  })
  return catalog
}

function mountFlat() {
  const catalog = useCourseCatalog({
    adapter: {
      async load() {
        return {
          directions: [specialtyOf(2, '维修', [])],
          levels: [levelDictOf(1, '入门')],
          items: [
            { specialty_id: 2, level_id: 1, count: 1 },
            { specialty_id: 2, level_id: 1, count: 1 },
            { specialty_id: null, level_id: null, count: 1 }
          ]
        }
      }
    },
    bidirectional: false
  })
  return catalog
}

describe('treeCatalogAdapter（树 → 计数组）', () => {
  it('目录树展开为计数组，等级按 sort_order 去重合并', async () => {
    const catalog = mountTree()
    await catalog.fetchCatalog()

    expect(catalog.directions.value.map(d => d.specialty_id)).toEqual([2, 3])
    expect(catalog.levels.value.map(l => l.name)).toEqual(['入门', '进阶'])
    expect(catalog.totalAll.value).toBe(4)
    expect(catalog.countOfDirection(2)).toBe(3)
    expect(catalog.countOfDirection(3)).toBe(1)
  })

  it('等级名称与方向名称查找收敛在 module 内', async () => {
    const catalog = mountTree()
    await catalog.fetchCatalog()

    expect(catalog.levelNameOf(2)).toBe('进阶')
    expect(catalog.specialtyNameOf(3)).toBe('安全')
    expect(catalog.levelNameOf(null)).toBe('')
  })
})

describe('双向计数联动（学员/导师树模式）', () => {
  it('选中等级后方向卡计数与「全部课程」收敛为该等级课程数', async () => {
    const catalog = mountTree()
    await catalog.fetchCatalog()

    catalog.selectLevel(1)
    expect(catalog.totalAll.value).toBe(2)
    expect(catalog.countOfDirection(2)).toBe(1)
    expect(catalog.countOfDirection(3)).toBe(1)
    // 方向范围未变，「全部等级」不受等级筛选影响
    expect(catalog.scopedTotal.value).toBe(4)
  })

  it('选中方向后等级卡计数与「全部等级」收敛为该方向课程数', async () => {
    const catalog = mountTree()
    await catalog.fetchCatalog()

    catalog.selectDirection(2)
    expect(catalog.scopedTotal.value).toBe(3)
    expect(catalog.countOfLevel(1)).toBe(1)
    expect(catalog.countOfLevel(2)).toBe(2)
    expect(catalog.totalAll.value).toBe(3)
  })

  it('方向 + 等级同时选中时计数取交集', async () => {
    const catalog = mountTree()
    await catalog.fetchCatalog()

    catalog.selectDirection(2)
    catalog.selectLevel(2)
    expect(catalog.totalAll.value).toBe(2)
    expect(catalog.scopedTotal.value).toBe(3)
    expect(catalog.countOfLevel(2)).toBe(2)
    expect(catalog.countOfDirection(2)).toBe(2)
  })

  it('选中变化触发 onSelect 回调（页面据此重置页码并重新拉取）', async () => {
    let fired = 0
    const catalog = useCourseCatalog({
      adapter: treeCatalogAdapter(async () => ({ specialties: tree })),
      onSelect: () => {
        fired++
      }
    })
    await catalog.fetchCatalog()

    catalog.selectDirection(2)
    catalog.selectLevel(null)
    expect(fired).toBe(2)
  })
})

describe('管理端扁平模式（bidirectional=false）', () => {
  it('方向卡计数不随等级筛选变化，「全部课程」恒定', async () => {
    const catalog = mountFlat()
    await catalog.fetchCatalog()

    expect(catalog.totalAll.value).toBe(3)
    catalog.selectLevel(1)
    expect(catalog.totalAll.value).toBe(3)
    expect(catalog.countOfDirection(2)).toBe(2)
  })

  it('未挂载 facet：count 与 -1 选择哨兵', async () => {
    const catalog = mountFlat()
    await catalog.fetchCatalog()

    expect(catalog.unmountedCount.value).toBe(1)
    catalog.selectDirection(-1)
    expect(catalog.scopedTotal.value).toBe(1)
    expect(catalog.countOfLevel(1)).toBe(0)
  })
})
