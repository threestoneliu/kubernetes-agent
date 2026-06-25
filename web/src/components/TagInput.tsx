import React from 'react'

interface TagInputProps {
  label: string
  tags: string[]
  onChange: (tags: string[]) => void
  placeholder?: string
}

export function TagInput({ label, tags, onChange, placeholder = '' }: TagInputProps) {
  const [input, setInput] = React.useState('')

  function addTag() {
    const trimmed = input.trim()
    if (!trimmed) return
    if (tags.includes(trimmed)) { setInput(''); return }
    onChange([...tags, trimmed])
    setInput('')
  }

  function removeTag(index: number) {
    onChange(tags.filter((_, i) => i !== index))
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') { e.preventDefault(); addTag() }
  }

  return (
    <label style={{ display: 'block', marginBottom: 12 }}>
      <span className="muted" style={{ fontSize: 12 }}>{label}</span>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 4, marginTop: 4, alignItems: 'center' }}>
        {tags.map((tag, i) => (
          <span key={i} style={{ display: 'inline-flex', alignItems: 'center', gap: 2, background: 'var(--accent)', color: '#fff', borderRadius: 4, padding: '2px 6px', fontSize: 12 }}>
            {tag}
            <button
              type="button"
              onClick={() => removeTag(i)}
              style={{ background: 'none', border: 'none', color: 'inherit', cursor: 'pointer', padding: 0, lineHeight: 1, fontSize: 14 }}
            >×</button>
          </span>
        ))}
        <input
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={tags.length === 0 ? placeholder : ''}
          style={{ border: 'none', outline: 'none', background: 'transparent', fontSize: 12, minWidth: 80 }}
        />
      </div>
    </label>
  )
}
