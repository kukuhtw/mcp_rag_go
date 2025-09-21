import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getTimeseries } from '../lib/api'
import DataTable from '../components/DataTable'
import Chart from '../components/Chart'
export default function Timeseries(){
  const [tag, setTag] = useState('FLOW_A12')
  const [limit, setLimit] = useState(100)
  const { data, isFetching, refetch } = useQuery({ queryKey:['timeseries', tag, limit], queryFn: () => getTimeseries(tag, undefined, undefined, limit) })
  const rows = (data?.points ?? []).map(p => ({ ts_utc: new Date(p.ts_utc).toISOString(), value: p.value, quality: p.quality }))
  return (<div className="space-y-4">
    <h2 className="text-lg font-medium">Timeseries</h2>
    <div className="flex flex-wrap items-end gap-3">
      <div><label className="block text-xs text-slate-600">tag_id</label><input value={tag} onChange={e=>setTag(e.target.value)} className="px-3 py-2 border rounded-lg" /></div>
      <div><label className="block text-xs text-slate-600">limit</label><input type="number" value={limit} onChange={e=>setLimit(parseInt(e.target.value)||0)} className="px-3 py-2 border rounded-lg w-28" /></div>
      <button onClick={()=>refetch()} className="px-4 py-2 rounded-lg bg-slate-900 text-white disabled:opacity-50" disabled={isFetching}>{isFetching ? 'Loading...' : 'Fetch'}</button>
    </div>
    <Chart data={rows} />
    <DataTable rows={rows} cols={[{ key: 'ts_utc', header: 'Timestamp (UTC)' },{ key: 'value', header: 'Value' },{ key: 'quality', header: 'Quality' }]} />
  </div>)
}
