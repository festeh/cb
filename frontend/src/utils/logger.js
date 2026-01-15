const LOG_LEVELS = {
  debug: 0,
  info: 1,
  warn: 2,
  error: 3
}

const currentLevel = LOG_LEVELS.debug

function timestamp() {
  const now = new Date()
  const pad = (n, len = 2) => String(n).padStart(len, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}.${pad(now.getMilliseconds(), 3)}`
}

function formatMessage(level, context, message, data) {
  const prefix = `${timestamp()} [${level.toUpperCase()}]`
  if (context) {
    return data !== undefined
      ? `${prefix} [${context}] ${message}`
      : `${prefix} [${context}] ${message}`
  }
  return `${prefix} ${message}`
}

function createLogger(context) {
  return {
    debug(message, data) {
      if (currentLevel <= LOG_LEVELS.debug) {
        if (data !== undefined) {
          console.debug(formatMessage('debug', context, message), data)
        } else {
          console.debug(formatMessage('debug', context, message))
        }
      }
    },

    info(message, data) {
      if (currentLevel <= LOG_LEVELS.info) {
        if (data !== undefined) {
          console.info(formatMessage('info', context, message), data)
        } else {
          console.info(formatMessage('info', context, message))
        }
      }
    },

    warn(message, data) {
      if (currentLevel <= LOG_LEVELS.warn) {
        if (data !== undefined) {
          console.warn(formatMessage('warn', context, message), data)
        } else {
          console.warn(formatMessage('warn', context, message))
        }
      }
    },

    error(message, error) {
      if (currentLevel <= LOG_LEVELS.error) {
        if (error !== undefined) {
          console.error(formatMessage('error', context, message), error)
        } else {
          console.error(formatMessage('error', context, message))
        }
      }
    }
  }
}

// Default logger without context
export const log = createLogger(null)

// Factory for contextual loggers
export function useLogger(context) {
  return createLogger(context)
}
