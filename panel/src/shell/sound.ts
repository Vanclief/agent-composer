// The notification chime: two quick tones from the Web Audio API, no
// audio file. Browsers keep an AudioContext suspended until the page gets
// a user gesture, so a chime asked for before that waits for the first
// click or key press. The speaker toggle in the top bar mutes it; the
// choice is in localStorage under agc.panel.sound ("on" or "off").
import { useSyncExternalStore } from 'react'

export const SOUND_KEY = 'agc.panel.sound'

const listeners = new Set<() => void>()

function readSoundOn() {
  try {
    return localStorage.getItem(SOUND_KEY) !== 'off'
  } catch {
    return true
  }
}

let soundOn = readSoundOn()

export function setSoundOn(on: boolean) {
  soundOn = on
  try {
    localStorage.setItem(SOUND_KEY, on ? 'on' : 'off')
  } catch {
    // Private mode: the choice lasts for this page only.
  }
  for (const listener of listeners) {
    listener()
  }
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function useSoundOn() {
  return useSyncExternalStore(subscribe, () => soundOn)
}

let context: AudioContext | null = null
let queued = false

// Two sine tones, A5 then E6, 110ms each with a short fade: 240ms in all.
function tones(ctx: AudioContext) {
  const start = ctx.currentTime + 0.01
  const notes: [number, number][] = [
    [880, 0],
    [1318.5, 0.13],
  ]
  for (const [frequency, offset] of notes) {
    const oscillator = ctx.createOscillator()
    const gain = ctx.createGain()
    oscillator.type = 'sine'
    oscillator.frequency.value = frequency
    const at = start + offset
    gain.gain.setValueAtTime(0, at)
    gain.gain.linearRampToValueAtTime(0.12, at + 0.01)
    gain.gain.exponentialRampToValueAtTime(0.001, at + 0.11)
    oscillator.connect(gain).connect(ctx.destination)
    oscillator.start(at)
    oscillator.stop(at + 0.12)
  }
}

function onGesture() {
  window.removeEventListener('pointerdown', onGesture, true)
  window.removeEventListener('keydown', onGesture, true)
  if (!context) {
    return
  }
  const ctx = context
  void ctx.resume().then(() => {
    if (queued && soundOn) {
      tones(ctx)
    }
    queued = false
  })
}

// Plays the chime now, or after the first user gesture when the browser
// has not allowed audio yet. Several chimes queued before that play once.
export function playChime() {
  if (!soundOn || typeof AudioContext === 'undefined') {
    return
  }
  try {
    context ??= new AudioContext()
  } catch {
    return
  }
  if (context.state === 'running') {
    tones(context)
    return
  }
  if (!queued) {
    queued = true
    window.addEventListener('pointerdown', onGesture, true)
    window.addEventListener('keydown', onGesture, true)
  }
}
