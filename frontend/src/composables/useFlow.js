import { ref } from 'vue'
import { useLogger } from '../utils/logger'

const log = useLogger('Flow')

export function useFlow(selectedFlow, currentImage, errorMessage) {
  const flowLoading = ref(false)
  const showResponse = ref(false)
  const currentStep = ref(0)
  const totalSteps = ref(0)
  const modelResponses = ref({})
  const selectedModelTab = ref('')

  let abortController = null

  function clearResponse() {
    showResponse.value = false
    modelResponses.value = {}
    currentStep.value = 0
  }

  async function runFlow(flowId) {
    if (!currentImage.value || flowLoading.value || !flowId) return

    const flow = selectedFlow.value
    if (!flow) return

    log.info(`Running flow: ${flow.name} (${flowId})`)
    log.info(`Image: ${currentImage.value}, Models: ${flow.models.join(', ')}`)

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
        body: JSON.stringify({ image: currentImage.value, flow_id: flowId }),
        signal: abortController.signal
      })

      if (!resp.ok) {
        const error = await resp.text()
        log.error('Flow request failed', error)
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
        log.error('Flow error', err)
        showResponse.value = false
        errorMessage.value = err.message
      } else {
        log.info('Flow aborted')
      }
    } finally {
      log.info('Flow completed')
      flowLoading.value = false
      abortController = null
    }
  }

  function stopFlow() {
    if (abortController) {
      abortController.abort()
    }
  }

  function updateFromSSE(state) {
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
  }

  return {
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
  }
}
