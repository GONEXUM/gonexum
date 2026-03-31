import React, { useEffect, useRef, useState } from 'react'
import { AppLoadSettings, AppSaveSettings, PreviewNFO, SelectDirectory, ValidateNFOTemplate } from '../../wailsjs/go/main/App'
import type { main } from '../../wailsjs/go/models'
import './SettingsPage.css'

const NFO_VARIABLES = [
  { group: 'TMDB', vars: [
    { name: '.TMDB.Title',    desc: 'Titre' },
    { name: '.TMDB.Year',     desc: 'Année' },
    { name: '.TMDB.Director', desc: 'Réalisateur' },
    { name: '.TMDB.Overview', desc: 'Synopsis' },
    { name: '.TMDB.Genres',   desc: 'Genres (tableau)' },
    { name: '.TMDB.Rating',   desc: 'Note (float)' },
    { name: '.TMDB.Runtime',  desc: 'Durée en minutes' },
    { name: '.TMDB.ID',       desc: 'ID TMDB' },
    { name: '.TMDB.MediaType',desc: 'movie / tv' },
  ]},
  { group: 'Média', vars: [
    { name: '.Media.Resolution',     desc: 'Résolution' },
    { name: '.Media.VideoCodec',     desc: 'Codec vidéo' },
    { name: '.Media.AudioCodec',     desc: 'Codec audio' },
    { name: '.Media.AudioLanguages', desc: 'Langues audio' },
    { name: '.Media.HDRFormat',      desc: 'HDR (vide si absent)' },
    { name: '.Media.Source',         desc: 'Source (BluRay…)' },
    { name: '.Media.Duration',       desc: 'Durée formatée' },
    { name: '.Media.FrameRate',      desc: 'FPS (float)' },
  ]},
]

const NFO_STARTER = `GONEXUM NFO
===========

{{ if .TMDB.Title -}}
Titre:        {{ .TMDB.Title }}
{{ end -}}
{{ if .TMDB.Year -}}
Année:        {{ .TMDB.Year }}
{{ end -}}
{{ if .TMDB.Genres -}}
Genre:        {{ join ", " .TMDB.Genres }}
{{ end -}}
{{ if .TMDB.Director -}}
Réalisateur:  {{ .TMDB.Director }}
{{ end -}}
{{ if .TMDB.Rating -}}
Note:         {{ printf "%.1f/10" .TMDB.Rating }}
{{ end -}}
{{ if .TMDB.Runtime -}}
Durée:        {{ printf "%d min" .TMDB.Runtime }}
{{ end }}
--- TECHNIQUE ---
{{ if .Media.Resolution -}}
Résolution:   {{ .Media.Resolution }}
{{ end -}}
{{ if .Media.VideoCodec -}}
Vidéo:        {{ .Media.VideoCodec }}
{{ end -}}
{{ if .Media.AudioCodec -}}
Audio:        {{ .Media.AudioCodec }}
{{ end -}}
{{ if .Media.AudioLanguages -}}
Langues:      {{ .Media.AudioLanguages }}
{{ end -}}
{{ if .Media.HDRFormat -}}
HDR:          {{ .Media.HDRFormat }}
{{ end -}}
{{ if .Media.Source -}}
Source:       {{ .Media.Source }}
{{ end -}}
{{ if .Media.Duration -}}
Durée:        {{ .Media.Duration }}
{{ end }}
{{ if .TMDB.Overview -}}
--- SYNOPSIS ---
{{ .TMDB.Overview }}
{{- end }}
`

interface SettingsPageProps {
  setupRequired?: boolean
  onSaved?: () => void
}

