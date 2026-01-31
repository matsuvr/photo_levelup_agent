/**
 * Security-first logging utility
 * Automatically masks sensitive information to prevent credential leaks
 *
 * CRITICAL: Never log the following:
 * - API keys (Google: AIzaSy..., Firebase, etc.)
 * - Authentication tokens (Bearer, JWT, access tokens, refresh tokens)
 * - Passwords or credentials
 * - Private keys (RSA, EC, etc.)
 * - Credit card numbers
 * - Personal identifiable information (PII) when possible
 */

type LogLevel = 'debug' | 'info' | 'warn' | 'error'

interface SensitivePattern {
  name: string
  pattern: RegExp
  mask: string
}

// Patterns for sensitive data detection
const SENSITIVE_PATTERNS: SensitivePattern[] = [
  // Google API Keys (AIzaSy...)
  { name: 'google-api-key', pattern: /AIzaSy[a-zA-Z0-9_-]{33}/g, mask: '[GOOGLE_API_KEY_MASKED]' },

  // Generic API keys
  { name: 'api-key', pattern: /api[_-]?key[:\s=]+['"]?[a-zA-Z0-9_-]{16,}['"]?/gi, mask: '[API_KEY_MASKED]' },
  { name: 'apikey', pattern: /apikey[:\s=]+['"]?[a-zA-Z0-9_-]{16,}['"]?/gi, mask: '[API_KEY_MASKED]' },

  // Authentication tokens
  { name: 'bearer-token', pattern: /Bearer\s+[a-zA-Z0-9_-]+(\.[a-zA-Z0-9_-]+)*/g, mask: '[BEARER_TOKEN_MASKED]' },
  { name: 'jwt-token', pattern: /eyJ[a-zA-Z0-9_-]*\.eyJ[a-zA-Z0-9_-]*\.[a-zA-Z0-9_-]*/g, mask: '[JWT_MASKED]' },
  { name: 'access-token', pattern: /access[_-]?token[:\s=]+['"]?[a-zA-Z0-9_-]{8,}['"]?/gi, mask: '[ACCESS_TOKEN_MASKED]' },
  { name: 'refresh-token', pattern: /refresh[_-]?token[:\s=]+['"]?[a-zA-Z0-9_-]{8,}['"]?/gi, mask: '[REFRESH_TOKEN_MASKED]' },

  // Passwords
  { name: 'password', pattern: /password[:\s=]+['"]?[^\s'"]+['"]?/gi, mask: '[PASSWORD_MASKED]' },
  { name: 'passwd', pattern: /passwd[:\s=]+['"]?[^\s'"]+['"]?/gi, mask: '[PASSWORD_MASKED]' },
  { name: 'pwd', pattern: /pwd[:\s=]+['"]?[^\s'"]+['"]?/gi, mask: '[PASSWORD_MASKED]' },

  // Private keys and certificates
  { name: 'private-key', pattern: /-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----[\s\S]*?-----END (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----/g, mask: '[PRIVATE_KEY_MASKED]' },
  { name: 'certificate', pattern: /-----BEGIN CERTIFICATE-----[\s\S]*?-----END CERTIFICATE-----/g, mask: '[CERTIFICATE_MASKED]' },

  // Credit cards (basic pattern)
  { name: 'credit-card', pattern: /\b(?:\d{4}[-\s]?){3}\d{4}\b/g, mask: '[CREDIT_CARD_MASKED]' },

  // Firebase config with API key
  { name: 'firebase-config', pattern: /apiKey[:\s]+['"][a-zA-Z0-9_-]{10,}['"]/gi, mask: 'apiKey: [FIREBASE_API_KEY_MASKED]' },

  // Service account keys
  { name: 'service-account', pattern: /"private_key_id"\s*:\s*"[^"]+"/g, mask: '"private_key_id": "[MASKED]"' },
  { name: 'client-secret', pattern: /client[_-]?secret[:\s=]+['"]?[a-zA-Z0-9_-]{8,}['"]?/gi, mask: '[CLIENT_SECRET_MASKED]' },
]

/**
 * Sanitizes a string by masking sensitive patterns
 */
function sanitizeMessage(message: string): string {
  let sanitized = message

  for (const { pattern, mask } of SENSITIVE_PATTERNS) {
    sanitized = sanitized.replace(pattern, mask)
  }

  return sanitized
}

/**
 * Converts any value to a safe string representation
 */
function toSafeString(value: unknown): string {
  if (value === null) return 'null'
  if (value === undefined) return 'undefined'
  if (typeof value === 'string') return sanitizeMessage(value)
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  if (value instanceof Error) return sanitizeMessage(`${value.name}: ${value.message}`)

  // For objects, safely stringify and then sanitize
  try {
    const stringified = JSON.stringify(value, null, 2)
    return sanitizeMessage(stringified)
  } catch {
    return '[Object with circular reference or non-serializable data]'
  }
}

/**
 * Logs a message with the specified level, automatically sanitizing sensitive data
 */
function log(level: LogLevel, message: string, ...args: unknown[]): void {
  const sanitizedMessage = sanitizeMessage(message)
  const sanitizedArgs = args.map(arg => toSafeString(arg))

  const timestamp = new Date().toISOString()
  const prefix = `[${timestamp}] [${level.toUpperCase()}]`

  switch (level) {
    case 'debug':
      // Only log debug in development
      if (process.env.NODE_ENV === 'development') {
        console.debug(prefix, sanitizedMessage, ...sanitizedArgs)
      }
      break
    case 'info':
      console.info(prefix, sanitizedMessage, ...sanitizedArgs)
      break
    case 'warn':
      console.warn(prefix, sanitizedMessage, ...sanitizedArgs)
      break
    case 'error':
      console.error(prefix, sanitizedMessage, ...sanitizedArgs)
      break
  }
}

/**
 * Validates that a message does not contain obvious sensitive data patterns
 * Returns true if the message appears safe to log
 */
export function validateSafeToLog(message: string): boolean {
  for (const { pattern, name } of SENSITIVE_PATTERNS) {
    if (pattern.test(message)) {
      console.warn(`[SECURITY WARNING] Attempted to log message containing potential ${name}. Use secureLog instead.`)
      return false
    }
  }
  return true
}

// Export convenience methods
export const secureLog = {
  debug: (message: unknown, ...args: unknown[]) => log('debug', toSafeString(message), ...args),
  info: (message: unknown, ...args: unknown[]) => log('info', toSafeString(message), ...args),
  warn: (message: unknown, ...args: unknown[]) => log('warn', toSafeString(message), ...args),
  error: (message: unknown, ...args: unknown[]) => log('error', toSafeString(message), ...args),

  // Raw methods for when you explicitly need unmasked output (use with extreme caution)
  raw: {
    debug: (...args: unknown[]) => console.debug(...args),
    info: (...args: unknown[]) => console.info(...args),
    warn: (...args: unknown[]) => console.warn(...args),
    error: (...args: unknown[]) => console.error(...args),
  }
}

export default secureLog
