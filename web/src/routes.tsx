// [FILE] web/src/routes.tsx
import { ReactNode } from "react";
import Login from "./pages/Login";
import Chat from "./pages/Chat";
import Timeseries from "./pages/Timeseries";
import Dashboard from "./pages/Dashboard";
import Admin from "./pages/Admin";

export type MenuItem = {
  key: string;
  label: string;
  path?: string;
  element?: ReactNode;
  children?: MenuItem[];
  requiresAuth?: boolean;
};

export const appRoutes: MenuItem[] = [
  { key: "login", label: "Login", path: "/login", element: <Login /> },
  // Samakan dengan App.tsx → Chat berada di /chat
  { key: "chat", label: "Chat", path: "/chat", element: <Chat /> },
  { key: "timeseries", label: "Timeseries", path: "/timeseries", element: <Timeseries /> },
  { key: "dashboard", label: "Dashboard", path: "/dashboard", element: <Dashboard /> },
  {
    key: "admin",
    label: "Admin",
    requiresAuth: true,
    children: [
      { key: "admin.docs", label: "Documents", path: "/admin/docs", element: <Admin /> },
      { key: "admin.upload", label: "Upload Document", path: "/admin/upload", element: <Admin section="upload" /> },
      { key: "admin.raw", label: "Raw Timeseries", path: "/admin/timeseries", element: <Admin section="raw" /> },
    ],
  },
];
