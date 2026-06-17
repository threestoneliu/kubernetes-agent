import { marked } from 'marked'
import DOMPurify from 'dompurify'

// Markdown renders an LLM-emitted markdown string as sanitised
// HTML. We pull in marked for parsing and DOMPurify for the XSS
// guard so the backend's streamed text can include headings,
// tables, code blocks, etc. without us trusting the source.
//
// The parse + sanitise pair is the simplest safe-enough pipeline
// for the MVP: marked handles GFM (the format the LLM emits), and
// DOMPurify strips <script>, on-* handlers, javascript: URLs, etc.
export function Markdown({ source }: { source: string }) {
  // marked.parse can run synchronously when given a string and no
  // async extensions; we keep the API call shape defensive in case
  // marked v18 returns a Promise in some builds.
  const raw = marked.parse(source, { async: false }) as string
  const clean = DOMPurify.sanitize(raw, {
    USE_PROFILES: { html: true },
    // Allow class attributes so syntax-highlight libraries can
    // hook in later; otherwise default to safe defaults.
    ADD_ATTR: ['class', 'target'],
  })
  return <div className="md" dangerouslySetInnerHTML={{ __html: clean }} />
}