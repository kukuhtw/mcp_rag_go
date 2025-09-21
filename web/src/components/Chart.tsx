import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, CartesianGrid } from 'recharts'
export default function Chart({ data }: { data: { ts_utc: string; value: number }[] }){
  return (<div className="h-72 w-full rounded-lg border bg-white p-3">
    <ResponsiveContainer><LineChart data={data}>
      <CartesianGrid strokeDasharray="3 3" /><XAxis dataKey="ts_utc" tick={{ fontSize: 12 }} minTickGap={24} />
      <YAxis tick={{ fontSize: 12 }} /><Tooltip /><Line type="monotone" dataKey="value" dot={false} strokeWidth={2} />
    </LineChart></ResponsiveContainer>
  </div>)
}
