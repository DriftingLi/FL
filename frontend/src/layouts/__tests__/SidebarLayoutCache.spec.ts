// 管理端外壳：页面切换与缓存（#1620 起的外壳，#1631 的回归锁）。
//
// 锁的行为是**「切走之后，新页面必须渲染出来」**。外壳内层的 <transition> 曾是 mode="out-in"，
// 而 out-in 的放行条件是「离场动画完成」的回调（BaseTransition 的 leavingHooks.afterLeave）：
// 根节点不是单个元素的页面（如 <UiAsyncSection> 的四分支模板 = Fragment 根）在渲染器里走的是
// Fragment 分支，那个回调永远不会被调用 ⇒ state.isLeaving 永久为真、内容列只剩一个空占位符。
// 线上形态：标签栏与侧栏都在，内容区从「离开该页那一刻」起永久空白（刷新才恢复）。
//
// 因此本用例刻意让**第一个页面是多根组件**：这正是管理端「管理员管理」页（根 = UiAsyncSection）
// 的形状，也是唯一能触发该状态的形状。
import { beforeEach, describe, expect, it } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, nextTick, onMounted } from 'vue'
import { createPinia } from 'pinia'
import { RouterView, createMemoryHistory, createRouter, type Router } from 'vue-router'

import { epLite } from '@/test/element-lite'
import SidebarLayout from '../SidebarLayout.vue'
import AdminTabBar from '@/components/layout/AdminTabBar.vue'

const mounts = { fragment: 0, plain: 0 }

/** 多根（Fragment）页面：与 <UiAsyncSection> 那类多分支模板同形。 */
const FragmentPage = defineComponent({
  name: 'PageFragment',
  setup() {
    onMounted(() => {
      mounts.fragment += 1
    })
    return () => [h('div', { class: 'frag-a' }, 'F-A'), h('div', { class: 'frag-b' }, 'F-B')]
  }
})

/** 单元素根页面（绝大多数管理页的形状）。 */
const PlainPage = defineComponent({
  name: 'PagePlain',
  setup() {
    onMounted(() => {
      mounts.plain += 1
    })
    return () => h('div', { class: 'plain' }, 'PLAIN')
  }
})

/** 外壳宿主：路由记录里的布局（与 AdminLayout 同形：SidebarLayout + 标签栏挂 content-header）。 */
const LayoutHost = defineComponent({
  name: 'LayoutHost',
  setup() {
    return () =>
      h(
        SidebarLayout,
        { menuItems: [], keepAliveNames: ['PageFragment', 'PagePlain'] },
        { 'content-header': () => h(AdminTabBar) }
      )
  }
})

function makeRouter(): Router {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      {
        path: '/admin',
        component: LayoutHost,
        children: [
          { path: 'frag', name: 'PageFragment', component: FragmentPage },
          { path: 'plain', name: 'PagePlain', component: PlainPage }
        ]
      }
    ]
  })
}

/** 等到过渡与 post-render effect 都跑完（进入/离场各需要至少两帧 rAF）。 */
async function settle(): Promise<void> {
  await flushPromises()
  await nextTick()
  await new Promise(resolve => setTimeout(resolve, 80))
  await nextTick()
}

async function setup(path: string): Promise<{ w: VueWrapper; router: Router }> {
  const router = makeRouter()
  await router.push(path)
  await router.isReady()
  const Host = defineComponent({ name: 'Host', setup: () => () => h(RouterView) })
  const w = mount(Host, {
    attachTo: document.body,
    global: {
      plugins: [router, createPinia(), epLite()],
      stubs: {
        // **必须显式关掉**：@vue/test-utils 默认把 <transition> 换成 stub，
        // 而本用例要锁的正是真实过渡机制里的卡死（stub 掉就等于把被测行为删掉，用例会假绿）。
        transition: false,
        // 顶栏与侧栏是外部组件（各自有 store/接口依赖），本用例的 seam 是内容列的路由出口
        AppTopBar: true,
        AppSidebar: true
      }
    }
  })
  await settle()
  return { w, router }
}

beforeEach(() => {
  mounts.fragment = 0
  mounts.plain = 0
})

describe('SidebarLayout 内容列：页面切换与缓存', () => {
  it('离开 Fragment 根页面后新页面仍要渲染（out-in 卡死的回归锁）', async () => {
    const { w, router } = await setup('/admin/frag')
    expect(w.find('.frag-a').exists()).toBe(true)

    await router.push('/admin/plain')
    await settle()
    expect(w.find('.plain').exists()).toBe(true)

    await router.push('/admin/frag')
    await settle()
    expect(w.find('.frag-a').exists()).toBe(true)
    w.unmount()
  })

  it('缓存名单内的页面往返后不重新挂载（DOM 与实例都被复用）', async () => {
    const { w, router } = await setup('/admin/plain')
    const firstNode = w.find('.plain').element
    await router.push('/admin/frag')
    await settle()
    await router.push('/admin/plain')
    await settle()
    expect(w.find('.plain').exists()).toBe(true)
    // 同一个 DOM 节点 = 走的是 keep-alive 的缓存实例，而不是重新挂载出来的新节点
    expect(w.find('.plain').element).toBe(firstNode)
    expect(mounts.plain).toBe(1)
    w.unmount()
  })
})
