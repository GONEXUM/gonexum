import React, { useEffect, useRef, useState } from 'react'
import { SearchTMDB, GetTMDBDetails, GenerateBBCode, LargestVideoFile, GetCategories } from '../../wailsjs/go/main/App'
import { getMediaInfoCLIText } from '../services/mediainfo'
import type { main } from '../../wailsjs/go/models'
import './ItemEditor.css'

export interface ItemOverrides {
  name?: string
  categoryId?: number
  description?: string
  tmdbId?: number
  tmdbType?: string
  tmdbTitle?: string
}

interface ItemEditorProps {
  path: string
  initialName: string
  initial: ItemOverrides
  onCancel: () => void
  onSave: (overrides: ItemOverrides) => void
}

export default function ItemEditor({ path, initialName, initial, onCancel, onSave }: ItemEditorProps) {
  const [name, setName] = useState(initial.name || initialName)
  const [categoryId, setCategoryId] = useState(initial.categoryId || 1)
  const [description, setDescription] = useState(initial.description || '')
  const [tmdbId, setTmdbId] = useState(initial.tmdbId || 0)
  const [tmdbType, setTmdbType] = useState(initial.tmdbType || 'movie')
  const [tmdbTitle, setTmdbTitle] = useState(initial.tmdbTitle || '')
  const [tmdbQuery, setTmdbQuery] = useState(initial.name || initialName)
  const [tmdbResults, setTmdbResults] = useState<main.TMDBResult[]>([])
  const [tmdbSearching, setTmdbSearching] = useState(false)
  const [categories, setCategories] = useState<{ id: number; name: string }[]>([
    { id: 1, name: 'Films' }, { id: 2, name: 'Séries' },
    { id: 3, name: 'Documentaires' }, { id: 4, name: 'Animés' },
  ])
  const cliTextRef = useRef('')

  useEffect(() => {
    GetCategories().then(c => { if (c?.length) setCategories(c) }).catch(() => {})
    // Search TMDB automatically on open
    doSearch(initial.name || initialName, tmdbType)
    // Pre-fill BBCode description if empty
    if (!description) {
      LargestVideoFile(path)
        .then(videoPath => getMediaInfoCLIText(videoPath))
        .then(cli => {
          cliTextRef.current = cli
          return GenerateBBCode(initial.name || initialName, cli)
        })
        .then(bb => { if (bb && !description) setDescription(bb) })
        .catch(() => {})
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const doSearch = async (q: string, type: string) => {
    if (!q.trim()) return
    setTmdbSearching(true)
    try {
      const results = await SearchTMDB(q, type)
      setTmdbResults(results || [])
    } catch {
      setTmdbResults([])
    } finally {
      setTmdbSearching(false)
    }
  }

  const pickTMDB = async (r: main.TMDBResult) => {
    setTmdbId(r.id)
    setTmdbType(r.mediaType)
    setTmdbTitle(r.title)
    // Auto-adjust category based on media type
    if (r.mediaType === 'tv') setCategoryId(2)
    else if (categoryId === 2) setCategoryId(1)
    // Refresh description with richer TMDB data
    try {
      const details = await GetTMDBDetails(r.id, r.mediaType)
      setTmdbTitle(details.title || r.title)
    } catch { /* ignore */ }
  }

  const clearTMDB = () => {
    setTmdbId(0)
    setTmdbTitle('')
  }

  const save = () => {
    onSave({
      name: name.trim() !== initialName ? name.trim() : undefined,
      categoryId: categoryId !== 1 ? categoryId : undefined,
      description: description.trim() || undefined,
      tmdbId: tmdbId > 0 ? tmdbId : undefined,
      tmdbType: tmdbId > 0 ? tmdbType : undefined,
      tmdbTitle: tmdbTitle || undefined,
    })
  }

  return (
    <div className="item-editor-overlay" onClick={onCancel}>
      <div className="item-editor" onClick={e => e.stopPropagation()}>
        <div className="item-editor-header">
          <h2>Édition de l'item</h2>
          <button className="btn btn-ghost btn-sm" onClick={onCancel}>✕</button>
        </div>

        <div className="item-editor-body">
          <div className="form-group">
            <label className="label">Nom du release</label>
            <input
              className="input"
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
            />
          </div>

          <div className="form-group">
            <label className="label">Catégorie</label>
            <select
              className="select"
              value={categoryId}
              onChange={e => setCategoryId(Number(e.target.value))}
            >
              {categories.map(c => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </div>

          <div className="form-group">
            <label className="label">Match TMDB</label>
            <div className="tmdb-search-row">
              <input
                className="input"
                type="text"
                placeholder="Rechercher..."
                value={tmdbQuery}
                onChange={e => setTmdbQuery(e.target.value)}
                onKeyDown={e => { if (e.key === 'Enter') doSearch(tmdbQuery, tmdbType) }}
              />
              <select
                className="select tmdb-type-select"
                value={tmdbType}
                onChange={e => setTmdbType(e.target.value)}
              >
                <option value="movie">Film</option>
                <option value="tv">Série</option>
              </select>
              <button
                className="btn btn-secondary btn-sm"
                onClick={() => doSearch(tmdbQuery, tmdbType)}
                disabled={tmdbSearching}
              >
                {tmdbSearching ? <span className="spinner" /> : '🔍'}
              </button>
            </div>

            {tmdbResults.length > 0 && (
              <div className="tmdb-results">
                {tmdbResults.slice(0, 8).map(r => (
                  <div
                    key={`${r.id}-${r.mediaType}`}
                    className={`tmdb-result ${tmdbId === r.id ? 'tmdb-result--selected' : ''}`}
                    onClick={() => pickTMDB(r)}
                  >
                    {r.posterPath
                      ? <img src={r.posterPath} alt="" className="tmdb-poster" />
                      : <div className="tmdb-poster-ph">🎬</div>
                    }
                    <div className="tmdb-info">
                      <div className="tmdb-title">{r.title}</div>
                      <div className="tmdb-meta">
                        {r.year || '—'} · {r.mediaType === 'tv' ? 'Série' : 'Film'}
                      </div>
                    </div>
                    {tmdbId === r.id && <span className="tmdb-check">✓</span>}
                  </div>
                ))}
              </div>
            )}

            {tmdbId > 0 && (
              <div className="tmdb-selected-info">
                <span className="text-xs">Sélectionné : <strong>{tmdbTitle || `#${tmdbId}`}</strong></span>
                <button className="btn btn-ghost btn-xs" onClick={clearTMDB}>Désélectionner</button>
              </div>
            )}
          </div>

          <div className="form-group">
            <label className="label">
              Description (BBCode, max 10000 car.)
              <span className="text-xs text-muted" style={{ marginLeft: 8 }}>
                {description.length}/10000
              </span>
            </label>
            <textarea
              className="input item-editor-description"
              value={description}
              onChange={e => setDescription(e.target.value.slice(0, 10000))}
              rows={8}
              placeholder="Pré-remplie automatiquement après analyse..."
            />
          </div>
        </div>

        <div className="item-editor-footer">
          <button className="btn btn-ghost" onClick={onCancel}>Annuler</button>
          <button className="btn btn-primary" onClick={save}>Appliquer</button>
        </div>
      </div>
    </div>
  )
}
