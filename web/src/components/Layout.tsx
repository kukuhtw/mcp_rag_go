// web/src/components/Layout.tsx
import { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import Sidebar from './Sidebar'

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen">
      <nav className="sticky top-0 bg-white border-b z-10">
        <div className="max-w-6xl mx-auto px-4 py-3 flex items-center gap-3">
          <Link to="/" className="text-xl font-semibold">MCP Oil & Gas</Link>
          <div className="ml-auto text-sm text-slate-500">
            API: {import.meta.env.VITE_API_BASE || 'http://localhost:8080'}
          </div>
        </div>
      </nav>
      <div className="max-w-6xl mx-auto px-4 flex">
        <Sidebar />
        <main className="flex-1 py-6 pl-6">{children}</main>
      </div>
    </div>
  )
}
