import { useEffect, useState } from 'react'
import { BrowserRouter, Routes, Route, useNavigate, useLocation } from 'react-router-dom'
import Layout from './components/Layout'
import UploadPage from './pages/UploadPage'
import SettingsPage from './pages/SettingsPage'
import HistoryPage from './pages/HistoryPage'
import { UploadQueueProvider } from './contexts/UploadQueueContext'
import { AppLoadSettings } from '../wailsjs/go/main/App'
import './style.css'

function AppRoutes() {
  const [configured, setConfigured] = useState<boolean | null>(null)
  const navigate = useNavigate()
  const location = useLocation()

  const checkSettings = () => {
    AppLoadSettings()
      .then(s => {
        const ok = !!(s.trackerUrl?.trim() && s.apiKey?.trim() && s.passkey?.trim())
        setConfigured(ok)
        if (!ok && location.pathname !== '/settings') {
          navigate('/settings', { replace: true })
        }
      })
      .catch(() => {
        setConfigured(false)
        navigate('/settings', { replace: true })
      })
  }

  useEffect(() => { checkSettings() }, [])

  if (configured === null) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%', padding: 40 }}>
        <span className="spinner spinner-lg" />
      </div>
    )
  }

  return (
    <>
      {!configured && location.pathname !== '/settings' && null}
      <Routes>
        <Route path="/" element={
          configured
            ? <UploadPage />
            : null
        } />
        <Route path="/history" element={configured ? <HistoryPage /> : null} />
        <Route path="/settings" element={
          <SettingsPage
            setupRequired={!configured}
            onSaved={checkSettings}
          />
        } />
      </Routes>
    </>
  )
}

function App() {
  return (
    <BrowserRouter>
      <UploadQueueProvider>
        <Layout>
          <AppRoutes />
        </Layout>
      </UploadQueueProvider>
    </BrowserRouter>
  )
}

export default App
