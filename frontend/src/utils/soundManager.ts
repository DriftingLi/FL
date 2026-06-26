let audioContext = null
let soundEnabled = true
let volume = 0.5

const SOUND_MAP = {
  click: { frequency: 800, duration: 0.08, type: 'sine' },
  success: { frequency: 523, duration: 0.3, type: 'sine', sweep: 784 },
  fail: { frequency: 300, duration: 0.3, type: 'sawtooth', sweep: 200 },
  alarm: { frequency: 600, duration: 0.5, type: 'square', sweep: 400 },
  detach: { frequency: 440, duration: 0.15, type: 'triangle' },
  attach: { frequency: 660, duration: 0.15, type: 'triangle' },
  tick: { frequency: 1000, duration: 0.03, type: 'sine' }
}

function getAudioContext() {
  if (!audioContext) {
    try {
      audioContext = new (window.AudioContext || window.webkitAudioContext)()
    } catch (e) {
      return null
    }
  }
  return audioContext
}

function playSound(soundName) {
  if (!soundEnabled) return

  const ctx = getAudioContext()
  if (!ctx) return

  const config = SOUND_MAP[soundName]
  if (!config) return

  try {
    if (ctx.state === 'suspended') {
      ctx.resume()
    }

    const oscillator = ctx.createOscillator()
    const gainNode = ctx.createGain()

    oscillator.type = config.type || 'sine'
    oscillator.frequency.setValueAtTime(config.frequency, ctx.currentTime)

    if (config.sweep) {
      oscillator.frequency.linearRampToValueAtTime(config.sweep, ctx.currentTime + config.duration)
    }

    gainNode.gain.setValueAtTime(volume * 0.3, ctx.currentTime)
    gainNode.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + config.duration)

    oscillator.connect(gainNode)
    gainNode.connect(ctx.destination)

    oscillator.start(ctx.currentTime)
    oscillator.stop(ctx.currentTime + config.duration)
  } catch (e) {
    // silently fail
  }
}

export function playClick() { playSound('click') }
export function playSuccess() { playSound('success') }
export function playFail() { playSound('fail') }
export function playAlarm() { playSound('alarm') }
export function playDetach() { playSound('detach') }
export function playAttach() { playSound('attach') }
export function playTick() { playSound('tick') }

export function setVolume(v) {
  volume = Math.max(0, Math.min(1, v))
}

export function getVolume() {
  return volume
}

export function setSoundEnabled(enabled) {
  soundEnabled = enabled
}

export function isSoundEnabled() {
  return soundEnabled
}

export function initAudio() {
  getAudioContext()
}
