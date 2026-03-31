import React from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import './Layout.css'

const NAV_ITEMS = [
  { to: '/', label: 'Uploader', icon: '↑' },
  { to: '/settings', label: 'Paramètres', icon: '⚙' },
]

interface LayoutProps {
  children: React.ReactNode
}

export default function Layout({ children }: LayoutProps) {
  const location = useLocation()

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-logo">
          <span className="logo-text">NEXUM</span>
        </div>

        <nav className="sidebar-nav">
          <div className="nav-section-label">NAVIGATION</div>
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              className={({ isActive }) =>
                'nav-item' + (isActive ? ' nav-item-active' : '')
              }
            >
              <span className="nav-arrow">▶</span>
              <span>{item.label}</span>
            </NavLink>
          ))}
        </nav>

        <div className="sidebar-footer">
          <span className="text-muted text-xs">GONEXUM v1.0</span>
        </div>
      </aside>

      <main className="main-content">
        {children}
      </main>
    </div>
  )
}
