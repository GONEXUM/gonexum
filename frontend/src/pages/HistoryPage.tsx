import { useEffect, useState, useRef } from 'react'
import { ListHistory, DeleteHistoryEntry, ClearHistory } from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import type { main } from '../../wailsjs/go/models'
import './HistoryPage.css'

function formatBytes(b: number): string {
  if (!b) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0, f = b
  while (f >= k && i < sizes.length - 1) { f /= k; i++ }
  return f.toFixed(2) + ' ' + sizes[i]
}

function formatDate(s: string): string {
  try {
    const d = new Date(s)
    return d.toLocaleString('fr-FR', {
      day: '2-digit', month: '2-digit', year: 'numeric',
      hour: '2-digit', minute: '2-digit'
    })
  } catch { return s }
}

export default function HistoryPage() {
  const [items, setItems] = useState<main.HistoryEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const searchTimer = useRef<number | null>(null)

  const load = async (q: string = search) => {
    setLoading(true)
    setError('')
    try {
      const res = await ListHistory(200, 0, q)
      setItems(res || [])
    } catch (e: any) {
      setError(String(e.message || e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load('') }, [])

  const onSearchChange = (v: string) => {
    setSearch(v)
    if (searchTimer.current) window.clearTimeout(searchTimer.current)
    searchTimer.current = window.setTimeout(() => load(v), 300)
  }

  const onDelete = async (id: number) => {
    try {
      await DeleteHistoryEntry(id)
      setItems(items.filter(i => i.id !== id))
    } catch (e: any) {
      alert('Erreur : ' + (e.message || e))
    }
  }

  const onClearAll = async () => {
    if (!confirm('Effacer TOUT l\'historique ? Cette action est irréversible.')) return
    try {
      await ClearHistory()
      setItems([])
    } catch (e: any) {
      alert('Erreur : ' + (e.message || e))
    }
  }

  return (
    <div className="history-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Historique</h1>
          <p className="page-subtitle">Tous les uploads récents depuis cet appareil</p>
        </div>
        <button className="btn btn-ghost" onClick={onClearAll} disabled={items.length === 0}>
          Tout effacer
        </button>
      </div>

      <div className="card">
        <div className="form-group">
          <input
            className="input"
            type="text"
            placeholder="Rechercher par nom de release ou titre TMDB..."
            value={search}
            onChange={e => onSearchChange(e.target.value)}
          />
        </div>

        {error && <div className="alert alert-error">{error}</div>}

        {loading && items.length === 0 && (
          <div className="history-empty"><span className="spinner" /> Chargement...</div>
        )}

        {!loading && items.length === 0 && (
          <div className="history-empty">Aucun upload enregistré</div>
        )}

        <div className="history-list">
          {items.map(it => (
            <div key={it.id} className={`history-item history-item--${it.status}`}>
              <div className={`hi-status hi-status--${it.status}`} />
              <div className="hi-main">
                <div className="hi-name" title={it.sourcePath}>{it.releaseName}</div>
                <div className="hi-meta">
                  <span>🕒 {formatDate(it.createdAt)}</span>
                  {it.tmdbTitle && <span>🎬 {it.tmdbTitle}</span>}
                  {it.categoryName && <span>📂 {it.categoryName}</span>}
                  {it.size > 0 && <span>💾 {formatBytes(it.size)}</span>}
                  {it.noUpload && <span className="hi-badge-warn">(pas d'upload)</span>}
                </div>
                {it.status === 'error' && it.errorMsg && (
                  <div className="hi-error">{it.errorMsg}</div>
                )}
              </div>
              <div className="hi-actions">
                {it.uploadUrl && (
                  <button
                    className="btn btn-ghost hi-link"
                    onClick={() => BrowserOpenURL(it.uploadUrl)}
                    title="Ouvrir sur nexum-core.com"
                  >
                    ↗ #{it.uploadId}
                  </button>
                )}
                <button
                  className="btn btn-ghost hi-del"
                  onClick={() => onDelete(it.id)}
                  title="Supprimer"
                >
                  ✕
                </button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
