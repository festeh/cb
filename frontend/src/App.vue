<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'

const images = ref([])
const currentIndex = ref(0)
const connected = ref(false)

// Settings
const showSettings = ref(false)
const settings = ref({
  openrouter_api_key: '',
  model: '',
  prompt: ''
})

// Flow response
const flowResponse = ref('')
const flowLoading = ref(false)
const showResponse = ref(false)

const currentImage = computed(() => images.value[currentIndex.value] || null)
const currentImageUrl = computed(() =>
  currentImage.value ? '/images/' + currentImage.value : ''
)
const canGoBack = computed(() => currentIndex.value < images.value.length - 1)
const canGoForward = computed(() => currentIndex.value > 0)

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
  flowResponse.value = ''
}

function handleKeydown(e) {
  if (showSettings.value) return
  if (e.key === 'ArrowLeft') goBack()
  else if (e.key === 'ArrowRight') goForward()
  else if (e.key === 'Escape' && showResponse.value) clearResponse()
}

async function loadImages() {
  try {
    const resp = await fetch('/images')
    images.value = await resp.json()
  } catch (err) {
    console.error('Failed to load images:', err)
  }
}

async function loadSettings() {
  try {
    const resp = await fetch('/settings')
    const data = await resp.json()
    settings.value = {
      openrouter_api_key: data.openrouter_api_key || '',
      model: data.model || '',
      prompt: data.prompt || ''
    }
  } catch (err) {
    console.error('Failed to load settings:', err)
  }
}

async function saveSettings() {
  try {
    await fetch('/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
    })
    showSettings.value = false
  } catch (err) {
    console.error('Failed to save settings:', err)
  }
}

async function runFlow() {
  if (!currentImage.value || flowLoading.value) return

  flowLoading.value = true
  flowResponse.value = ''
  showResponse.value = true

  try {
    const resp = await fetch('/flow', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ image: currentImage.value })
    })

    if (!resp.ok) {
      const error = await resp.text()
      flowResponse.value = `Error: ${error}`
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
          flowResponse.value += data
        }
      }
    }
  } catch (err) {
    flowResponse.value = `Error: ${err.message}`
  } finally {
    flowLoading.value = false
  }
}

let eventSource = null

function connectSSE() {
  eventSource = new EventSource('/events')

  eventSource.onopen = () => {
    connected.value = true
  }

  eventSource.onerror = () => {
    connected.value = false
  }

  eventSource.addEventListener('newimage', (e) => {
    images.value.unshift(e.data)
    currentIndex.value = 0
    clearResponse()
  })
}

onMounted(() => {
  loadImages()
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
        <button class="close-btn" @click="clearResponse">&times;</button>
      </div>
      <div class="response-content">
        <div v-if="flowLoading && !flowResponse" class="loading">Loading...</div>
        <pre class="response-text">{{ flowResponse }}</pre>
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

      <!-- Run Flow button -->
      <button
        v-if="currentImage"
        class="flow-btn"
        @click="runFlow"
        :disabled="flowLoading"
      >
        {{ flowLoading ? 'Running...' : 'Run Flow' }}
      </button>
    </template>

    <div v-if="images.length > 0" class="counter">
      {{ images.length - currentIndex }} / {{ images.length }}
    </div>

    <!-- Settings modal -->
    <div v-if="showSettings" class="modal-overlay" @click.self="showSettings = false">
      <div class="modal">
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
          <label>Model</label>
          <input
            type="text"
            v-model="settings.model"
            placeholder="anthropic/claude-sonnet-4"
          />
        </div>

        <div class="form-group">
          <label>Prompt</label>
          <textarea
            v-model="settings.prompt"
            placeholder="Describe this image."
            rows="4"
          ></textarea>
        </div>

        <div class="modal-actions">
          <button class="btn-secondary" @click="showSettings = false">Cancel</button>
          <button class="btn-primary" @click="saveSettings">Save</button>
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
  transition:
    background 0.2s,
    opacity 0.2s;
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
}

.flow-btn {
  position: absolute;
  bottom: 16px;
  right: 16px;
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
  height: 80px;
  width: auto;
  border-radius: 4px;
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

.response-content {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

.response-text {
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: system-ui, -apple-system, sans-serif;
  font-size: 15px;
  line-height: 1.6;
  color: #e0e0e0;
  margin: 0;
}

.loading {
  color: #888;
}

/* Settings modal */
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
.form-group textarea {
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
</style>
