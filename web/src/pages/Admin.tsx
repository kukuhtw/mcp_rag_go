// web/src/pages/Admin.tsx (ubah signature & render sesuai section)
import { useEffect, useState } from 'react'
import { getTimeseries } from '../lib/api'

export default function Admin({ section }: { section?: 'upload'|'raw' }) {
  const [tag, setTag] = useState('FLOW_A12')
  const [rows, setRows] = useState<any[]>([])
  const [file, setFile] = useState<File | null>(null)
  const [docs, setDocs] = useState<{filename:string,size:number}[]>([])
  const api = (import.meta.env.VITE_API_BASE || 'http://localhost:8080')
  const token = localStorage.getItem('admintoken') || ''

  const authorizedFetch = (input: RequestInfo, init?: RequestInit) =>
    fetch(input, { ...(init||{}), headers: { ...(init?.headers||{}), Authorization: `Bearer ${token}` } })

  const listDocs = async () => {
    const res = await authorizedFetch(`${api}/admin/docs`)
    if (res.ok) {
      const j = await res.json()
      setDocs(j.docs || [])
    }
  }

  const upload = async () => {
    if (!file) return
    const fd = new FormData()
    fd.append('file', file)
    const res = await authorizedFetch(`${api}/admin/docs/upload`, { method: 'POST', body: fd })
    alert(await res.text())
    listDocs()
  }

  const fetchTS = async () => {
    const data = await getTimeseries(tag)
    setRows(data.points || [])
  }

  useEffect(()=>{ listDocs() }, [])

  if (!token) {
    return <div className="p-6 text-sm text-red-600">Not authenticated. Please go to <b>Login</b>.</div>
  }

  return (
    <div className="space-y-8">
      {(section === undefined || section === 'raw') && (
        <section className="space-y-2">
          <h2 className="text-lg font-medium">Raw Timeseries</h2>
          <div className="flex gap-2">
            <input className="border rounded px-3 py-2" value={tag} onChange={e=>setTag(e.target.value)} />
            <button onClick={fetchTS} className="px-3 py-2 bg-slate-900 text-white rounded">Load</button>
          </div>
          <div className="overflow-x-auto rounded border bg-white">
            <table className="min-w-full text-sm">
              <thead><tr><th className="px-3 py-2 text-left">ts_utc</th><th className="px-3 py-2 text-left">value</th><th className="px-3 py-2 text-left">quality</th></tr></thead>
              <tbody>
                {rows.map((r,i)=>(
                  <tr key={i} className="border-t">
                    <td className="px-3 py-2">{r.ts_utc}</td>
                    <td className="px-3 py-2">{r.value}</td>
                    <td className="px-3 py-2">{r.quality}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {(section === undefined || section === 'upload') && (
        <section className="space-y-2">
          <h2 className="text-lg font-medium">Upload Document (vectorize)</h2>
          <input type="file" onChange={e=>setFile(e.target.files?.[0] || null)} />
          <button onClick={upload} className="px-3 py-2 bg-slate-900 text-white rounded disabled:opacity-50" disabled={!file}>Upload</button>
          <div className="text-sm text-slate-600">Docs tersimpan di folder <code>uploads/</code>.</div>
        </section>
      )}

      <section className="space-y-2">
        <h2 className="text-lg font-medium">Uploaded Docs</h2>
        <div className="overflow-x-auto rounded border bg-white">
          <table className="min-w-full text-sm">
            <thead><tr><th className="px-3 py-2 text-left">filename</th><th className="px-3 py-2 text-left">size</th></tr></thead>
            <tbody>
              {docs.map((d,i)=>(
                <tr key={i} className="border-t">
                  <td className="px-3 py-2">{d.filename}</td>
                  <td className="px-3 py-2">{d.size}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  )
}
