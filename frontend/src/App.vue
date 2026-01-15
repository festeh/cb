<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { useImages } from './composables/useImages'
import { useSettings } from './composables/useSettings'
import { useFlow } from './composables/useFlow'
import { useSSE } from './composables/useSSE'
import { useLogger } from './utils/logger'
import ErrorModal from './components/ErrorModal.vue'
import SettingsModal from './components/SettingsModal.vue'
import FlowEditorModal from './components/FlowEditorModal.vue'
import ResponseView from './components/ResponseView.vue'

const log = useLogger('App')

const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']

function formatTimestamp(filename) {
  const match = filename.match(/(\d{4})-(\d{2})-(\d{2})_(\d{2})-(\d{2})-(\d{2})/)
  if (match) {
    const day = parseInt(match[3], 10)
    const month = months[parseInt(match[2], 10) - 1]
    const time = `${match[4]}:${match[5]}:${match[6]}`
    return `${day} ${month} ${time}`
  }
  return filename
}

// Images
const {
  images,
  currentIndex,
  currentImage,
  currentImageUrl,
  canGoBack,
  canGoForward,
  loadImages,
  loadCurrentImage,
  goBack: goBackRaw,
  goForward: goForwardRaw,
  addImage
} = useImages()

// Settings
const {
  showSettings,
  settings,
  selectedFlowId,
  selectedFlow,
  errorMessage,
  loadSettings,
  saveSettings
} = useSettings()

// Flow
const {
  flowLoading,
  showResponse,
  currentStep,
  totalSteps,
  modelResponses,
  selectedModelTab,
  clearResponse,
  runFlow,
  stopFlow,
  updateFromSSE
} = useFlow(selectedFlow, currentImage, errorMessage)

// SSE
const { connected, on, connect } = useSSE()

// Flow editor
const editingFlow = ref(null)
const showFlowEditor = ref(false)

// Navigation with response clearing
function goBack() {
  goBackRaw()
  clearResponse()
}

function goForward() {
  goForwardRaw()
  clearResponse()
}

function handleKeydown(e) {
  if (showSettings.value || showFlowEditor.value) return
  if (e.key === 'ArrowLeft') goBack()
  else if (e.key === 'ArrowRight') goForward()
  else if (e.key === 'Escape' && showResponse.value) clearResponse()
}

// Flow editor functions
function createNewFlow() {
  editingFlow.value = {
    id: crypto.randomUUID(),
    name: 'New Flow',
    steps: [{ prompt: '' }],
    models: ['']
  }
  showFlowEditor.value = true
}

function editFlow(flow) {
  editingFlow.value = JSON.parse(JSON.stringify(flow))
  showFlowEditor.value = true
}

function deleteFlow(flowId) {
  settings.value.flows = settings.value.flows.filter(f => f.id !== flowId)
  if (settings.value.default_flow_id === flowId) {
    settings.value.default_flow_id = ''
  }
  if (selectedFlowId.value === flowId) {
    selectedFlowId.value = ''
  }
}

function saveFlow() {
  const idx = settings.value.flows.findIndex(f => f.id === editingFlow.value.id)
  if (idx >= 0) {
    settings.value.flows[idx] = editingFlow.value
  } else {
    settings.value.flows.push(editingFlow.value)
  }
  showFlowEditor.value = false
  editingFlow.value = null
}

// SSE event handlers
on('newimage', (e) => {
  addImage(e.data)
  clearResponse()
})

on('scroll', (e) => {
  log.debug(`Scroll: ${e.data}`)
  const container = document.querySelector('.response-content')
  if (container) {
    const step = 200
    container.scrollTop += e.data === 'up' ? -step : step
  }
})

on('tabchange', (e) => {
  const tabIdx = parseInt(e.data)
  const flow = selectedFlow.value
  if (flow && flow.models && tabIdx >= 0 && tabIdx < flow.models.length) {
    log.debug(`Tab change: ${tabIdx} -> ${flow.models[tabIdx]}`)
    selectedModelTab.value = flow.models[tabIdx]
  }
})

on('flowupdate', (e) => {
  try {
    const state = JSON.parse(e.data)

    // Sync flow ID from SSE
    if (state.flow_id && state.flow_id !== selectedFlowId.value) {
      log.info(`Flow ID synced: ${state.flow_id}`)
      selectedFlowId.value = state.flow_id
    }

    updateFromSSE(state)
  } catch (err) {
    log.error('Failed to parse flowupdate', err)
  }
})

