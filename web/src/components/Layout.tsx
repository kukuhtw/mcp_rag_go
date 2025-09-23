// web/src/components/Layout.tsx
import { Link, Outlet, useLocation } from "react-router-dom";

export default function Layout() {
  const loc = useLocation();
  const isActive = (p: string) =>
    loc.pathname === p ? "text-slate-900 font-semibold" : "text-slate-600";

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b bg-white">
        <div className="max-w-4xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="font-bold">MCP+RAG Chat Demo</div>
          <nav className="flex gap-4 text-sm">
            <Link className={isActive("/chat")} to="/chat">Chat</Link>
          </nav>
        </div>
      </header>

      <main className="max-w-4xl mx-auto px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
