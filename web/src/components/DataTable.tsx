type Col<T> = { key: keyof T; header: string }
export default function DataTable<T extends Record<string, any>>({ rows, cols }: { rows: T[]; cols: Col<T>[] }){
  return (<div className="overflow-x-auto rounded-lg border bg-white">
    <table className="min-w-full text-sm"><thead className="bg-slate-50"><tr>
      {cols.map(c => <th key={String(c.key)} className="px-3 py-2 text-left font-medium text-slate-600">{c.header}</th>)}
    </tr></thead><tbody>
      {rows.map((r,i)=>(<tr key={i} className="border-t">
        {cols.map(c => <td key={String(c.key)} className="px-3 py-2">{String(r[c.key] ?? '')}</td>)}
      </tr>))}
    </tbody></table>
  </div>)
}
