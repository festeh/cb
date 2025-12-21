<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import MarkdownIt from 'markdown-it'
import hljs from 'highlight.js'
import mk from '@traptitech/markdown-it-katex'

const md = new MarkdownIt({
  html: true,
  highlight: (str, lang) => {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return hljs.highlight(str, { language: lang }).value
      } catch (_) {}
    }
    return ''
  }
}).use(mk)

const images = ref([])
const currentIndex = ref(0)
const connected = ref(false)

// Settings
const showSettings = ref(false)
const settings = ref({
  openrouter_api_key: '',
  flows: [],
  default_flow_id: ''
})

// Flow editor
const editingFlow = ref(null)
const showFlowEditor = ref(false)

// Flow execution
const selectedFlowId = ref('')
const flowLoading = ref(false)
const showResponse = ref(false)
const currentStep = ref(0)
const totalSteps = ref(0)
const modelResponses = ref({}) // { model: { step: text } }
const selectedModelTab = ref('')

// Error
const errorMessage = ref('')

// Abort controller for stopping flow
let abortController = null

const currentImage = computed(() => images.value[currentIndex.value] || null)
const currentImageUrl = computed(() =>
  currentImage.value ? '/api/images/' + currentImage.value : ''
)
const canGoBack = computed(() => currentIndex.value < images.value.length - 1)
const canGoForward = computed(() => currentIndex.value > 0)

const selectedFlow = computed(() =>
  settings.value.flows.find(f => f.id === selectedFlowId.value)
)

function renderMarkdown(text) {
  if (!text) return ''
  return md.render(text)
}

function getModelResponseLength(model) {
  const responses = modelResponses.value[model] || {}
  return Object.values(responses).join('').length
}

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

function goBack() {
  if (canGoBack.value) {
    currentIndex.value++
    clearResponse()
  }
}

function goForward() {
  if (canGoForward.value) {
    currentIndex.value--
    clearResponse()
  }
}

function clearResponse() {
  showResponse.value = false
  modelResponses.value = {}
  currentStep.value = 0
}

function handleKeydown(e) {
  if (showSettings.value || showFlowEditor.value) return
  if (e.key === 'ArrowLeft') goBack()
  else if (e.key === 'ArrowRight') goForward()
  else if (e.key === 'Escape' && showResponse.value) clearResponse()
}

async function loadImages() {
  try {
    const resp = await fetch('/api/images')
    images.value = await resp.json()
  } catch (err) {
    console.error('Failed to load images:', err)
  }
}

async function loadCurrentImage() {
  try {
    const resp = await fetch('/api/current-image')
    const data = await resp.json()
    if (data.image && images.value.length > 0) {
      const idx = images.value.indexOf(data.image)
      if (idx >= 0) {
        currentIndex.value = idx
      }
    }
  } catch (err) {
    console.error('Failed to load current image:', err)
  }
}

async function reportCurrentImage() {
  const img = currentImage.value
  if (!img) return
  try {
    await fetch('/api/current-image', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ image: img })
    })
  } catch (err) {
    console.error('Failed to report current image:', err)
  }
}

async function loadSettings() {
  try {
    const resp = await fetch('/api/settings')
    const data = await resp.json()
    settings.value = {
      openrouter_api_key: data.openrouter_api_key || '',
      flows: data.flows || [],
      default_flow_id: data.default_flow_id || ''
    }
    selectedFlowId.value = settings.value.default_flow_id
  } catch (err) {
    console.error('Failed to load settings:', err)
  }
}

async function saveSettings() {
  try {
    await fetch('/api/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
    })
    showSettings.value = false
  } catch (err) {
    console.error('Failed to save settings:', err)
  }
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

function addStep() {
  editingFlow.value.steps.push({ prompt: '' })
}

function removeStep(idx) {
  if (editingFlow.value.steps.length > 1) {
    editingFlow.value.steps.splice(idx, 1)
  }
}

function addModel() {
  editingFlow.value.models.push('')
}

function removeModel(idx) {
  if (editingFlow.value.models.length > 1) {
    editingFlow.value.models.splice(idx, 1)
  }
}

// Update default flow when selection changes
watch(selectedFlowId, (newId) => {
  if (newId && newId !== settings.value.default_flow_id) {
    settings.value.default_flow_id = newId
    saveSettings()
  }
})

// Report image selection to backend
watch(currentIndex, () => {
  reportCurrentImage()
})

