<template>
  <div class="segment-list">
    <div class="header">
      <h3>分段分析</h3>
      <div class="actions">
        <select v-model="activePreset" class="preset-select" @change="onPresetChange">
          <option value="trail_running">越野跑</option>
          <option value="hiking">徒步登山</option>
          <option value="mtb">山地骑行</option>
        </select>
        <button @click="generateCommentary" :disabled="generating || segments.length === 0" class="btn-commentary">
          {{ generating ? '生成中...' : '生成解说' }}
        </button>
        <button @click="analyze" :disabled="analyzing" class="btn-analyze">
          {{ analyzing ? '分析中...' : '重新分析' }}
        </button>
      </div>
    </div>
    <div v-if="segments.length === 0 && !loading" class="empty">
      选择活动类型后点击"重新分析"生成分段
    </div>
    <div v-if="loading" class="loading">加载中...</div>
    <div v-else class="segments">
      <div
        v-for="seg in segments"
        :key="seg.id"
        class="segment-item"
        :class="[seg.type.toLowerCase(), { active: seg.id === props.activeSegmentId }]"
        :style="{ borderLeftColor: segmentColor(seg.type) }"
      >
        <div class="seg-header" @click="toggleEdit(seg)">
          <span class="seg-type">{{ typeLabel(seg.type) }}</span>
          <span class="seg-dist">{{ (seg.distance / 1000).toFixed(2) }} km</span>
          <span class="seg-ele">{{ seg.elevation_gain.toFixed(0) }}m↑ {{ seg.elevation_loss.toFixed(0) }}m↓</span>
          <span class="seg-slope">{{ seg.average_slope > 0 ? '+' : '' }}{{ seg.average_slope.toFixed(1) }}%</span>
          <span class="seg-range">{{ (seg.start_distance / 1000).toFixed(1) }}-{{ (seg.end_distance / 1000).toFixed(1) }}km</span>
          <span class="edit-icon">{{ editingSeg?.id === seg.id ? '✕' : '✏️' }}</span>
        </div>
        <div v-if="editingSeg?.id === seg.id" class="seg-editor">
          <textarea
            v-model="editingCommentary"
            class="commentary-input"
            placeholder="添加解说文字（可留空）..."
            rows="2"
          ></textarea>
          <div class="editor-actions">
            <button @click="saveSegment(seg)" class="btn-save">保存</button>
            <button @click="cancelEdit" class="btn-cancel">取消</button>
            <button @click="mergeSegment(seg)" class="btn-merge" title="与下一段合并">合并下一段</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { api } from '@/api/client'
import type { RouteSegment } from '@/types/gpx'

const props = defineProps<{ routeId: number | null; activeSegmentId?: number | null }>()

const segments = ref<RouteSegment[]>([])
const loading = ref(false)
const analyzing = ref(false)
const generating = ref(false)
const activePreset = ref('trail_running')
const editingSeg = ref<RouteSegment | null>(null)
const editingCommentary = ref('')

const emit = defineEmits<{ segmentUpdated: [seg: RouteSegment]; segmentsChanged: [] }>()

watch(() => props.routeId, (id) => {
  if (id) loadSegments(id)
  else segments.value = []
})

onMounted(() => {
  if (props.routeId) loadSegments(props.routeId)
})

function showToast(msg: string, type: 'info' | 'error' | 'success' = 'info') {
  window.dispatchEvent(new CustomEvent('gxp-toast', { detail: { message: msg, type } }))
}

async function loadSegments(routeId: number) {
  loading.value = true
  try {
    segments.value = await api.getSegments(routeId) as RouteSegment[]
  } catch (e) {
    console.error('Failed to load segments:', e)
  } finally {
    loading.value = false
  }
}

