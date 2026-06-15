// The chat endpoint requires POST + JSON body + text/event-stream response,
// which EventSource (GET-only, no body, no custom headers) cannot do. We
// drive fetch ourselves and parse the SSE frame wire format manually.

export type SseEvent = {
  id: string
  type: string
  payload: unknown
}

export type OpenChatSseOptions = {
  body: { session_id: string; message: string; cluster_id: string }
  onEvent: (e: SseEvent) => void
  onError: (err: Error) => void
  onClose: () => void
  lastEventId?: string
}

export function openChatSse(opts: OpenChatSseOptions): () => void {
  const controller = new AbortController()

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'text/event-stream',
  }
  // Server reads Last-Event-ID as advisory; pass through if the caller
  // remembers the last frame id from a previous connection.
  if (opts.lastEventId) {
    headers['Last-Event-ID'] = opts.lastEventId
  }

  fetch('/api/chat', {
    method: 'POST',
    headers,
    body: JSON.stringify(opts.body),
    signal: controller.signal,
  })
    .then(async (res) => {
      if (!res.ok || !res.body) {
        const txt = await res.text().catch(() => '')
        opts.onError(new Error(`HTTP ${res.status}: ${txt || res.statusText}`))
        opts.onClose()
        return
      }
      const reader = res.body.getReader()
      const decoder = new TextDecoder('utf-8')
      let buf = ''
      // eslint-disable-next-line no-constant-condition
      while (true) {
        const { value, done } = await reader.read()
        if (done) break
        buf += decoder.decode(value, { stream: true })
        let idx
        while ((idx = buf.indexOf('\n\n')) !== -1) {
          const frame = buf.slice(0, idx)
          buf = buf.slice(idx + 2)
          const ev = parseFrame(frame)
          if (ev) opts.onEvent(ev)
        }
      }
      // Drain any trailing buffered frame that lacked a final newline.
      if (buf.trim().length > 0) {
        const ev = parseFrame(buf)
        if (ev) opts.onEvent(ev)
      }
      opts.onClose()
    })
    .catch((err: unknown) => {
      // AbortError fires when the caller invokes the returned close() — not
      // a real error, swallow it.
      if (err instanceof DOMException && err.name === 'AbortError') {
        opts.onClose()
        return
      }
      const e = err instanceof Error ? err : new Error(String(err))
      opts.onError(e)
      opts.onClose()
    })

  return () => controller.abort()
}

function parseFrame(frame: string): SseEvent | null {
  let id = ''
  let type = ''
  let data = ''
  for (const line of frame.split('\n')) {
    if (line.startsWith('id:')) {
      id = line.slice(3).trim()
    } else if (line.startsWith('event:')) {
      type = line.slice(6).trim()
    } else if (line.startsWith('data:')) {
      // SSE allows multi-line data: concatenation is space-joined by spec;
      // we append verbatim because our payloads are JSON and contain no
      // newlines of their own.
      data += line.slice(5).trim()
    }
    // Comment lines (starting with ':') and unknown fields are ignored.
  }
  if (!type) return null
  let payload: unknown = null
  if (data) {
    try {
      payload = JSON.parse(data)
    } catch {
      payload = data
    }
  }
  return { id, type, payload }
}