async function runFlow() {
  if (!currentImage.value || flowLoading.value || !selectedFlowId.value) return

  const flow = selectedFlow.value
  if (!flow) return

  abortController = new AbortController()
  flowLoading.value = true
  modelResponses.value = {}
  currentStep.value = 0
  totalSteps.value = flow.steps.length
  selectedModelTab.value = flow.models[0] || ''
  showResponse.value = true

  // Initialize response containers for each model
  for (const model of flow.models) {
    modelResponses.value[model] = {}
    for (let i = 0; i < flow.steps.length; i++) {
      modelResponses.value[model][i] = ''
    }
  }

  try {
    const resp = await fetch('/api/flow', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ image: currentImage.value, flow_id: selectedFlowId.value }),
      signal: abortController.signal
    })

    if (!resp.ok) {
      const error = await resp.text()
      showResponse.value = false
      errorMessage.value = error
      flowLoading.value = false
      return
    }

    const reader = resp.body.getReader()
    const decoder = new TextDecoder()

    while (true) {
      const { done, value } = await reader.read()
      if (done) break

      const chunk = decoder.decode(value)
      const lines = chunk.split('\n')

      for (const line of lines) {
        if (line.startsWith('data: ')) {
          const data = line.slice(6)
          if (data === '[DONE]') break

          try {
            const parsed = JSON.parse(data)
            if (parsed.content && parsed.model) {
              if (!modelResponses.value[parsed.model]) {
                modelResponses.value[parsed.model] = {}
              }
              if (!modelResponses.value[parsed.model][parsed.step]) {
                modelResponses.value[parsed.model][parsed.step] = ''
              }
              modelResponses.value[parsed.model][parsed.step] += parsed.content
            }
            if (parsed.done && parsed.step !== undefined) {
              currentStep.value = parsed.step + 1
            }
            if (parsed.error && parsed.model) {
              if (!modelResponses.value[parsed.model]) {
                modelResponses.value[parsed.model] = {}
              }
              modelResponses.value[parsed.model][parsed.step] = `Error: ${parsed.error}`
            }
          } catch (e) {
            // Ignore parse errors
          }
        }
      }
    }
  } catch (err) {
    if (err.name !== 'AbortError') {
      showResponse.value = false
      errorMessage.value = err.message
    }
  } finally {
    flowLoading.value = false
    abortController = null
  }
}

function stopFlow() {
  if (abortController) {
    abortController.abort()
  }
}

let eventSource = null
let reconnectTimeout = null

function connectSSE() {
  if (eventSource) {
    eventSource.close()
  }

  eventSource = new EventSource('/api/events')

  eventSource.onopen = () => {
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
  }

  eventSource.addEventListener('connected', () => {
    console.log('SSE connected')
    connected.value = true
  })

  eventSource.onerror = () => {
    connected.value = false
    eventSource.close()
    // Retry after 3 seconds
    if (!reconnectTimeout) {
      reconnectTimeout = setTimeout(() => {
        reconnectTimeout = null
        connectSSE()
      }, 3000)
    }
  }

  eventSource.addEventListener('newimage', (e) => {
    images.value.unshift(e.data)
    currentIndex.value = 0
    clearResponse()
  })

  eventSource.addEventListener('scroll', (e) => {
    console.log('SSE scroll event:', e.data)
    const container = document.querySelector('.response-content')
    if (container) {
      const step = 200
      container.scrollTop += e.data === 'up' ? -step : step
      console.log('Scrolled container to:', container.scrollTop)
    } else {
      console.log('No .response-content container found')
    }
  })

  eventSource.addEventListener('tabchange', (e) => {
    console.log('SSE tabchange event:', e.data)
    const tabIdx = parseInt(e.data)
    const flow = selectedFlow.value
    if (flow && flow.models && tabIdx >= 0 && tabIdx < flow.models.length) {
      selectedModelTab.value = flow.models[tabIdx]
      console.log('Switched to tab:', flow.models[tabIdx])
    }
  })

  eventSource.addEventListener('flowupdate', (e) => {
    try {
      const state = JSON.parse(e.data)
      if (!state.responses) return

      // Update model responses from SSE
      for (const model of Object.keys(state.responses)) {
        if (!modelResponses.value[model]) {
          modelResponses.value[model] = {}
        }
        for (const [step, text] of Object.entries(state.responses[model])) {
          modelResponses.value[model][parseInt(step)] = text
        }
      }

      // Update UI state if not already in a flow
      if (!flowLoading.value && state.running) {
        showResponse.value = true
        totalSteps.value = Object.keys(state.responses[Object.keys(state.responses)[0]] || {}).length || 1
        // Set initial tab only if not already set
        if (!selectedModelTab.value && state.models && state.models.length > 0) {
          selectedModelTab.value = state.models[0]
        }
      }

      currentStep.value = state.step
    } catch (err) {
      console.error('Failed to parse flowupdate:', err)
    }
  })
}

