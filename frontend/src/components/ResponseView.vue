<script setup>
import { renderMarkdown } from '../utils/markdown'

const props = defineProps({
  imageUrl: String,
  flow: Object,
  modelResponses: Object,
  selectedModelTab: String,
  currentStep: Number,
  totalSteps: Number,
  loading: Boolean
})

const emit = defineEmits(['close', 'selectTab'])

function getModelResponseLength(model) {
  const responses = props.modelResponses[model] || {}
  return Object.values(responses).join('').length
}
</script>

<template>
  <div class="response-view">
    <div class="response-header">
      <img :src="imageUrl" class="thumbnail" />
      <div class="model-tabs">
        <button
          v-for="model in (flow?.models || [])"
          :key="model"
          :class="['tab', { active: selectedModelTab === model }]"
          @click="emit('selectTab', model)"
        >
          {{ model.split('/').pop() }}
          <span class="tab-stats" v-if="getModelResponseLength(model)">{{ getModelResponseLength(model) }}</span>
        </button>
      </div>
      <div class="step-indicator" v-if="totalSteps > 0">
        {{ Math.min(currentStep + 1, totalSteps) }}/{{ totalSteps }}
        <span v-if="loading" class="loading-dot">...</span>
      </div>
      <button class="close-btn" @click="emit('close')">&times;</button>
    </div>
    <div class="response-content">
      <template v-for="(_, stepIdx) in (flow?.steps || [])" :key="stepIdx">
        <div v-if="modelResponses[selectedModelTab]?.[stepIdx]" class="step-response">
          <div class="step-label">Step {{ stepIdx + 1 }}</div>
          <div class="markdown-body" v-html="renderMarkdown(modelResponses[selectedModelTab][stepIdx])"></div>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.response-view {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  background: #0d0d0d;
}

.response-header {
  display: flex;
  align-items: center;
  padding: 16px;
  gap: 16px;
  border-bottom: 1px solid #333;
}

.thumbnail {
  height: 60px;
  width: auto;
  border-radius: 4px;
}

.step-indicator {
  color: #888;
  font-size: 14px;
}

.loading-dot {
  animation: blink 1s infinite;
}

@keyframes blink {
  50% { opacity: 0; }
}

.close-btn {
  margin-left: auto;
  background: rgba(255, 255, 255, 0.1);
  border: none;
  color: #e0e0e0;
  width: 36px;
  height: 36px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.2);
}

.model-tabs {
  display: flex;
  gap: 4px;
  flex: 1;
}

.tab {
  padding: 8px 16px;
  background: transparent;
  border: none;
  color: #888;
  font-size: 13px;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.2s;
}

.tab:hover {
  background: rgba(255, 255, 255, 0.1);
  color: #e0e0e0;
}

.tab.active {
  background: #333;
  color: #e0e0e0;
}

.tab-stats {
  font-size: 11px;
  color: #666;
  margin-left: 6px;
}

.response-content {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.step-response {
  margin-bottom: 20px;
}

.step-label {
  font-size: 11px;
  color: #666;
  margin-bottom: 8px;
  text-transform: uppercase;
}

.markdown-body {
  font-family: system-ui, -apple-system, sans-serif;
  font-size: 14px;
  line-height: 1.6;
  color: #e0e0e0;
}

.markdown-body :deep(p) {
  margin: 0 0 1em;
}

.markdown-body :deep(p:last-child) {
  margin-bottom: 0;
}

.markdown-body :deep(pre) {
  background: #1a1a1a;
  border-radius: 6px;
  padding: 12px;
  overflow-x: auto;
  margin: 1em 0;
}

.markdown-body :deep(code) {
  font-family: 'SF Mono', Monaco, Consolas, monospace;
  font-size: 13px;
}

.markdown-body :deep(:not(pre) > code) {
  background: #333;
  padding: 2px 6px;
  border-radius: 4px;
}

.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin: 1.5em 0 0.5em;
  font-weight: 600;
}

.markdown-body :deep(h1:first-child),
.markdown-body :deep(h2:first-child),
.markdown-body :deep(h3:first-child) {
  margin-top: 0;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 1em 0;
  padding-left: 1.5em;
}

.markdown-body :deep(blockquote) {
  border-left: 3px solid #444;
  margin: 1em 0;
  padding-left: 1em;
  color: #999;
}

.markdown-body :deep(table) {
  border-collapse: collapse;
  margin: 1em 0;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid #333;
  padding: 8px 12px;
}

.markdown-body :deep(th) {
  background: #1a1a1a;
}
</style>
