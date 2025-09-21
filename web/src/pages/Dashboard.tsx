import { useQuery } from '@tanstack/react-query'
import { health } from '../lib/api'
export default function Dashboard(){
  const { data, isLoading } = useQuery({ queryKey:['health'], queryFn: health })
  return (<div className="space-y-4">
    <h2 className="text-lg font-medium">Status</h2>
    <pre className="p-4 bg-white rounded-lg border">{isLoading ? 'Loading...' : JSON.stringify(data, null, 2)}</pre>
    <p className="text-sm text-slate-600">Gunakan tab <b>Timeseries</b> untuk mengambil data dari endpoint <code>/timeseries</code>.</p>
  </div>)
}