async function analyze() {
  if (!props.routeId) return
  analyzing.value = true
  try {
    const result = await api.analyzeRoute(props.routeId, activePreset.value)
    segments.value = result as RouteSegment[]
    emit('segmentsChanged')
    showToast(`分析完成，共 ${segments.value.length} 个分段`, 'success')
  } catch (e) {
    showToast('分析失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
  } finally {
    analyzing.value = false
  }
}

function onPresetChange() {
  if (props.routeId) analyze()
}

function toggleEdit(seg: RouteSegment) {
  if (editingSeg.value?.id === seg.id) {
    cancelEdit()
  } else {
    editingSeg.value = seg
    editingCommentary.value = seg.commentary || ''
  }
}

function cancelEdit() {
  editingSeg.value = null
  editingCommentary.value = ''
}

async function saveSegment(seg: RouteSegment) {
  if (!seg.id) return
  try {
    const updated = await api.updateSegment(seg.id, { commentary: editingCommentary.value }) as RouteSegment
    const idx = segments.value.findIndex(s => s.id === seg.id)
    if (idx >= 0) segments.value[idx] = updated
    editingSeg.value = null
    editingCommentary.value = ''
    emit('segmentUpdated', updated)
  } catch (e) {
    showToast('保存失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
  }
}

async function mergeSegment(seg: RouteSegment) {
  if (!seg.id) return
  try {
    const updated = await api.mergeSegment(seg.id) as RouteSegment
    segments.value = await api.getSegments(props.routeId!) as RouteSegment[]
    cancelEdit()
    emit('segmentUpdated', updated)
    emit('segmentsChanged')
    showToast('已合并，分段数据已更新', 'success')
  } catch (e) {
    showToast('合并失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
  }
}

// 为没有解说的分段生成模板文案并直接保存到后端
async function generateCommentary() {
  if (!props.routeId) return
  const empty = segments.value.filter(s => !s.commentary)
  if (empty.length === 0) {
    showToast('所有分段都已有解说', 'info')
    return
  }
  generating.value = true
  try {
    for (const seg of empty) {
      const updated = await api.updateSegment(seg.id, { commentary: generateTemplateCommentary(seg) }) as RouteSegment
      const idx = segments.value.findIndex(s => s.id === updated.id)
      if (idx >= 0) segments.value[idx] = updated
      emit('segmentUpdated', updated)
    }
    showToast(`已为 ${empty.length} 个分段生成并保存解说`, 'success')
  } catch (e) {
    showToast('生成解说失败: ' + (e instanceof Error ? e.message : String(e)), 'error')
  } finally {
    generating.value = false
  }
}

function generateTemplateCommentary(seg: RouteSegment): string {
  const distKm = (seg.distance / 1000).toFixed(2)
  const gain = seg.elevation_gain.toFixed(0)
  const loss = seg.elevation_loss.toFixed(0)
  const avgSlope = seg.average_slope.toFixed(1)

  const templates: Record<string, string> = {
    FLAT: `平缓路段，长度${distKm}公里，${gain}米爬升${loss}米下降，平均坡度${avgSlope}%，地形相对稳定。`,
    CLIMB: `进入爬升段，长度${distKm}公里，累计爬升${gain}米，平均坡度+${avgSlope}%，注意控制呼吸节奏。`,
    STEEP_CLIMB: `陡坡爬升！长度${distKm}公里，累计爬升${gain}米，最大坡度+${seg.max_slope}%，建议步行通过。`,
    DESCENT: `进入下降段，长度${distKm}公里，累计下降${loss}米，平均坡度${avgSlope}%，注意控制速度。`,
    STEEP_DESCENT: `陡降路段！长度${distKm}公里，累计下降${loss}米，最大坡度${seg.max_slope}%，小心湿滑路面。`,
  }
  return templates[seg.type] ?? `路线路段，长度${distKm}公里。`
}

function segmentColor(type: string): string {
  const colors: Record<string, string> = {
    FLAT: '#8899aa',
    CLIMB: '#f0a500',
    STEEP_CLIMB: '#e94560',
    DESCENT: '#533483',
    STEEP_DESCENT: '#ff6b6b',
  }
  return colors[type] || '#8899aa'
}

function typeLabel(type: string): string {
  const labels: Record<string, string> = {
    FLAT: '平缓',
    CLIMB: '爬升',
    STEEP_CLIMB: '陡坡',
    DESCENT: '下降',
    STEEP_DESCENT: '陡降',
  }
  return labels[type] || type
}
</script>

<style scoped>
.segment-list { background: #16213e; border-right: 1px solid #0f3460; padding: 12px; width: 280px; flex-shrink: 0; height: 100%; box-sizing: border-box; display: flex; flex-direction: column; }
.segment-list .segments { overflow-y: auto; flex: 1; min-height: 0; }
.header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 12px; flex-wrap: wrap; gap: 6px; }
.header h3 { font-size: 13px; color: #e94560; }
.actions { display: flex; gap: 6px; align-items: center; flex-wrap: wrap; justify-content: flex-end; }
.btn-commentary { background: #533483; color: #eee; border: none; padding: 4px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; }
.btn-commentary:hover:not(:disabled) { background: #6a429f; }
.btn-commentary:disabled { opacity: 0.5; cursor: not-allowed; }
.preset-select { background: #0f3460; color: #eee; border: none; padding: 4px 6px; border-radius: 4px; font-size: 11px; }
.btn-analyze { background: #0f3460; color: #eee; border: none; padding: 4px 8px; border-radius: 4px; font-size: 11px; cursor: pointer; }
.btn-analyze:hover:not(:disabled) { background: #e94560; }
.btn-analyze:disabled { opacity: 0.5; cursor: not-allowed; }
.empty { color: #667; font-size: 12px; text-align: center; padding: 20px 0; }
.loading { color: #8899aa; font-size: 12px; text-align: center; padding: 20px 0; }
.segments { display: flex; flex-direction: column; gap: 2px; overflow-y: auto; flex: 1; min-height: 0; padding-right: 4px; }
.segment-item { border-left: 3px solid #8899aa; background: #1a1a2e; border-radius: 4px; padding: 8px 10px; cursor: pointer; transition: background 0.15s; }
.segment-item.active { background: #0f3460; outline: 1px solid #e94560; }
.segment-item:hover { background: #1e2a45; }
.seg-header { display: flex; align-items: center; gap: 6px; font-size: 11px; flex-wrap: wrap; }
.seg-type { font-weight: 600; min-width: 36px; }
.seg-dist { color: #aaa; }
.seg-ele { color: #8899aa; }
.seg-slope { color: #f0a500; }
.seg-range { color: #667; margin-left: auto; }
.edit-icon { font-size: 12px; }
.seg-editor { margin-top: 8px; }
.commentary-input { width: 100%; background: #0f3460; color: #eee; border: 1px solid #2a3a5e; border-radius: 4px; padding: 6px; font-size: 11px; resize: vertical; }
.editor-actions { display: flex; gap: 6px; margin-top: 6px; }
.btn-save { background: #4ecca3; color: #16213e; border: none; padding: 3px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; font-weight: 600; }
.btn-cancel { background: transparent; color: #8899aa; border: 1px solid #0f3460; padding: 3px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; }
.btn-merge { background: #533483; color: #eee; border: none; padding: 3px 10px; border-radius: 4px; font-size: 11px; cursor: pointer; }
</style>
