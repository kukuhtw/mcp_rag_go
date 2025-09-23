// web/src/pages/Chat.tsx
import { useEffect, useMemo, useRef, useState } from "react";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, CartesianGrid, ResponsiveContainer, Legend,
} from "recharts";

type Meta = { router?: string; model?: string };

// Bentuk umum item dari event "sources"
type ExecResult = {
  route?: any;
  data?: any;
  Data?: any;
  error?: string;
};

type SeriesPoint = { x: string | number; y: number };
type SeriesBundle = { name: string; points: SeriesPoint[] };

export default function Chat() {
  const [q, setQ] = useState("");
  const [answer, setAnswer] = useState("");
  const [debugLog, setDebugLog] = useState("");
  const [meta, setMeta] = useState<Meta>({});
  const [busy, setBusy] = useState(false);
  const [showDebug, setShowDebug] = useState(true);
  const [vizItems, setVizItems] = useState<ExecResult[]>([]);
  const esRef = useRef<EventSource | null>(null);

  const appendAnswer = (s: string) => setAnswer((p) => p + s);
  const appendDebug = (s: string) => setDebugLog((p) => p + s);

  const closeES = () => {
    try { esRef.current?.close(); } catch {}
    esRef.current = null;
    setBusy(false);
  };

  // Heuristik bahasa
  const detectLang = (text: string): "en" | "id" => {
    const t = text.trim();
    if (!t) return "id";
    const enHints = /\b(the|and|of|for|what|how|why|please|show|compare|top|amount|vendor|status|drilling|timeseries|average|daily)\b/i;
    const letters = (t.match(/[A-Za-z]/g)?.length || 0);
    const asciiish = letters >= Math.max(8, Math.floor(t.length * 0.35));
    return enHints.test(t) || asciiish ? "en" : "id";
  };

  const start = () => {
    if (!q || busy) return;

    setAnswer("");
    setDebugLog("");
    setMeta({});
    setVizItems([]);
    closeES();

    const lang = detectLang(q);
    const apiBase = import.meta.env.VITE_API_BASE || "http://localhost:8080";
    const url = `${apiBase}/chat/stream?q=${encodeURIComponent(q)}&lang=${lang}`;

    appendDebug(`[connecting] ${url}\n`);
    const es = new EventSource(url);
    esRef.current = es;
    setBusy(true);

    es.addEventListener("open", () => appendDebug(`[connected]\n`));

    es.addEventListener("meta", (e: MessageEvent) => {
      appendDebug(`[meta] ${e.data}\n`);
      try { setMeta((m) => ({ ...m, ...JSON.parse(e.data) })); } catch {}
    });

    es.addEventListener("phase", (e: MessageEvent) => appendDebug(`[phase] ${e.data}\n`));

    es.addEventListener("plan", (e: MessageEvent) => {
      try {
        const obj = JSON.parse(e.data);
        if (obj?.model || obj?.router) setMeta((m) => ({ ...m, ...obj }));
      } catch {}
      appendDebug(`[plan] ${e.data}\n`);
    });

    // === sumber data untuk visualisasi ===
    es.addEventListener("sources", (e: MessageEvent) => {
      appendDebug(`[sources] ${e.data}\n`);
      try {
        const arr: ExecResult[] = JSON.parse(e.data);
        setVizItems(arr || []);
      } catch {}
    });

    // === token stream ke jawaban ===
    es.addEventListener("delta", (e: MessageEvent) => {
      try {
        const data = JSON.parse(String(e.data));
        const token =
          data?.choices?.[0]?.delta?.content ??
          data?.delta ??
          data?.content ?? "";
        if (token) { appendAnswer(String(token)); return; }
        appendDebug(`[delta:raw] ${e.data}\n`);
      } catch { appendDebug(`[delta:text] ${e.data}\n`); }
    });

    es.addEventListener("err", (e: MessageEvent) => {
      appendDebug(`[error] ${e.data || "unknown error"}\n`);
      closeES();
    });

    es.addEventListener("message", (e: MessageEvent) => {
      try {
        const obj = JSON.parse(e.data);
        if (obj?.event === "delta") {
          const token =
            obj?.data?.choices?.[0]?.delta?.content ??
            obj?.data?.delta ?? obj?.data?.content ?? obj?.data ?? "";
          if (token) { appendAnswer(String(token)); return; }
        }
        const token2 =
          obj?.choices?.[0]?.delta?.content ?? obj?.delta ?? obj?.content ?? "";
        if (token2) { appendAnswer(String(token2)); return; }
        if (typeof obj === "string") { appendDebug(`[message] ${obj}\n`); return; }
        appendDebug(`[message:raw] ${e.data}\n`);
      } catch { appendDebug(`[message:text] ${e.data}\n`); }
    });

    es.addEventListener("error", () => {
      appendDebug(`[connection error] stream closed\n`);
      closeES();
    });

    es.addEventListener("done", () => {
      appendDebug(`[done]\n`);
      closeES();
    });
  };

  useEffect(() => () => closeES(), []);

  const endpointPreview =
    (import.meta.env.VITE_API_BASE || "http://localhost:8080") + "/chat/stream";

  // ===================== PARSER SOURCES → SERIES =====================
  // Kita cari pola:
  // 1) { tag:"FLOW_C03", series:[{ts_utc|timestamp|time|t, value|v|y}] }
  // 2) { series:[...] } (tanpa tag) → nama diambil dari route/Tool atau "series-#"
  // 3) Array langsung of points → {ts_utc,value,...}
  // ===================== PARSER SOURCES → SERIES =====================
type SeriesPoint = { x: string | number; y: number };
type SeriesBundle = { name: string; points: SeriesPoint[] };

const seriesBundles: SeriesBundle[] = useMemo(() => {
  const bundles: SeriesBundle[] = [];

  const normalizePoint = (p: any): SeriesPoint | null => {
  if (p?.quality === 0) return null; // abaikan data berkualitas 0
  const t = p?.ts_utc ?? p?.timestamp ?? p?.time ?? p?.t ?? p?.x ?? (Array.isArray(p) ? p[0] : undefined);
  const vRaw = p?.value ?? p?.v ?? p?.y ?? (Array.isArray(p) ? p[1] : undefined);
  const v = Number(vRaw);
  if (t === undefined || Number.isNaN(v)) return null;
  return { x: typeof t === "string" ? t : String(t), y: v };
};

  const tryPushSeries = (name: string, arr: any[]) => {
    const points = arr.map(normalizePoint).filter(Boolean) as SeriesPoint[];
    if (points.length) bundles.push({ name, points });
  };

  for (const it of vizItems) {
    const route = it?.route ?? {};
    const params = route?.params ?? route?.Params ?? {};
    const data = (it?.data ?? it?.Data ?? {}) as any;

    const routeName =
      params?.tag ||
      data?.tag ||
      data?.tag_id ||
      route?.tool ||
      route?.Tool ||
      `series-${bundles.length + 1}`;

    // Bentuk 1: { points: [...] }  ← INI YANG ADA DI PAYLOAD KAMU
    if (Array.isArray(data?.points)) {
      tryPushSeries(String(routeName), data.points);
      continue;
    }

    // Bentuk 2: { tag/name, series: [...] }
    if ((data?.tag || data?.name) && Array.isArray(data?.series)) {
      tryPushSeries(String(data.tag || data.name), data.series);
      continue;
    }

    // Bentuk 3: series bersarang di result/data
    const seriesCandidates = [
      data?.series, data?.Series,
      data?.result?.series, data?.result?.Series,
      data?.data?.series, data?.data?.Series,
      // tambahkan kemungkinan "points" di dalam result/data juga
      data?.result?.points, data?.data?.points,
    ].filter(Array.isArray) as any[][];
    if (seriesCandidates.length) {
      seriesCandidates.forEach((arr, idx) =>
        tryPushSeries(
          `${routeName}${seriesCandidates.length > 1 ? ` #${idx + 1}` : ""}`,
          arr
        )
      );
      continue;
    }

    // Bentuk 4: array of points langsung
    if (Array.isArray(data) && data.length && typeof data[0] === "object") {
      tryPushSeries(String(routeName), data);
    }
  }

  return bundles;
}, [vizItems]);


  // Data tabular ringan (opsional)
  const parsedTables = useMemo(() => {
    const out: { title: string; rows: Record<string, any>[] }[] = [];
    for (const it of vizItems) {
      const title = (it?.route?.tool || it?.route?.Tool || "").toString().toLowerCase() || "table";
      const data = (it?.data ?? it?.Data ?? {}) as any;
      const cand = data?.items || data?.rows || data?.result?.items || data?.retrieved_chunks;
      if (Array.isArray(cand) && cand.length && typeof cand[0] === "object") {
        out.push({ title, rows: cand });
      }
    }
    return out;
  }, [vizItems]);

  // Gabungkan semua series ke satu dataset untuk LineChart stacked (key per series)
  const mergedForRecharts = useMemo(() => {
    if (!seriesBundles.length) return [];
    // indeks by x
    const byX = new Map<string | number, any>();
    for (const s of seriesBundles) {
      for (const p of s.points) {
        const k = p.x;
        if (!byX.has(k)) byX.set(k, { x: k });
        byX.get(k)[s.name] = p.y;
      }
    }
    // sort by x jika bisa di-parse tanggal
    const arr = Array.from(byX.values());
    const isDate = typeof arr[0]?.x === "string" && /\d{4}-\d{2}-\d{2}T/.test(arr[0].x);
    if (isDate) {
      arr.sort((a, b) => (new Date(a.x).getTime() - new Date(b.x).getTime()));
    }
    return arr;
  }, [seriesBundles]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">MCP+RAG Chat Demo</h2>
        {busy ? <span className="text-xs text-slate-500">streaming…</span> : null}
      </div>

      {/* INPUT */}
      <div className="flex gap-2">
        <textarea
          className="border rounded px-3 py-2 flex-1 min-h-24 resize-y"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Tanya sesuatu… / Ask something… (Ctrl/⌘+Enter)"
          onKeyDown={(e) => {
            const isMac = navigator.platform.toLowerCase().includes("mac");
            if ((isMac ? e.metaKey : e.ctrlKey) && e.key === "Enter") {
              e.preventDefault();
              start();
            }
          }}
        />
        <div className="flex flex-col gap-2">
          <button
            onClick={start}
            disabled={!q || busy}
            className="px-4 py-2 rounded bg-slate-900 text-white disabled:opacity-50"
          >
            Send
          </button>
          {busy && (
            <button
              onClick={closeES}
              className="px-3 py-2 rounded border"
              title="Stop streaming"
            >
              Stop
            </button>
          )}
          <button
            onClick={() => setShowDebug((v) => !v)}
            className="px-3 py-2 rounded border"
            title="Toggle debug panel"
          >
            {showDebug ? "Hide Debug" : "Show Debug"}
          </button>
        </div>
      </div>

      <div className="text-xs text-slate-500">
        Endpoint: {(import.meta.env.VITE_API_BASE || "http://localhost:8080") + "/chat/stream"}
        {meta?.router && (
          <>
            {" · "}Router: <b>{meta.router}</b>
            {meta.model ? <> · Model: <b>{meta.model}</b></> : null}
          </>
        )}
      </div>

      {/* RESPONSE & DEBUG */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div>
          <div className="mb-2 text-sm text-slate-600">Response</div>
          <textarea
            readOnly
            className="w-full min-h-40 p-4 bg-white border rounded whitespace-pre-wrap font-mono text-sm"
            value={answer || "..."}
          />
        </div>
        {showDebug && (
          <div>
            <div className="mb-2 text-sm text-slate-600">Debug / Tracking</div>
            <textarea
              readOnly
              className="w-full min-h-40 p-4 bg-white border rounded whitespace-pre-wrap font-mono text-xs text-slate-700"
              value={debugLog || "[no logs yet]"}
            />
          </div>
        )}
      </div>

      {/* ===== Visualization ===== */}
      {(seriesBundles.length > 0 || parsedTables.length > 0) && (
        <div className="space-y-6">
          <h3 className="text-base font-semibold">Data Visualization</h3>

          {/* Multi-series LineChart */}
          {seriesBundles.length > 0 && (
            <div className="w-full h-72 border rounded p-3">
              <div className="text-sm mb-2 text-slate-600">
                {seriesBundles.map(s => s.name).join(", ")}
              </div>
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={mergedForRecharts} margin={{ top: 10, right: 20, bottom: 10, left: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis dataKey="x" />
                  <YAxis />
                  <Tooltip />
                  <Legend />
                  {seriesBundles.map((s) => (
                    <Line
                      key={s.name}
                      type="monotone"
                      dataKey={s.name}
                      dot={false}
                      isAnimationActive={false}
                    />
                  ))}
                </LineChart>
              </ResponsiveContainer>
            </div>
          )}

          {/* Simple tables (opsional) */}
          {parsedTables.map((tbl, idx) => {
            const cols = Object.keys(tbl.rows[0] ?? {});
            return (
              <div key={`tb-${idx}`} className="border rounded p-3 overflow-auto">
                <div className="text-sm mb-2 text-slate-600">{tbl.title}</div>
                <table className="min-w-full text-sm">
                  <thead>
                    <tr className="text-left border-b">
                      {cols.map((c) => <th key={c} className="py-1 pr-4">{c}</th>)}
                    </tr>
                  </thead>
                  <tbody>
                    {tbl.rows.slice(0, 200).map((row, rIdx) => (
                      <tr key={rIdx} className="border-b last:border-0">
                        {cols.map((c) => (
                          <td key={c} className="py-1 pr-4">
                            {String(row?.[c] ?? "")}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
                {tbl.rows.length > 200 && (
                  <div className="text-xs text-slate-500 mt-2">Showing first 200 rows…</div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
