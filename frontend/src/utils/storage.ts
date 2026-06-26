export function getStorage(key) {
  try {
    const value = localStorage.getItem(key)
    return value ? JSON.parse(value) : null
  } catch (e) {
    return localStorage.getItem(key)
  }
}

export function setStorage(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch (e) {
    localStorage.setItem(key, value)
  }
}

export function removeStorage(key) {
  localStorage.removeItem(key)
}

export function clearStorage() {
  localStorage.clear()
}
