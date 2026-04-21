import React, { useState } from 'react'
import { SelectFiles, SelectDirectory } from '../../wailsjs/go/main/App'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import { useUploadQueue, QueueItem, QueueItemStatus } from '../contexts/UploadQueueContext'
import ItemEditor from '../components/ItemEditor'
import './UploadPage.css'

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function statusIcon(s: QueueItemStatus) {
  if (s === 'analyzing')  return <span className="queue-status-icon"><span className="spinner" /></span>
  if (s === 'review')     return <span className="queue-status-icon queue-status-pending">◐</span>
  if (s === 'ready')      return <span className="queue-status-icon queue-status-pending" style={{ color: 'var(--color-accent)' }}>●</span>
  if (s === 'processing') return <span className="queue-status-icon queue-status-processing"><span className="spinner" /></span>
  if (s === 'done')       return <span className="queue-status-icon queue-status-done">✓</span>
  return <span className="queue-status-icon queue-status-error">✕</span>
}

export default function UploadPage() {
  const {
    queue, running, dragging, setDragging,
    addPaths, updateItem, validate, unvalidate, validateAll,
    remove, clearDone, clearAll, start, stop, reanalyze,
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

  const analyzing = queue.filter(i => i.status === 'analyzing').length
  const review = queue.filter(i => i.status === 'review').length
  const ready = queue.filter(i => i.status === 'ready').length
  const processing = queue.find(i => i.status === 'processing')
  const done = queue.filter(i => i.status === 'done').length
  const errors = queue.filter(i => i.status === 'error').length

  return (
    <div className="upload-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Uploader</h1>
          <p className="page-subtitle text-secondary">
            Glissez un ou plusieurs fichiers — l'analyse démarre automatiquement
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
              {analyzing > 0 && <span className="queue-stat queue-stat--pending">{analyzing} en analyse</span>}
              {review > 0 && <span className="queue-stat queue-stat--pending">{review} à valider</span>}
              {ready > 0 && <span className="queue-stat queue-stat--pending" style={{ color: 'var(--color-accent)' }}>{ready} prêt{ready > 1 ? 's' : ''}</span>}
              {done > 0 && <span className="queue-stat queue-stat--done">{done} terminé{done > 1 ? 's' : ''}</span>}
              {errors > 0 && <span className="queue-stat queue-stat--error">{errors} erreur{errors > 1 ? 's' : ''}</span>}
            </div>
            <div style={{ display: 'flex', gap: 8 }}>
              {review > 0 && !running && (
                <button className="btn btn-ghost btn-sm" onClick={validateAll}>✓ Tout valider</button>
              )}
              {done > 0 && !running && (
                <button className="btn btn-ghost btn-sm" onClick={clearDone}>Effacer terminés</button>
              )}
              {!running && queue.length > 0 && analyzing === 0 && review === 0 && ready === 0 && (
                <button className="btn btn-ghost btn-sm" onClick={clearAll}>Tout effacer</button>
              )}
              {running ? (
                <button className="btn btn-secondary btn-sm" onClick={stop}>⏹ Arrêter</button>
              ) : (
                <button className="btn btn-primary" onClick={start} disabled={ready === 0}>
                  ▶ Lancer ({ready})
                </button>
              )}
            </div>
          </div>
        )}

        {/* Queue list */}
        {queue.length > 0 && (
          <div className="queue-list">
            {queue.map(item => (
              <QueueItemCard
                key={item.id}
                item={item}
                running={running}
                onValidate={() => validate(item.id)}
                onUnvalidate={() => unvalidate(item.id)}
                onEdit={() => setEditingId(item.id)}
                onRemove={() => remove(item.id)}
                onReanalyze={() => reanalyze(item.id)}
              />
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
            initialName={item.overrides?.name || item.analysis?.releaseName || item.name}
            initial={{
              name: item.overrides?.name,
              categoryId: item.overrides?.categoryId ?? item.analysis?.categoryId,
              description: item.overrides?.description ?? item.analysis?.bbcodeDescription,
              tmdbId: item.overrides?.tmdbId ?? item.analysis?.tmdb?.id,
              tmdbType: item.overrides?.tmdbType ?? item.analysis?.tmdbType,
              tmdbTitle: item.overrides?.tmdbTitle ?? item.analysis?.tmdb?.title,
            }}
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

// ─── Item card ───────────────────────────────────────────────────────────

interface CardProps {
  item: QueueItem
  running: boolean
  onValidate: () => void
  onUnvalidate: () => void
  onEdit: () => void
  onRemove: () => void
  onReanalyze: () => void
}

function QueueItemCard({ item, running, onValidate, onUnvalidate, onEdit, onRemove, onReanalyze }: CardProps) {
  const a = item.analysis
  const ov = item.overrides || {}

  // Effective display values (overrides prioritaires)
  const effName = ov.name || a?.releaseName || item.name
  const effTmdbTitle = ov.tmdbTitle || a?.tmdb?.title || ''
  const effTmdbYear = a?.tmdb?.year || ''
  const effTmdbType = ov.tmdbType || a?.tmdbType || ''
  const effCategoryId = ov.categoryId ?? a?.categoryId
  const effCategoryName = a?.categoryName || ''
  const { queue: _, categories } = useUploadQueue() as any
  const effCategory = effCategoryId
    ? (categories?.find((c: any) => c.id === effCategoryId)?.name || effCategoryName)
    : effCategoryName

  const mi = a?.mediaInfo
  const hasOverrides = Object.keys(ov).length > 0

  return (
    <div className={`queue-item queue-item--${item.status} queue-item-card`}>
      <div className="queue-item-header">
        <div className="queue-item-status">{statusIcon(item.status)}</div>
        <div className="queue-item-info">
          <div className="queue-item-name">
            {effName}
            {hasOverrides && (
              <span className="qi-edited-badge" title="Personnalisé">✎</span>
            )}
          </div>
          {item.status === 'processing' && item.step && (
            <div className="queue-item-step">{item.step}</div>
          )}
        </div>
        <div className="queue-item-actions">
          {item.status === 'review' && !running && (
            <button className="btn btn-primary btn-xs" onClick={onValidate} title="Valider">
              ✓ OK
            </button>
          )}
          {item.status === 'ready' && !running && (
            <button className="btn btn-ghost btn-xs" onClick={onUnvalidate} title="Déverrouiller">
              ↩
            </button>
          )}
          {(item.status === 'review' || item.status === 'ready') && !running && (
            <button className="btn btn-ghost btn-xs" onClick={onEdit} title="Éditer">✎</button>
          )}
          {item.status === 'error' && !running && (
            <button className="btn btn-ghost btn-xs" onClick={onReanalyze} title="Ré-analyser">↻</button>
          )}
          {(item.status === 'review' || item.status === 'ready' || item.status === 'error') && !running && (
            <button className="btn btn-ghost btn-xs queue-item-remove" onClick={onRemove}>✕</button>
          )}
        </div>
      </div>

      {/* Inline details — visible only once analysis is done */}
      {a && (item.status === 'review' || item.status === 'ready') && (
        <div className="queue-item-details">
          <div className="qi-details-text">
            <div className="qi-detail-row">
              <span className="qi-detail-label">🎬 TMDB</span>
              <span className="qi-detail-value">
                {effTmdbTitle
                  ? `${effTmdbTitle} ${effTmdbYear ? '(' + effTmdbYear + ')' : ''} · ${effTmdbType === 'tv' ? 'Série' : 'Film'}`
                  : <span style={{ color: 'var(--color-warning)' }}>Aucun match</span>
                }
              </span>
            </div>
            <div className="qi-detail-row">
              <span className="qi-detail-label">📂 Catégorie</span>
              <span className="qi-detail-value">{effCategory || '—'}</span>
            </div>
            {mi && (mi.resolution || mi.videoCodec || mi.audioCodec) && (
              <div className="qi-detail-row">
                <span className="qi-detail-label">📺 Média</span>
                <span className="qi-detail-value">
                  {[mi.resolution, mi.videoCodec, mi.audioCodec, mi.source, mi.hdrFormat]
                    .filter(Boolean).join(' · ')}
                </span>
              </div>
            )}
            {mi && mi.audioLanguages && (
              <div className="qi-detail-row">
                <span className="qi-detail-label">🔊 Langues</span>
                <span className="qi-detail-value">{mi.audioLanguages}</span>
              </div>
            )}
          </div>
          {a.tmdb?.posterPath ? (
            <img
              src={a.tmdb.posterPath}
              alt=""
              className="qi-poster"
              loading="lazy"
            />
          ) : (
            <div className="qi-poster qi-poster-placeholder">🎬</div>
          )}
        </div>
      )}

      {item.duplicateWarning && item.status !== 'done' && item.status !== 'error' && (
        <div className="queue-item-duplicate">
          ⚠ Doublon potentiel :{' '}
          <a
            href="#" onClick={e => { e.preventDefault(); BrowserOpenURL(item.duplicateWarning!.url) }}
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
            >🔗 Voir sur nexum</button>
          )}
        </div>
      )}
    </div>
  )
}
