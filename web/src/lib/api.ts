// [FILE] web/src/lib/api.ts
// Helper REST API (bukan SSE).
// Digunakan untuk endpoint lain (timeseries, healthz, dsb).

import axios from "axios";

const api = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || "http://localhost:8080",
  timeout: 15000,
});

export default api;
