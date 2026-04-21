import React, { useState, useEffect, useRef, useCallback } from 'react'
import {
  SelectFiles, SelectDirectory, CreateTorrent,
  SearchTMDB, GetTMDBDetails, GenerateNFO, SaveNFO,
  UploadTorrent, DownloadTorrent, LargestVideoFile,
  AppLoadSettings, GenerateBBCode, CheckDuplicate, SaveHistoryEntry,
} from '../../wailsjs/go/main/App'
import { getMediaInfoJS, getMediaInfoCLIText } from '../services/mediainfo'
import { BrowserOpenURL, OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'
import type { main } from '../../wailsjs/go/models'
import './UploadPage.css'

// ─── Types ─────────────────────────────────────────────────────────────────

type QueueItemStatus = 'pending' | 'processing' | 'done' | 'error'

interface QueueItem {
  id: string
  path: string
  name: string
  status: QueueItemStatus
  step?: string
  error?: string
  uploadResult?: main.UploadResponse
  duplicateWarning?: { id: number; name: string; url: string }
}

let _queueIdCounter = 0
function makeQueueItem(path: string): QueueItem {
  return {
    id: String(++_queueIdCounter),
    path,
    name: path.split(/[/\\]/).pop() ?? path,
    status: 'pending',
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

// ─── Page ──────────────────────────────────────────────────────────────────

export default function UploadPage() {
  const [queue, setQueue] = useState<QueueItem[]>([])
  const [running, setRunning] = useState(false)
  const [dragging, setDragging] = useState(false)
  const queueRef = useRef<QueueItem[]>([])
  const runningRef = useRef(false)

  useEffect(() => { queueRef.current = queue }, [queue])

  const updateItem = useCallback((id: string, patch: Partial<QueueItem>) => {
    setQueue(q => q.map(it => it.id === id ? { ...it, ...patch } : it))
  }, [])

  const addPaths = useCallback((paths: string[]) => {
    const existing = new Set(queueRef.current.map(i => i.path))
    const fresh = paths
      .filter(p => p && !existing.has(p))
      .map(makeQueueItem)
    if (fresh.length === 0) return
    setQueue(q => [...q, ...fresh])

    // Duplicate pre-check as soon as the item enters the queue
    fresh.forEach(item => {
      CheckDuplicate(item.name).then(dup => {
        if (dup && dup.found) {
          updateItem(item.id, {
            duplicateWarning: { id: dup.id || 0, name: dup.name || '', url: dup.url || '' },
          })
        }
      }).catch(() => {})
    })
  }, [updateItem])

  // ── File drop registration ──────────────────────────────────────────────
  useEffect(() => {
    OnFileDrop((_x, _y, paths) => {
      if (paths.length === 0) return
      addPaths(paths)
      setDragging(false)
    }, true)
    return () => OnFileDropOff()
  }, [addPaths])

  // ── Queue processor ─────────────────────────────────────────────────────
  const processItem = async (item: QueueItem, nfoMode: string) => {
    const upd = (patch: Partial<QueueItem>) => updateItem(item.id, patch)
    upd({ status: 'processing', error: undefined, step: 'Création du torrent…' })

    let catId = 1
    try {
      const torrent = await CreateTorrent(item.path)
      upd({ name: torrent.name, step: 'Analyse média…' })

      let mi: main.MediaInfo = {} as main.MediaInfo
      let cliText = ''
      try {
        const videoPath = await LargestVideoFile(item.path)
        const parsed = await getMediaInfoJS(videoPath)
        mi = parsed as any
        try { cliText = await getMediaInfoCLIText(videoPath) } catch { /* */ }
      } catch { /* non bloquant */ }

      upd({ step: 'Recherche TMDB…' })
      let tmdb: main.TMDBDetails = {} as main.TMDBDetails
      let tmdbType = 'movie'
      try {
        const results = await SearchTMDB(torrent.name, '')
        if (results && results.length > 0) {
          const details = await GetTMDBDetails(results[0].id, results[0].mediaType)
          tmdb = details
          tmdbType = results[0].mediaType
          catId = tmdbType === 'tv' ? 2 : 1
        }
      } catch { /* non bloquant */ }

      upd({ step: 'Génération NFO…' })
      let nfo = ''
      if (nfoMode === 'mediainfo' && cliText) {
        nfo = cliText
      } else {
        nfo = await GenerateNFO(tmdb, mi, cliText)
      }
      try { await SaveNFO(nfo, torrent.name) } catch { /* non bloquant */ }

      upd({ step: 'Vérification doublon…' })
      const dup = await CheckDuplicate(torrent.name).catch(() => null)
      if (dup && dup.found) {
        throw new Error(`Doublon sur nexum : ${dup.name} (ID #${dup.id})`)
      }

      upd({ step: 'Upload…' })
      const bbDesc = await GenerateBBCode(torrent.name, cliText).catch(() => '')
      const result = await UploadTorrent({
        torrentPath: torrent.filePath,
        nfoContent: nfo,
        name: torrent.name,
        categoryId: catId,
        description: bbDesc || tmdb.overview || '',
        tmdbId: tmdb.id || 0,
        tmdbType,
        resolution: (mi as any).resolution || '',
        videoCodec: (mi as any).videoCodec || '',
        audioCodec: (mi as any).audioCodec || '',
        audioLanguages: (mi as any).audioLanguages || '',
        subtitleLanguages: (mi as any).subtitleLanguages || '',
        hdrFormat: (mi as any).hdrFormat || '',
        source: (mi as any).source || '',
      })

      if (result.torrent_id) {
        try { await DownloadTorrent(result.torrent_id) } catch { /* */ }
      }

      SaveHistoryEntry({
        id: 0, createdAt: '', sourcePath: item.path,
        releaseName: torrent.name, torrentPath: torrent.filePath, nfoPath: '',
        infoHash: torrent.infoHash, size: torrent.size,
        categoryId: catId, categoryName: '',
        tmdbId: tmdb.id || 0, tmdbType, tmdbTitle: tmdb.title || '',
        uploadUrl: result.url, uploadId: result.torrent_id,
        status: 'done', errorMsg: '', noUpload: false,
      } as main.HistoryEntry).catch(() => {})

      upd({ status: 'done', step: undefined, uploadResult: result, name: torrent.name })
    } catch (e) {
      SaveHistoryEntry({
        id: 0, createdAt: '', sourcePath: item.path,
        releaseName: item.name, torrentPath: '', nfoPath: '',
        infoHash: '', size: 0, categoryId: catId, categoryName: '',
        tmdbId: 0, tmdbType: '', tmdbTitle: '',
        uploadUrl: '', uploadId: 0,
        status: 'error', errorMsg: String(e), noUpload: false,
      } as main.HistoryEntry).catch(() => {})
      upd({ status: 'error', step: undefined, error: String(e) })
    }
  }

  const start = async () => {
    if (runningRef.current) return
    runningRef.current = true
    setRunning(true)
    const settings = await AppLoadSettings().catch(() => ({ nfoMode: 'nfo' } as any))
    while (runningRef.current) {
      const next = queueRef.current.find(i => i.status === 'pending')
      if (!next) break
      await processItem(next, settings.nfoMode || 'nfo')
    }
    runningRef.current = false
    setRunning(false)
  }

  const stop = () => {
    runningRef.current = false
    setRunning(false)
  }

  // ── Actions ─────────────────────────────────────────────────────────────
  const pickFiles = async () => {
    const paths = await SelectFiles(
      'Ajouter des fichiers vidéo', 'Fichiers vidéo',
      '*.mkv;*.mp4;*.avi;*.mov;*.ts;*.m2ts;*.wmv'
    ).catch(() => [])
    if (paths?.length) addPaths(paths)
  }

  const pickDir = async () => {
    const path = await SelectDirectory('Ajouter un dossier').catch(() => '')
    if (path) addPaths([path])
  }

  const remove = (id: string) => setQueue(q => q.filter(i => i.id !== id))
  const clearDone = () => setQueue(q => q.filter(i => i.status !== 'done'))
  const clearAll = () => setQueue([])

  // ── Stats ───────────────────────────────────────────────────────────────
  const pending = queue.filter(i => i.status === 'pending').length
  const processing = queue.find(i => i.status === 'processing')
  const done = queue.filter(i => i.status === 'done').length
  const errors = queue.filter(i => i.status === 'error').length
  const allDone = queue.length > 0 && pending === 0 && !processing

  const statusIcon = (s: QueueItemStatus) => {
    if (s === 'pending')    return <span className="queue-status-icon queue-status-pending">○</span>
    if (s === 'processing') return <span className="queue-status-icon queue-status-processing"><span className="spinner" /></span>
    if (s === 'done')       return <span className="queue-status-icon queue-status-done">✓</span>
    return <span className="queue-status-icon queue-status-error">✕</span>
  }

  return (
    <div className="upload-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Uploader</h1>
          <p className="page-subtitle text-secondary">
            Glissez un ou plusieurs fichiers — tout est automatique (TMDB, NFO, description, upload)
          </p>
        </div>
      </div>

      <div className="queue-panel">
        {/* Drop zone */}
        <div
          className={`queue-drop-zone${dragging ? ' queue-drop-zone--active' : ''}`}
          style={{ '--wails-drop-target': 'drop' } as React.CSSProperties}
          onDragOver={e => { e.preventDefault(); setDragging(true) }}
          onDragLeave={() => setDragging(false)}
        >
          <span className="drop-icon" style={{ fontSize: 28 }}>📂</span>
          <p className="drop-title" style={{ fontSize: 14 }}>Glissez des fichiers ou dossiers ici</p>
          <div className="queue-add-buttons">
            <button className="btn btn-secondary btn-sm" onClick={pickFiles}>+ Fichiers</button>
            <button className="btn btn-secondary btn-sm" onClick={pickDir}>+ Dossier</button>
          </div>
        </div>

        {/* Controls */}
        {queue.length > 0 && (
          <div className="queue-controls">
            <div className="queue-stats">
              <span className="queue-stat">{queue.length} élément{queue.length > 1 ? 's' : ''}</span>
              {pending > 0 && <span className="queue-stat queue-stat--pending">{pending} en attente</span>}
              {done > 0 && <span className="queue-stat queue-stat--done">{done} terminé{done > 1 ? 's' : ''}</span>}
              {errors > 0 && <span className="queue-stat queue-stat--error">{errors} erreur{errors > 1 ? 's' : ''}</span>}
            </div>
            <div style={{ display: 'flex', gap: 8 }}>
              {done > 0 && !running && (
                <button className="btn btn-ghost btn-sm" onClick={clearDone}>Effacer terminés</button>
              )}
              {allDone && !running && (
                <button className="btn btn-ghost btn-sm" onClick={clearAll}>Tout effacer</button>
              )}
              {running ? (
                <button className="btn btn-secondary btn-sm" onClick={stop}>⏹ Arrêter</button>
              ) : (
                <button className="btn btn-primary" onClick={start} disabled={pending === 0}>
                  ▶ Lancer ({pending})
                </button>
              )}
            </div>
          </div>
        )}

        {/* Queue list */}
        {queue.length > 0 && (
          <div className="queue-list">
            {queue.map(item => (
              <div key={item.id} className={`queue-item queue-item--${item.status}`}>
                <div className="queue-item-status">{statusIcon(item.status)}</div>
                <div className="queue-item-info">
                  <div className="queue-item-name">{item.name}</div>
                  {item.status === 'processing' && item.step && (
                    <div className="queue-item-step">{item.step}</div>
                  )}
                  {item.duplicateWarning && item.status !== 'done' && item.status !== 'error' && (
                    <div className="queue-item-step" style={{ color: 'var(--color-warning)' }}>
                      ⚠ Doublon potentiel :{' '}
                      <a
                        href="#" onClick={e => { e.preventDefault(); BrowserOpenURL(item.duplicateWarning!.url) }}
                        style={{ color: 'var(--color-warning)', textDecoration: 'underline' }}
                      >
                        {item.duplicateWarning.name} (#{item.duplicateWarning.id})
                      </a>
                    </div>
                  )}
                  {item.status === 'error' && item.error && (
                    <div className="queue-item-error">{item.error}</div>
                  )}
                  {item.status === 'done' && item.uploadResult && (
                    <div className="queue-item-done-row">
                      <span className="text-xs text-secondary">
                        ID {item.uploadResult.torrent_id}
                        {item.uploadResult.size ? ' — ' + formatBytes(item.uploadResult.size) : ''}
                      </span>
                      {item.uploadResult.url && (
                        <button
                          className="btn btn-ghost btn-xs"
                          onClick={() => BrowserOpenURL(item.uploadResult!.url)}
                        >🔗</button>
                      )}
                    </div>
                  )}
                </div>
                {item.status === 'pending' && !running && (
                  <button
                    className="btn btn-ghost btn-xs queue-item-remove"
                    onClick={() => remove(item.id)}
                  >✕</button>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
