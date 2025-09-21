import axios from 'axios'
const api = axios.create({ baseURL: import.meta.env.VITE_API_BASE || 'http://localhost:8080', timeout: 10000 })
export type TSPoint = { ts_utc: string; value: number; quality: number }
export type TSResponse = { tag_id: string; points: TSPoint[]; count: number }
export async function getTimeseries(tag_id: string, start?: string, end?: string, limit = 200) {
  const params: any = { tag_id, limit }; if (start) params.start = start; if (end) params.end = end
  const { data } = await api.get<TSResponse>('/timeseries', { params }); return data
}
export async function health(){ const { data } = await api.get('/healthz'); return data }
export default api
