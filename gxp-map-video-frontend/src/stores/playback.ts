import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { TrackPoint } from '@/types/gpx'

export const usePlaybackStore = defineStore('playback', () => {
  const isPlaying = ref(false)
  const progress = ref(0)
  const speed = ref(1)
  const cameraAltitude = ref(100)
  const cameraPitch = ref(60)
  const currentPoint = ref<TrackPoint | null>(null)
  const phase = ref<'idle' | 'intro' | 'playing' | 'outro' | 'ended'>('idle')

  function play() { isPlaying.value = true }
  function pause() { isPlaying.value = false }
  function seekTo(p: number) { progress.value = p }
  function setSpeed(s: number) { speed.value = s }
  function setCameraAltitude(a: number) { cameraAltitude.value = a }
  function setCameraPitch(p: number) { cameraPitch.value = p }
  function setCurrentPoint(pt: TrackPoint | null) { currentPoint.value = pt }

  return {
    isPlaying, progress, speed, cameraAltitude, cameraPitch,
    currentPoint, phase, play, pause, seekTo, setSpeed,
    setCameraAltitude, setCameraPitch, setCurrentPoint,
  }
})
