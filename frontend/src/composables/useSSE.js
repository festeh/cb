import { ref, onUnmounted } from 'vue'
import { useLogger } from '../utils/logger'

const log = useLogger('SSE')

export function useSSE() {
  const connected = ref(false)

  let eventSource = null
  let reconnectTimeout = null
  const listeners = {}

  function on(event, callback) {
    listeners[event] = callback
  }

  function connect() {
    if (eventSource) {
      eventSource.close()
    }

    log.info('Connecting to SSE...')
    eventSource = new EventSource('/api/events')

    eventSource.onopen = () => {
      if (reconnectTimeout) {
        clearTimeout(reconnectTimeout)
        reconnectTimeout = null
      }
    }

    eventSource.addEventListener('connected', () => {
      log.info('Connected')
      connected.value = true
    })

    eventSource.onerror = (e) => {
      log.error('Connection error, reconnecting in 3s...', e)
      connected.value = false
      eventSource.close()
      // Retry after 3 seconds
      if (!reconnectTimeout) {
        reconnectTimeout = setTimeout(() => {
          reconnectTimeout = null
          connect()
        }, 3000)
      }
    }

    // Register custom event listeners
    for (const [event, callback] of Object.entries(listeners)) {
      eventSource.addEventListener(event, callback)
    }
  }

  function disconnect() {
    log.info('Disconnecting')
    if (eventSource) {
      eventSource.close()
      eventSource = null
    }
    if (reconnectTimeout) {
      clearTimeout(reconnectTimeout)
      reconnectTimeout = null
    }
  }

  onUnmounted(disconnect)

  return {
    connected,
    on,
    connect,
    disconnect
  }
}
