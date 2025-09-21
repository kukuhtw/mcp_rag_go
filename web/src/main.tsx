// web/src/main.tsx (gantikan seluruh isi)
import React from 'react'
import ReactDOM from 'react-dom/client'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import './styles/index.css'
import Layout from './components/Layout'
import { appRoutes } from './routes'

const routes = [
  // public login route terpisah tanpa layout
  {
    path: '/login',
    element: <Layout><div className="max-w-sm mx-auto w-full"><></></div></Layout>,
    children: []
  },
  // app routes di bawah layout
  {
    path: '/',
    element: <Layout><div id="outlet" /></Layout>,
    children: appRoutes
      .filter(r => r.key !== 'login')
      .flatMap(r => r.children?.length ? r.children : [r])
      .map(r => ({ path: r.path!.replace(/^\//,''), element: r.element }))
  }
]

// Override untuk /login agar benar-benar render login page full
routes[0].element = React.createElement(Layout, null, appRoutes.find(r=>r.key==='login')!.element)

const router = createBrowserRouter(routes)
ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode><RouterProvider router={router} /></React.StrictMode>
)
