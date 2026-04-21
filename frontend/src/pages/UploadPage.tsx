import React, { useState } from 'react'
import { SelectFiles, SelectDirectory } from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { useUploadQueue, QueueItemStatus } from '../contexts/UploadQueueContext'
import ItemEditor from '../components/ItemEditor'
import './UploadPage.css'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

export default function UploadPage() {
  const {
    queue, running, dragging, setDragging,
    addPaths, updateItem, remove, clearDone, clearAll, start, stop,
  } = useUploadQueue()

  const [editingId, setEditingId] = useState<string | null>(null)

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
                  <div className="queue-item-name">
                    {item.name}
                    {item.overrides && Object.keys(item.overrides).length > 0 && (
                      <span
                        title="Personnalisé"
                        style={{
                          marginLeft: 8, fontSize: 10, padding: '1px 6px',
                          background: 'var(--color-accent-dim)', color: 'var(--color-accent)',
                          borderRadius: 10, fontWeight: 600,
                        }}
                      >✎</span>
                    )}
                  </div>
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
                  <>
                    <button
                      className="btn btn-ghost btn-xs"
                      onClick={() => setEditingId(item.id)}
                      title="Éditer (TMDB, nom, description, catégorie)"
                    >✎</button>
                    <button
                      className="btn btn-ghost btn-xs queue-item-remove"
                      onClick={() => remove(item.id)}
                    >✕</button>
                  </>
                )}
              </div>
            ))}
          </div>
        )}
      </div>

      {editingId && (() => {
        const item = queue.find(i => i.id === editingId)
        if (!item) return null
        return (
          <ItemEditor
            path={item.path}
            initialName={item.name}
            initial={item.overrides || {}}
            onCancel={() => setEditingId(null)}
            onSave={(overrides) => {
              updateItem(item.id, {
                overrides,
                name: overrides.name || item.name,
              })
              setEditingId(null)
            }}
          />
        )
      })()}
    </div>
  )
}
