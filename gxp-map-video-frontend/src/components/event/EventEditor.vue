<template>
  <div class="event-editor">
    <div class="header">
      <h3>故事事件</h3>
      <button @click="createNew" class="btn-add">+ 添加事件</button>
    </div>
    <div v-if="events.length === 0" class="empty">
      暂无事件，点击上方按钮添加
    </div>
    <div v-for="ev in events" :key="ev.id" class="event-item" :class="ev.event_type.toLowerCase()">
      <div class="ev-header" @click="toggleEdit(ev)">
        <span class="ev-icon">{{ eventIcon(ev.event_type) }}</span>
        <span class="ev-title">{{ ev.title || ev.event_type }}</span>
        <span class="ev-pos">{{ ev.position.toFixed(2) }} km</span>
        <span class="ev-script-trunc">{{ truncate(ev.script, 20) }}</span>
      </div>
      <div v-if="editingId === ev.id" class="ev-form">
        <input v-model="editForm.title" class="field" placeholder="标题" />
        <select v-model="editForm.event_type" class="field">
          <option value="COMMENTARY">解说</option>
          <option value="WARNING">警告</option>
          <option value="POI">兴趣点</option>
          <option value="VIEWPOINT">观景点</option>
          <option value="JUNCTION">岔路口</option>
          <option value="REST">休息点</option>
        </select>
        <textarea v-model="editForm.script" class="field commentary" placeholder="解说文字（规则模板自动生成，后期接AI）" rows="3"></textarea>
        <div class="ev-row">
          <label>前停留(s) <input v-model.number="editForm.hold_before" type="number" min="0" max="30" class="num-input" /></label>
          <label>后停留(s) <input v-model.number="editForm.hold_after" type="number" min="0" max="30" class="num-input" /></label>
        </div>
        <div class="editor-actions">
          <button @click="saveEvent(ev)" class="btn-save">保存</button>
          <button @click="deleteEvent(ev.id!)" class="btn-delete">删除</button>
          <button @click="cancelEdit" class="btn-cancel">取消</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { api } from '@/api/client'
import type { StoryEvent } from '@/types/gpx'

const props = defineProps<{ routeId: number | null }>()
const emit = defineEmits<{ eventChanged: [ev: StoryEvent] }>()

const events = ref<StoryEvent[]>([])
const editingId = ref<number | null>(null)
const editForm = ref({ title: '', event_type: 'COMMENTARY', script: '', hold_before: 0, hold_after: 0 })

watch(() => props.routeId, (id) => {
  if (id) loadEvents(id)
  else events.value = []
})

onMounted(() => {
  if (props.routeId) loadEvents(props.routeId)
})

function showToast(msg: string, type: 'info' | 'error' | 'success' = 'info') {
  window.dispatchEvent(new CustomEvent('gxp-toast', { detail: { message: msg, type } }))
}

async function loadEvents(routeId: number) {
  events.value = await api.getEvents(routeId) as StoryEvent[]
}

function toggleEdit(ev: StoryEvent) {
  if (editingId.value === ev.id) {
    cancelEdit()
  } else {
    editingId.value = ev.id ?? null
    editForm.value = {
      title: ev.title || '',
      event_type: ev.event_type,
      script: ev.script || '',
      hold_before: ev.hold_before,
      hold_after: ev.hold_after,
    }
  }
}

function cancelEdit() {
  editingId.value = null
}

function createNew() {
  if (!props.routeId) return
  editingId.value = -1 as any
  editForm.value = { title: '', event_type: 'COMMENTARY', script: '', hold_before: 0, hold_after: 0 }
}

async function saveEvent(ev?: StoryEvent) {
  const form = editForm.value
  if (!props.routeId) return

  if (editingId.value && editingId.value > 0 && ev) {
    // Update existing
    try {
      const updated = await api.updateEvent(editingId.value, form) as StoryEvent
      const idx = events.value.findIndex(e => e.id === ev.id)
      if (idx >= 0) events.value[idx] = updated
      emit('eventChanged', updated)
      cancelEdit()
    } catch (e) {
      showToast('保存失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
    }
  } else {
    // Create new at current playback position or midpoint
    const pos = events.value.length > 0
      ? (events.value[events.value.length - 1]?.position ?? 0) + 1
      : 0.5
    try {
      const created = await api.createEvent(props.routeId, { ...form, position: pos }) as StoryEvent
      events.value.push(created)
      emit('eventChanged', created)
      cancelEdit()
    } catch (e) {
      showToast('创建失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
    }
  }
}

async function deleteEvent(id: number) {
  if (!confirm('确定删除这个事件？')) return
  try {
    await api.deleteEvent(id)
    events.value = events.value.filter(e => e.id !== id)
    cancelEdit()
  } catch (e) {
    showToast('删除失败', 'error')
  }
}

function eventIcon(type: string): string {
  const icons: Record<string, string> = {
    COMMENTARY: '🎙️', WARNING: '⚠️', POI: '📍',
    VIEWPOINT: '🏔️', JUNCTION: '🔀', REST: '🧘',
  }
  return icons[type] || '📌'
}

function truncate(s: string, n: number): string {
  return s.length > n ? s.slice(0, n) + '...' : s
}
</script>

<style scoped>
.event-editor { background: #16213e; border-right: 1px solid #0f3460; padding: 12px; overflow-y: auto; width: 280px; flex-shrink: 0; }
.header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; }
.header h3 { font-size: 13px; color: #e94560; }
.btn-add { background: #4ecca3; color: #16213e; border: none; padding: 4px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; font-weight: 600; }
.empty { color: #667; font-size: 12px; text-align: center; padding: 20px 0; }
.event-item { border-left: 3px solid #8899aa; background: #1a1a2e; border-radius: 4px; padding: 8px 10px; margin-bottom: 4px; cursor: pointer; }
.event-item.commentary { border-left-color: #4ecca3; }
.event-item.warning { border-left-color: #e94560; }
.event-item.poi { border-left-color: #533483; }
.event-item.viewpoint { border-left-color: #f0a500; }
.event-item.junction { border-left-color: #00b4d8; }
.event-item.rest { border-left-color: #98d8aa; }
.ev-header { display: flex; align-items: center; gap: 6px; font-size: 11px; }
.ev-icon { font-size: 13px; }
.ev-title { font-weight: 600; flex: 1; }
.ev-pos { color: #aaa; }
.ev-script-trunc { color: #667; font-size: 10px; max-width: 100px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ev-form { margin-top: 8px; display: flex; flex-direction: column; gap: 6px; }
.field { background: #0f3460; color: #eee; border: 1px solid #2a3a5e; border-radius: 4px; padding: 5px 8px; font-size: 11px; }
.field.commentary { resize: vertical; min-height: 60px; }
select.field { cursor: pointer; }
.ev-row { display: flex; gap: 12px; font-size: 11px; color: #aaa; }
.ev-row label { display: flex; align-items: center; gap: 4px; }
.num-input { width: 50px; background: #0f3460; color: #eee; border: 1px solid #2a3a5e; border-radius: 3px; padding: 2px 4px; font-size: 11px; }
.editor-actions { display: flex; gap: 6px; }
.btn-save { background: #4ecca3; color: #16213e; border: none; padding: 3px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; font-weight: 600; }
.btn-delete { background: transparent; color: #e94560; border: 1px solid #e94560; padding: 3px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; }
.btn-cancel { background: transparent; color: #8899aa; border: 1px solid #0f3460; padding: 3px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; }
</style>
