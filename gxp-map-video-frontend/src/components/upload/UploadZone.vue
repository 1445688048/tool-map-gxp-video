<template>
  <div class="upload-zone" :class="{ dragging: isDragging, disabled: uploading }" @click="onClick" @dragover="onDragOver" @dragleave="isDragging = false" @drop="onDrop">
    <input ref="input" type="file" accept=".gpx" hidden @change="onSelect" />
    <div class="upload-content">
      <svg viewBox="0 0 24 24" width="48" height="48" fill="none" stroke="#e94560" stroke-width="1.5">
        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
        <polyline points="17 8 12 3 7 8"/>
        <line x1="12" y1="3" x2="12" y2="15"/>
      </svg>
      <p v-if="!uploading">{{ isDragging ? '松开上传' : '拖放 GPX 文件到这里，或点击选择' }}</p>
      <p v-else>解析中...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouteStore } from '@/stores/route'

const emit = defineEmits<{ loaded: [route: any] }>()
const input = ref<HTMLInputElement | null>(null)
const isDragging = ref(false)
const uploading = ref(false)
const routeStore = useRouteStore()

function onClick() {
  input.value?.click()
}

async function onSelect(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0]
  if (file) await handleFile(file)
}

function onDragOver(e: DragEvent) {
  e.preventDefault()
  isDragging.value = true
}

async function onDrop(e: DragEvent) {
  e.preventDefault()
  isDragging.value = false
  const file = e.dataTransfer?.files[0]
  if (file && file.name.endsWith('.gpx')) {
    await handleFile(file)
  }
}

async function handleFile(file: File) {
  uploading.value = true
  try {
    const route = await routeStore.uploadGPX(file)
    emit('loaded', route)
  } catch (err) {
    alert('上传失败: ' + (err instanceof Error ? err.message : String(err)))
  } finally {
    uploading.value = false
  }
}
</script>

<style scoped>
.upload-zone { margin: 12px 20px; padding: 24px; border: 2px dashed #0f3460; border-radius: 8px; text-align: center; cursor: pointer; transition: all 0.2s; }
.upload-zone.dragging { border-color: #e94560; background: rgba(233, 69, 96, 0.1); }
.upload-zone.disabled { opacity: 0.5; cursor: not-allowed; }
.upload-content p { color: #8899aa; margin-top: 8px; font-size: 14px; }
</style>
