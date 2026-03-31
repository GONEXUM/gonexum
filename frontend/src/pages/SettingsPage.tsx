import React, { useEffect, useState } from 'react'
import { AppLoadSettings, AppSaveSettings, SelectDirectory } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'
import './SettingsPage.css'

export default function SettingsPage() {
  const [settings, setSettings] = useState<main.Settings>({
    apiKey: '',
    passkey: '',
    tmdbApiKey: '',
    trackerUrl: 'https://nexum-core.com',
    outputDir: '',
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  const [showApiKey, setShowApiKey] = useState(false)
  const [showPasskey, setShowPasskey] = useState(false)

  useEffect(() => {
    AppLoadSettings()
      .then(s => { setSettings(s); setLoading(false) })
      .catch(e => { setError(String(e)); setLoading(false) })
  }, [])

  const handleSave = async () => {
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      await AppSaveSettings(settings)
      setSaved(true)
      setTimeout(() => setSaved(false), 3000)
    } catch (e) {
      setError(String(e))
    }
    setSaving(false)
  }

  const handlePickOutputDir = async () => {
    try {
      const path = await SelectDirectory('Dossier de sortie des torrents')
      if (path) setSettings(s => ({ ...s, outputDir: path }))
    } catch (e) { setError(String(e)) }
  }

  const set = (key: keyof main.Settings, value: string) => {
    setSettings(s => ({ ...s, [key]: value }))
    setSaved(false)
  }

  if (loading) {
    return (
      <div className="settings-page">
        <div style={{ display: 'flex', justifyContent: 'center', padding: 40 }}>
          <span className="spinner spinner-lg" />
        </div>
      </div>
    )
  }

  return (
    <div className="settings-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Paramètres</h1>
          <p className="text-secondary" style={{ marginTop: 4, fontSize: 13 }}>
            Configurez vos clés d'accès pour le tracker Nexum et l'API TMDB
          </p>
        </div>
      </div>

      {error && (
        <div className="alert alert-error" style={{ marginBottom: 16 }}>
          <span>⚠</span>
          <span>{error}</span>
        </div>
      )}

      {saved && (
        <div className="alert alert-success" style={{ marginBottom: 16 }}>
          <span>✓</span>
          <span>Paramètres sauvegardés avec succès.</span>
        </div>
      )}

      {/* Tracker section */}
      <section className="settings-section">
        <div className="settings-section-header">
          <div className="settings-section-icon">🔑</div>
          <div>
            <h2 className="settings-section-title">Tracker Nexum</h2>
            <p className="text-secondary text-sm">Clés d'accès pour nexum-core.com</p>
          </div>
        </div>

        <div className="settings-fields">
          <div className="form-group">
            <label className="label">URL du Tracker</label>
            <input
              className="input"
              value={settings.trackerUrl}
              onChange={e => set('trackerUrl', e.target.value)}
              placeholder="https://nexum-core.com"
            />
            <span className="field-hint text-muted text-xs">URL de base du tracker (sans /announce)</span>
          </div>

          <div className="form-group">
            <label className="label">
              Clé API
              <span className="label-hint">(Paramètres → Clé API sur le site)</span>
            </label>
            <div className="input-with-toggle">
              <input
                className="input"
                type={showApiKey ? 'text' : 'password'}
                value={settings.apiKey}
                onChange={e => set('apiKey', e.target.value)}
                placeholder="Votre clé API Nexum..."
              />
              <button
                className="toggle-btn"
                onClick={() => setShowApiKey(v => !v)}
                type="button"
              >
                {showApiKey ? '🙈' : '👁'}
              </button>
            </div>
            <span className="field-hint text-muted text-xs">
              Utilisée pour l'upload via <code>?apikey=…</code>
            </span>
          </div>

          <div className="form-group">
            <label className="label">
              Passkey
              <span className="label-hint">(différent de la clé API)</span>
            </label>
            <div className="input-with-toggle">
              <input
                className="input"
                type={showPasskey ? 'text' : 'password'}
                value={settings.passkey}
                onChange={e => set('passkey', e.target.value)}
                placeholder="Votre passkey tracker..."
              />
              <button
                className="toggle-btn"
                onClick={() => setShowPasskey(v => !v)}
                type="button"
              >
                {showPasskey ? '🙈' : '👁'}
              </button>
            </div>
            <span className="field-hint text-muted text-xs">
              Intégré dans l'URL d'announce du fichier .torrent
            </span>
          </div>
        </div>
      </section>

      <div className="divider" />

      {/* Output section */}
      <section className="settings-section">
        <div className="settings-section-header">
          <div className="settings-section-icon">📂</div>
          <div>
            <h2 className="settings-section-title">Dossier de sortie</h2>
            <p className="text-secondary text-sm">Où enregistrer les fichiers .torrent créés</p>
          </div>
        </div>

        <div className="settings-fields">
          <div className="form-group">
            <label className="label">Dossier</label>
            <div style={{ display: 'flex', gap: 8 }}>
              <input
                className="input"
                value={settings.outputDir}
                onChange={e => set('outputDir', e.target.value)}
                placeholder="Dossier temporaire par défaut..."
              />
              <button className="btn btn-secondary" onClick={handlePickOutputDir}>
                Parcourir
              </button>
            </div>
            <span className="field-hint text-muted text-xs">
              Laissez vide pour utiliser le dossier temporaire du système
            </span>
          </div>
        </div>
      </section>

      <div className="settings-save-bar">
        <button
          className="btn btn-primary btn-lg"
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? <><span className="spinner" />Sauvegarde...</> : '💾 Sauvegarder'}
        </button>
      </div>
    </div>
  )
}
