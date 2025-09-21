// web/src/pages/Login.tsx
import { useState } from 'react'

export default function Login() {
  const [u, setU] = useState('')
  const [p, setP] = useState('')
  const [msg, setMsg] = useState('')

  const submit = async () => {
    try {
      const res = await fetch(`${import.meta.env.VITE_API_BASE || 'http://localhost:8080'}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: u, password: p }),
      })
      if (!res.ok) {
        setMsg('Invalid credentials')
        return
      }
      const data = await res.json()
      localStorage.setItem('admintoken', data.token)
      setMsg('Login success. Redirecting...')
      window.setTimeout(()=>{ window.location.reload() }, 800)
    } catch (e: any) {
      setMsg('Network error')
    }
  }

  return (
    <div className="p-8 max-w-sm mx-auto space-y-3">
      <h2 className="text-lg font-medium">Admin Login</h2>
      <input className="border px-3 py-2 w-full rounded" placeholder="Username"
             value={u} onChange={e=>setU(e.target.value)} />
      <input className="border px-3 py-2 w-full rounded" placeholder="Password" type="password"
             value={p} onChange={e=>setP(e.target.value)} />
      <button className="px-4 py-2 bg-slate-900 text-white rounded w-full" onClick={submit}>
        Login
      </button>
      {msg && <div className="text-sm text-slate-600">{msg}</div>}
    </div>
  )
}
