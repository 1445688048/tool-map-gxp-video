<template>
  <div class="intro-outro-config">
    <h3>Intro / Outro 镜头</h3>
    <div class="section">
      <h4>Intro（开场）</h4>
      <label><input v-model.number="introConfig.enabled" type="checkbox" /> 启用</label>
      <div class="field-row">
        <span>时长(s)</span>
        <input v-model.number="introConfig.duration" type="number" min="1" max="10" class="num" />
      </div>
      <div class="field-row">
        <span>起始俯角(°)</span>
        <input v-model.number="introConfig.startPitch" type="number" min="10" max="60" class="num" />
      </div>
      <div class="field-row">
        <span>结束俯角(°)</span>
        <input v-model.number="introConfig.endPitch" type="number" min="30" max="85" class="num" />
      </div>
      <div class="field-row">
        <span>起始高度(m)</span>
        <input v-model.number="introConfig.startAltitude" type="number" min="200" max="5000" class="num" />
      </div>
      <div class="field-row">
        <span>结束高度(m)</span>
        <input v-model.number="introConfig.endAltitude" type="number" min="30" max="500" class="num" />
      </div>
    </div>
    <div class="section">
      <h4>Outro（结尾）</h4>
      <label><input v-model.number="outroConfig.enabled" type="checkbox" /> 启用</label>
      <div class="field-row">
        <span>时长(s)</span>
        <input v-model.number="outroConfig.duration" type="number" min="1" max="10" class="num" />
      </div>
      <div class="field-row">
        <span>起始俯角(°)</span>
        <input v-model.number="outroConfig.startPitch" type="number" min="30" max="85" class="num" />
      </div>
      <div class="field-row">
        <span>结束俯角(°)</span>
        <input v-model.number="outroConfig.endPitch" type="number" min="10" max="50" class="num" />
      </div>
      <div class="field-row">
        <span>结束高度(m)</span>
        <input v-model.number="outroConfig.endAltitude" type="number" min="200" max="5000" class="num" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'

const introConfig = ref({
  enabled: true,
  duration: 3,
  startPitch: 30,
  endPitch: 60,
  startAltitude: 1500,
  endAltitude: 100,
})

const outroConfig = ref({
  enabled: true,
  duration: 4,
  startPitch: 60,
  endPitch: 30,
  endAltitude: 1500,
})

defineExpose({ introConfig, outroConfig })
</script>

<style scoped>
.intro-outro-config { background: #16213e; padding: 12px; }
h3 { font-size: 13px; color: #e94560; margin-bottom: 10px; }
.section { margin-bottom: 14px; }
h4 { font-size: 11px; color: #4ecca3; margin-bottom: 6px; }
label { display: flex; align-items: center; gap: 6px; font-size: 11px; color: #aaa; margin-bottom: 4px; cursor: pointer; }
.field-row { display: flex; align-items: center; justify-content: space-between; font-size: 11px; color: #8899aa; margin-bottom: 3px; }
.num { width: 56px; background: #0f3460; color: #eee; border: 1px solid #2a3a5e; border-radius: 3px; padding: 2px 6px; font-size: 11px; text-align: center; }
</style>