export default function SettingsPage({ setupRequired, onSaved }: SettingsPageProps = {}) {
  const [settings, setSettings] = useState<main.Settings>({
    apiKey: '',
    passkey: '',
    tmdbApiKey: '',
    trackerUrl: 'https://nexum-core.com',
    outputDir: '',
    nfoTemplate: '',
  })
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState('')
  const [showApiKey, setShowApiKey] = useState(false)
  const [showPasskey, setShowPasskey] = useState(false)
  const [nfoValidError, setNfoValidError] = useState('')
  const [nfoPreview, setNfoPreview] = useState('')
  const [nfoPreviewLoading, setNfoPreviewLoading] = useState(false)
  const nfoValidateTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)

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
      onSaved?.()
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

  const handleNfoTemplateChange = (value: string) => {
    set('nfoTemplate', value)
    setNfoValidError('')
    if (nfoValidateTimeout.current) clearTimeout(nfoValidateTimeout.current)
    if (value.trim() === '') return
    nfoValidateTimeout.current = setTimeout(async () => {
      try {
        await ValidateNFOTemplate(value)
        setNfoValidError('')
      } catch (e) {
        setNfoValidError(String(e))
      }
    }, 500)
  }

  const handleNfoPreview = async () => {
    setNfoPreviewLoading(true)
    setNfoPreview('')
    try {
      const result = await PreviewNFO(settings.nfoTemplate ?? '')
      setNfoPreview(result)
    } catch (e) {
      setNfoPreview(`Erreur : ${String(e)}`)
    }
    setNfoPreviewLoading(false)
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

      {setupRequired && (
        <div className="alert alert-warning" style={{ marginBottom: 16 }}>
          <span>⚙</span>
          <span>Veuillez configurer vos paramètres avant d'utiliser l'application.</span>
        </div>
      )}

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

      <div className="divider" />

      {/* NFO Template section */}
      <section className="settings-section">
        <div className="settings-section-header">
          <div className="settings-section-icon">📄</div>
          <div>
            <h2 className="settings-section-title">Template NFO personnalisé</h2>
            <p className="text-secondary text-sm">
              Laissez vide pour utiliser le template par défaut (encadré ASCII)
            </p>
          </div>
        </div>

        <div className="settings-fields">
          {/* Variables reference */}
          <details className="nfo-vars-details">
            <summary className="nfo-vars-summary">Variables disponibles</summary>
            <div className="nfo-vars-grid">
              {NFO_VARIABLES.map(g => (
                <div key={g.group}>
                  <p className="nfo-vars-group">{g.group}</p>
                  {g.vars.map(v => (
                    <div key={v.name} className="nfo-var-row">
                      <code className="nfo-var-name">{`{{ ${v.name} }}`}</code>
                      <span className="nfo-var-desc">{v.desc}</span>
                    </div>
                  ))}
                </div>
              ))}
            </div>
            <p className="nfo-vars-hint text-muted text-xs">
              Fonctions disponibles&nbsp;:
              <code>padRight "texte" 16</code>,&nbsp;
              <code>center "texte" 60</code>,&nbsp;
              <code>truncate "texte" 40</code>,&nbsp;
              <code>repeat "═" 60</code>,&nbsp;
              <code>join ", " .TMDB.Genres</code>,&nbsp;
              <code>printf "%.1f" .TMDB.Rating</code>
            </p>
          </details>

          {/* Textarea */}
          <div className="form-group">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 6 }}>
              <label className="label" style={{ marginBottom: 0 }}>Template</label>
              <div style={{ display: 'flex', gap: 8 }}>
                <button
                  className="btn btn-secondary btn-sm"
                  type="button"
                  onClick={() => handleNfoTemplateChange(NFO_STARTER)}
                >
                  Charger le modèle de départ
                </button>
                {settings.nfoTemplate && (
                  <button
                    className="btn btn-secondary btn-sm"
                    type="button"
                    onClick={() => { handleNfoTemplateChange(''); setNfoPreview('') }}
                  >
                    Réinitialiser (défaut)
                  </button>
                )}
              </div>
            </div>
            <textarea
              className={`input nfo-textarea${nfoValidError ? ' nfo-textarea--error' : ''}`}
              value={settings.nfoTemplate ?? ''}
              onChange={e => handleNfoTemplateChange(e.target.value)}
              placeholder="Laissez vide pour le template par défaut…"
              spellCheck={false}
            />
            {nfoValidError && (
              <span className="nfo-error text-xs">{nfoValidError}</span>
            )}
          </div>

          {/* Preview */}
          <div>
            <button
              className="btn btn-secondary"
              type="button"
              onClick={handleNfoPreview}
              disabled={nfoPreviewLoading || !!nfoValidError}
            >
              {nfoPreviewLoading ? <><span className="spinner" />Génération...</> : '👁 Prévisualiser'}
            </button>
            {nfoPreview && (
              <pre className="nfo-preview">{nfoPreview}</pre>
            )}
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
