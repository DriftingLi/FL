export function getStorage<T = unknown>(key: string): T | null {
  try {
    const value = localStorage.getItem(key)
    return value ? JSON.parse(value) as T : null
  } catch {
    return localStorage.getItem(key) as unknown as T | null
  }
}

export function setStorage<T = unknown>(key: string, value: T): void {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    localStorage.setItem(key, String(value))
  }
}

export function removeStorage(key: string): void {
  localStorage.removeItem(key)
}

export function clearStorage(): void {
  localStorage.clear()
}
