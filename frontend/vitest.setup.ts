// Vitest setup：Node 25+ 内置 localStorage 遮蔽环境实现的兜底（vitest-dev/vitest#8757，Node 不修）。
// Node 25+ 把 globalThis.localStorage 设为需 --localstorage-file 的 accessor（未提供时求值为
// 不可用的对象），vitest 的 populateGlobal 因 key 已存在而跳过 happy-dom 实现拷贝 → 测试里
// localStorage.clear/getItem 全炸。本 shim 仅在环境 localStorage 不可用时替换为等价内存实现；
// Node ≤24（CI）下环境实现正常，isBroken 为 false，此处零影响。
const g = globalThis as any

function storageUsable(ls: any): boolean {
  return (
    typeof ls === 'object' &&
    typeof ls.getItem === 'function' &&
    typeof ls.setItem === 'function' &&
    typeof ls.removeItem === 'function' &&
    typeof ls.clear === 'function'
  )
}

if (!storageUsable(g.localStorage)) {
  const store = new Map<string, string>()
  const memoryStorage: Storage = {
    get length() {
      return store.size
    },
    clear: () => store.clear(),
    getItem: (key: string) => (store.has(key) ? (store.get(key) as string) : null),
    key: (index: number) => Array.from(store.keys())[index] ?? null,
    removeItem: (key: string) => {
      store.delete(key)
    },
    setItem: (key: string, value: string) => {
      store.set(String(key), String(value))
    }
  }
  g.localStorage = memoryStorage
  if (g.window && !storageUsable(g.window.localStorage)) {
    g.window.localStorage = memoryStorage
  }
}