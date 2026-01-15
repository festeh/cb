import { ref, computed, watch } from 'vue'
import { useLogger } from '../utils/logger'

const log = useLogger('Settings')

export function useSettings() {
  const showSettings = ref(false)
  const settings = ref({
    openrouter_api_key: '',
    flows: [],
    default_flow_id: ''
  })
  const selectedFlowId = ref('')
  const errorMessage = ref('')

  const selectedFlow = computed(() =>
    settings.value.flows.find(f => f.id === selectedFlowId.value)
  )

  async function loadSettings() {
    log.info('Loading settings...')
    try {
      const resp = await fetch('/api/settings')
      const data = await resp.json()
      log.info(`Loaded ${data.flows?.length || 0} flows`)
      settings.value = {
        openrouter_api_key: data.openrouter_api_key || '',
        flows: data.flows || [],
        default_flow_id: data.default_flow_id || ''
      }
      selectedFlowId.value = settings.value.default_flow_id
    } catch (err) {
      log.error('Failed to load settings', err)
    }
  }

  async function saveSettings() {
    log.info('Saving settings...')
    try {
      const resp = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings.value)
      })
      if (resp.ok) {
        log.info('Settings saved')
        showSettings.value = false
      } else {
        const text = await resp.text()
        log.error('Failed to save settings', text)
        errorMessage.value = text
      }
    } catch (err) {
      log.error('Failed to save settings', err)
      errorMessage.value = err.message
    }
  }

  // Update default flow when selection changes
  watch(selectedFlowId, (newId) => {
    if (newId && newId !== settings.value.default_flow_id) {
      settings.value.default_flow_id = newId
      saveSettings()
    }
  })

  return {
    showSettings,
    settings,
    selectedFlowId,
    selectedFlow,
    errorMessage,
    loadSettings,
    saveSettings
  }
}
