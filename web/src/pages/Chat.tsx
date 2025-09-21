// web/src/pages/Chat.tsx
import { useEffect, useRef, useState } from 'react'

type Meta = { router?: string; model?: string }

export default function Chat() {
  const [q, setQ] = useState('')
  const [answer, setAnswer] = useState('')
  const [meta, setMeta] = useState<Meta>({})
  const esRef = useRef<EventSource | null>(null)

  const start = () => {
    setAnswer('')
    setMeta({})
    if (esRef.current) { esRef.current.close() }
    const api = (import.meta.env.VITE_API_BASE || 'http://localhost:8080')
    const es = new EventSource(`${api}/chat/stream?q=${encodeURIComponent(q)}`)
    esRef.current = es

    es.addEventListener('meta', (e: any) => {
      try { setMeta(JSON.parse(e.data)) } catch {}
    })
    es.addEventListener('delta', (e: any) => {
      // OpenAI stream format -> forward apa adanya (akan ada baris "data: ...")
      // Ambil hanya token delta jika format JSON {"choices":[{"delta":{"content":"..."}}]}
      try {
        const data = JSON.parse(e.data.replace(/^data:\s*/,''))
        const token = data?.choices?.[0]?.delta?.content ?? ''
        if (token) setAnswer(prev => prev + token)
      } catch {
        // fallback: append raw
        setAnswer(prev => prev + (e.data || ''))
      }
    })
    es.addEventListener('error', () => {
      es.close()
    })
    es.addEventListener('done', () => {
      es.close()
    })
  }

  useEffect(() => () => { esRef.current?.close() }, [])

  return (
    <div className="space-y-4">
      <h2 className="text-lg font-medium">Public Chat</h2>
      <div className="flex gap-2">
        <input className="border rounded px-3 py-2 flex-1" value={q} onChange={e=>setQ(e.target.value)} placeholder="Ask something..." />
        <button onClick={start} className="px-4 py-2 rounded bg-slate-900 text-white">Send</button>
      </div>
      {meta?.router && (
        <div className="text-sm text-slate-600">
          Router: <b>{meta.router}</b>{meta.model ? <> · Model: <b>{meta.model}</b></> : null}
        </div>
      )}
      <div className="min-h-40 p-4 bg-white border rounded whitespace-pre-wrap">{answer || '...'}</div>
    </div>
  )
}
