// web/src/components/Sidebar.tsx
import { NavLink } from 'react-router-dom'
import { appRoutes, MenuItem } from '../routes'

function Item({ item }: { item: MenuItem }) {
  if (item.children?.length) {
    return (
      <div className="space-y-1">
        <div className="text-[13px] uppercase tracking-wide text-slate-500 mt-4">{item.label}</div>
        <div className="pl-2">
          {item.children.map(c => <Item key={c.key} item={c} />)}
        </div>
      </div>
    )
  }
  if (!item.path) return null
  return (
    <NavLink
      to={item.path}
      className={({isActive}) => `block px-3 py-2 rounded hover:bg-slate-100 ${isActive?'bg-slate-900 text-white hover:bg-slate-900':''}`}
      end
    >
      {item.label}
    </NavLink>
  )
}

export default function Sidebar() {
  return (
    <aside className="w-60 shrink-0 border-r bg-white p-3 h-[calc(100vh-57px)] sticky top-[57px] overflow-auto">
      {appRoutes.filter(r => r.key!=='login').map(r => <Item key={r.key} item={r} />)}
    </aside>
  )
}
