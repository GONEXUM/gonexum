import React from 'react'
import { NavLink, useLocation } from 'react-router-dom'
import { AppLoadSettings, CheckUpdate, GetAppVersion } from '../../wailsjs/go/main/App'
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
  const [configured, setConfigured] = React.useState<boolean>(true)
  const [update, setUpdate] = React.useState<{ version: string; url: string } | null>(null)
  const [appVersion, setAppVersion] = React.useState('')

  React.useEffect(() => {
    AppLoadSettings()
      .then(s => setConfigured(!!(s.trackerUrl?.trim() && s.apiKey?.trim() && s.passkey?.trim())))
      .catch(() => setConfigured(false))
  }, [location.pathname])

  React.useEffect(() => {
    GetAppVersion().then(v => setAppVersion(v)).catch(() => {})
    CheckUpdate()
      .then(info => { if (info.available) setUpdate({ version: info.latest, url: info.url }) })
      .catch(() => {})
  }, []) // re-check after each navigation (e.g. after saving settings)

  return (
    <div className="layout">
      <aside className="sidebar">
        <div className="sidebar-logo">
          <span className="logo-text">NEXUM</span>
        </div>

        <nav className="sidebar-nav">
          <div className="nav-section-label">NAVIGATION</div>
          {NAV_ITEMS.map((item) => {
            const locked = !configured && item.to === '/'
            return locked ? (
              <span
                key={item.to}
                className="nav-item nav-item-locked"
                title="Configurez d'abord vos paramètres"
              >
                <span className="nav-arrow">▶</span>
                <span>{item.label}</span>
                <span className="nav-lock">🔒</span>
              </span>
            ) : (
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
            )
          })}
        </nav>

        <div className="sidebar-footer">
          <span className="text-muted text-xs">GONEXUM {appVersion}</span>
          {update && (
            <a
              href={update.url}
              target="_blank"
              rel="noopener noreferrer"
              className="update-badge"
              title={`v${update.version} disponible`}
            >
              ↑ v{update.version}
            </a>
          )}
        </div>
      </aside>

      <main className="main-content">
        {children}
      </main>
    </div>
  )
}