onMounted(async () => {
  log.info('App mounted')
  await loadImages()
  await loadCurrentImage()
  loadSettings()
  connect()
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="viewer">
    <!-- Settings button -->
    <button class="settings-btn" @click="showSettings = true" title="Settings">
      <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="3"></circle>
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"></path>
      </svg>
    </button>

    <div v-if="currentImage" class="time-header">
      {{ formatTimestamp(currentImage) }}
    </div>

    <div :class="['status', connected ? 'connected' : 'disconnected']">
      {{ connected ? 'Live' : 'Reconnecting...' }}
    </div>

    <!-- Response view -->
    <ResponseView
      v-if="showResponse"
      :imageUrl="currentImageUrl"
      :flow="selectedFlow"
      :modelResponses="modelResponses"
      :selectedModelTab="selectedModelTab"
      :currentStep="currentStep"
      :totalSteps="totalSteps"
      :loading="flowLoading"
      @close="clearResponse"
      @selectTab="selectedModelTab = $event"
    />

    <!-- Normal image view -->
    <template v-else>
      <button class="nav-btn prev" @click="goBack" :disabled="!canGoBack">
        &larr;
      </button>

      <div class="image-container">
        <div v-if="images.length === 0" class="empty">No images yet</div>
        <img v-else :src="currentImageUrl" :key="currentImage" />
      </div>

      <button class="nav-btn next" @click="goForward" :disabled="!canGoForward">
        &rarr;
      </button>

      <!-- Flow controls -->
      <div v-if="currentImage" class="flow-controls">
        <select v-model="selectedFlowId" class="flow-select">
          <option value="" disabled>Select flow...</option>
          <option v-for="flow in settings.flows" :key="flow.id" :value="flow.id">
            {{ flow.name }}
          </option>
        </select>
        <button
          v-if="flowLoading"
          class="flow-btn stop"
          @click="stopFlow"
        >
          Stop
        </button>
        <button
          v-else
          class="flow-btn"
          @click="runFlow(selectedFlowId)"
          :disabled="!selectedFlowId"
        >
          Run
        </button>
      </div>
    </template>

    <div v-if="images.length > 0" class="counter">
      {{ images.length - currentIndex }} / {{ images.length }}
    </div>

    <!-- Modals -->
    <ErrorModal
      v-if="errorMessage"
      :message="errorMessage"
      @close="errorMessage = ''"
    />

    <SettingsModal
      v-if="showSettings"
      :settings="settings"
      @close="showSettings = false"
      @save="saveSettings"
      @editFlow="editFlow"
      @deleteFlow="deleteFlow"
      @createFlow="createNewFlow"
    />

    <FlowEditorModal
      v-if="showFlowEditor && editingFlow"
      :flow="editingFlow"
      @close="showFlowEditor = false"
      @save="saveFlow"
    />
  </div>
</template>

<style scoped>
.viewer {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

.settings-btn {
  position: absolute;
  top: 16px;
  left: 16px;
  background: rgba(255, 255, 255, 0.1);
  border: none;
  color: #e0e0e0;
  width: 40px;
  height: 40px;
  border-radius: 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.2s;
  z-index: 10;
}

.settings-btn:hover {
  background: rgba(255, 255, 255, 0.2);
}

.image-container {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  width: 100%;
  padding: 20px;
}

.image-container img {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
  border-radius: 4px;
}

.nav-btn {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  background: rgba(255, 255, 255, 0.1);
  border: none;
  color: #e0e0e0;
  font-size: 48px;
  width: 80px;
  height: 120px;
  cursor: pointer;
  transition: background 0.2s, opacity 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  user-select: none;
  border-radius: 8px;
}

.nav-btn:hover {
  background: rgba(255, 255, 255, 0.2);
}

.nav-btn:disabled {
  opacity: 0.2;
  cursor: default;
}

.nav-btn:disabled:hover {
  background: rgba(255, 255, 255, 0.1);
}

.nav-btn.prev {
  left: 20px;
}

.nav-btn.next {
  right: 20px;
}

.status {
  position: absolute;
  top: 16px;
  right: 16px;
  font-size: 12px;
  padding: 6px 12px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.5);
  z-index: 10;
}

.status.connected {
  color: #4ade80;
}

.status.disconnected {
  color: #f87171;
}

.counter {
  position: absolute;
  bottom: 16px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 14px;
  color: #888;
  background: rgba(0, 0, 0, 0.5);
  padding: 8px 16px;
  border-radius: 4px;
}

.empty {
  color: #666;
  font-size: 18px;
}

.time-header {
  position: absolute;
  top: 16px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 18px;
  color: #e0e0e0;
  background: rgba(0, 0, 0, 0.5);
  padding: 8px 16px;
  border-radius: 4px;
  z-index: 10;
}

/* Flow controls */
.flow-controls {
  position: absolute;
  bottom: 16px;
  right: 16px;
  display: flex;
  gap: 8px;
}

.flow-select {
  padding: 10px 12px;
  background: #1a1a1a;
  border: 1px solid #333;
  border-radius: 8px;
  color: #e0e0e0;
  font-size: 14px;
  min-width: 150px;
}

.flow-btn {
  background: #3b82f6;
  border: none;
  color: white;
  padding: 10px 20px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  transition: background 0.2s;
}

.flow-btn:hover {
  background: #2563eb;
}

.flow-btn:disabled {
  background: #4b5563;
  cursor: default;
}

.flow-btn.stop {
  background: #dc2626;
}

.flow-btn.stop:hover {
  background: #b91c1c;
}
</style>