onMounted(async () => {
  await loadImages()
  await loadCurrentImage()
  loadSettings()
  connectSSE()
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
  if (eventSource) {
    eventSource.close()
  }
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
    <div v-if="showResponse" class="response-view">
      <div class="response-header">
        <img :src="currentImageUrl" class="thumbnail" />
        <div class="model-tabs">
          <button
            v-for="model in (selectedFlow?.models || [])"
            :key="model"
            :class="['tab', { active: selectedModelTab === model }]"
            @click="selectedModelTab = model"
          >
            {{ model.split('/').pop() }}
            <span class="tab-stats" v-if="getModelResponseLength(model)">{{ getModelResponseLength(model) }}</span>
          </button>
        </div>
        <div class="step-indicator" v-if="totalSteps > 0">
          {{ Math.min(currentStep + 1, totalSteps) }}/{{ totalSteps }}
          <span v-if="flowLoading" class="loading-dot">...</span>
        </div>
        <button class="close-btn" @click="clearResponse">&times;</button>
      </div>
      <div class="response-content">
        <template v-for="(_, stepIdx) in (selectedFlow?.steps || [])" :key="stepIdx">
          <div v-if="modelResponses[selectedModelTab]?.[stepIdx]" class="step-response">
            <div class="step-label">Step {{ stepIdx + 1 }}</div>
            <div class="markdown-body" v-html="renderMarkdown(modelResponses[selectedModelTab][stepIdx])"></div>
          </div>
        </template>
      </div>
    </div>

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
          @click="runFlow"
          :disabled="!selectedFlowId"
        >
          Run
        </button>
      </div>
    </template>

    <div v-if="images.length > 0" class="counter">
      {{ images.length - currentIndex }} / {{ images.length }}
    </div>

    <!-- Error popup -->
    <div v-if="errorMessage" class="modal-overlay" @click.self="errorMessage = ''">
      <div class="modal error-modal">
        <h2>Error</h2>
        <p class="error-text">{{ errorMessage }}</p>
        <div class="modal-actions">
          <button class="btn-primary" @click="errorMessage = ''">OK</button>
        </div>
      </div>
    </div>

    <!-- Settings modal -->
    <div v-if="showSettings" class="modal-overlay" @click.self="showSettings = false">
      <div class="modal settings-modal">
        <h2>Settings</h2>

        <div class="form-group">
          <label>OpenRouter API Key</label>
          <input
            type="password"
            v-model="settings.openrouter_api_key"
            placeholder="sk-or-..."
          />
        </div>

        <div class="form-group">
          <label>Flows</label>
          <div class="flows-list">
            <div v-for="flow in settings.flows" :key="flow.id" class="flow-item">
              <span class="flow-name">{{ flow.name }}</span>
              <span class="flow-info">{{ flow.steps.length }} steps, {{ flow.models.length }} models</span>
              <button class="btn-small" @click="editFlow(flow)">Edit</button>
              <button class="btn-small btn-danger" @click="deleteFlow(flow.id)">Delete</button>
            </div>
            <button class="btn-secondary" @click="createNewFlow">+ Add Flow</button>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn-secondary" @click="showSettings = false">Cancel</button>
          <button class="btn-primary" @click="saveSettings">Save</button>
        </div>
      </div>
    </div>

    <!-- Flow editor modal -->
    <div v-if="showFlowEditor && editingFlow" class="modal-overlay" @click.self="showFlowEditor = false">
      <div class="modal flow-editor-modal">
        <h2>{{ editingFlow.id ? 'Edit Flow' : 'New Flow' }}</h2>

        <div class="form-group">
          <label>Flow Name</label>
          <input type="text" v-model="editingFlow.name" placeholder="My Flow" />
        </div>

        <div class="form-group">
          <label>Steps</label>
          <div class="steps-list">
            <div v-for="(step, idx) in editingFlow.steps" :key="idx" class="step-item">
              <span class="step-number">{{ idx + 1 }}</span>
              <textarea
                v-model="step.prompt"
                placeholder="Enter prompt for this step..."
                rows="2"
              ></textarea>
              <button
                class="btn-small btn-danger"
                @click="removeStep(idx)"
                :disabled="editingFlow.steps.length <= 1"
              >-</button>
            </div>
            <button class="btn-secondary" @click="addStep">+ Add Step</button>
          </div>
        </div>

        <div class="form-group">
          <label>Models</label>
          <div class="models-list">
            <div v-for="(model, idx) in editingFlow.models" :key="idx" class="model-item">
              <input
                type="text"
                v-model="editingFlow.models[idx]"
                placeholder="anthropic/claude-sonnet-4"
              />
              <button
                class="btn-small btn-danger"
                @click="removeModel(idx)"
                :disabled="editingFlow.models.length <= 1"
              >-</button>
            </div>
            <button class="btn-secondary" @click="addModel">+ Add Model</button>
          </div>
        </div>

        <div class="modal-actions">
          <button class="btn-secondary" @click="showFlowEditor = false">Cancel</button>
          <button class="btn-primary" @click="saveFlow">Save Flow</button>
        </div>
      </div>
    </div>
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

/* Response view */
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

.markdown-body p {
  margin: 0 0 1em;
}

.markdown-body p:last-child {
  margin-bottom: 0;
}

.markdown-body pre {
  background: #1a1a1a;
  border-radius: 6px;
  padding: 12px;
  overflow-x: auto;
  margin: 1em 0;
}

.markdown-body code {
  font-family: 'SF Mono', Monaco, Consolas, monospace;
  font-size: 13px;
}

.markdown-body :not(pre) > code {
  background: #333;
  padding: 2px 6px;
  border-radius: 4px;
}

.markdown-body h1, .markdown-body h2, .markdown-body h3 {
  margin: 1.5em 0 0.5em;
  font-weight: 600;
}

.markdown-body h1:first-child, .markdown-body h2:first-child, .markdown-body h3:first-child {
  margin-top: 0;
}

.markdown-body ul, .markdown-body ol {
  margin: 1em 0;
  padding-left: 1.5em;
}

.markdown-body blockquote {
  border-left: 3px solid #444;
  margin: 1em 0;
  padding-left: 1em;
  color: #999;
}

.markdown-body table {
  border-collapse: collapse;
  margin: 1em 0;
}

.markdown-body th, .markdown-body td {
  border: 1px solid #333;
  padding: 8px 12px;
}

.markdown-body th {
  background: #1a1a1a;
}

/* Modals */
.modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  background: #1a1a1a;
  border-radius: 12px;
  padding: 24px;
  width: 90%;
  max-width: 500px;
  border: 1px solid #333;
  max-height: 90vh;
  overflow-y: auto;
}

.settings-modal {
  max-width: 600px;
}

.flow-editor-modal {
  max-width: 700px;
}

.modal h2 {
  margin: 0 0 20px 0;
  font-weight: 500;
  color: #e0e0e0;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 14px;
  color: #888;
}

.form-group input,
.form-group textarea,
.form-group select {
  width: 100%;
  padding: 10px 12px;
  background: #0d0d0d;
  border: 1px solid #333;
  border-radius: 6px;
  color: #e0e0e0;
  font-size: 14px;
  font-family: inherit;
}

.form-group input:focus,
.form-group textarea:focus {
  outline: none;
  border-color: #3b82f6;
}

.form-group textarea {
  resize: vertical;
}

.modal-actions {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  margin-top: 24px;
}

.btn-primary,
.btn-secondary {
  padding: 10px 20px;
  border-radius: 6px;
  font-size: 14px;
  cursor: pointer;
  border: none;
}

.btn-primary {
  background: #3b82f6;
  color: white;
}

.btn-primary:hover {
  background: #2563eb;
}

.btn-secondary {
  background: #333;
  color: #e0e0e0;
}

.btn-secondary:hover {
  background: #444;
}

.btn-small {
  padding: 4px 10px;
  font-size: 12px;
  border-radius: 4px;
  border: none;
  background: #333;
  color: #e0e0e0;
  cursor: pointer;
}

.btn-small:hover {
  background: #444;
}

.btn-small:disabled {
  opacity: 0.5;
  cursor: default;
}

.btn-danger {
  background: #7f1d1d;
}

.btn-danger:hover {
  background: #991b1b;
}

/* Flows list */
.flows-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.flow-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  background: #0d0d0d;
  border-radius: 6px;
}

.flow-name {
  font-weight: 500;
  color: #e0e0e0;
}

.flow-info {
  color: #666;
  font-size: 12px;
  flex: 1;
}

/* Steps list */
.steps-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.step-item {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.step-number {
  width: 24px;
  height: 24px;
  background: #333;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: #888;
  flex-shrink: 0;
  margin-top: 8px;
}

.step-item textarea {
  flex: 1;
}

/* Models list */
.models-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.model-item {
  display: flex;
  gap: 8px;
}

.model-item input {
  flex: 1;
}

.error-modal h2 {
  color: #f87171;
}

.error-text {
  color: #e0e0e0;
  line-height: 1.5;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
