import React, { useState, useEffect, useRef } from 'react'
import {
  SelectFile, SelectDirectory, CreateTorrent,
  SearchTMDB, GetTMDBDetails, GenerateNFO, UploadTorrent, DownloadTorrent, ReadTextFile, LargestVideoFile,
  AppLoadSettings,
} from '../../wailsjs/go/main/App'
import { getMediaInfoJS, getMediaInfoCLIText } from '../services/mediainfo'
import { BrowserOpenURL, EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import type { main } from '../../wailsjs/go/models'
import './UploadPage.css'

interface TorrentProgress {
  phase: 'start' | 'hashing' | 'writing'
  percent: number
  bytesDone: number
  totalBytes: number
  currentFile: string
}

type Step = 'source' | 'media' | 'metadata' | 'upload' | 'done'

const STEPS: { id: Step; label: string }[] = [
  { id: 'source', label: 'Source' },
  { id: 'media', label: 'Média' },
  { id: 'metadata', label: 'Métadonnées' },
  { id: 'upload', label: 'Upload' },
]

const CATEGORIES = [
  { id: 1, label: 'Films' },
  { id: 2, label: 'Séries' },
  { id: 3, label: 'Documentaires' },
  { id: 4, label: 'Animés' },
]

const RESOLUTIONS = ['2160p', '1080p', '720p', 'SD']
const VIDEO_CODECS = ['H.265', 'H.264', 'AV1', 'VP9', 'XviD']
const AUDIO_CODECS = ['Atmos', 'TrueHD', 'DTS-HD MA', 'DTS-HD', 'DTS', 'EAC3', 'AC3', 'FLAC', 'AAC', 'MP3']
const HDR_FORMATS = ['', 'HDR', 'HDR10', 'HDR10+', 'DV', 'HDR DV']
const SOURCES = ['BluRay', 'WEB-DL', 'WEBRip', 'HDTV', 'DVDRip', 'DCP']

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function StepIndicator({ current, steps }: { current: Step; steps: typeof STEPS }) {
  const currentIdx = steps.findIndex(s => s.id === current)
  return (
    <div className="step-indicator">
      {steps.map((step, idx) => (
        <React.Fragment key={step.id}>
          <div className={`step-dot ${idx <= currentIdx ? 'step-done' : ''} ${idx === currentIdx ? 'step-current' : ''}`}>
            <span className="step-num">{idx < currentIdx ? '✓' : idx + 1}</span>
            <span className="step-label">{step.label}</span>
          </div>
          {idx < steps.length - 1 && (
            <div className={`step-line ${idx < currentIdx ? 'step-line-done' : ''}`} />
          )}
        </React.Fragment>
      ))}
    </div>
  )
}

export default function UploadPage() {
  const [step, setStep] = useState<Step>('source')
  const [sourcePath, setSourcePath] = useState('')
  const [isDir, setIsDir] = useState(false)
  const [torrent, setTorrent] = useState<main.TorrentResult | null>(null)
  const [mediaInfo, setMediaInfo] = useState<main.MediaInfo | null>(null)
  const [tmdbResults, setTmdbResults] = useState<main.TMDBResult[]>([])
  const [tmdbDetails, setTmdbDetails] = useState<main.TMDBDetails | null>(null)
  const [nfoContent, setNfoContent] = useState('')
  const [uploadResult, setUploadResult] = useState<main.UploadResponse | null>(null)

  const [loading, setLoading] = useState(false)
  const [loadingMsg, setLoadingMsg] = useState('')
  const [error, setError] = useState('')

  // Upload form state
  const [name, setName] = useState('')
  const [categoryId, setCategoryId] = useState(1)
  const [description, setDescription] = useState('')
  const [tmdbType, setTmdbType] = useState('movie')
  const [resolution, setResolution] = useState('')
  const [videoCodec, setVideoCodec] = useState('')
  const [audioCodec, setAudioCodec] = useState('')
  const [audioLangs, setAudioLangs] = useState('')
  const [hdrFormat, setHdrFormat] = useState('')
  const [source, setSource] = useState('')

  // TMDB search
  const [tmdbQuery, setTmdbQuery] = useState('')
  const [tmdbSearchStatus, setTmdbSearchStatus] = useState<'idle' | 'searching' | 'found' | 'not_found'>('idle')
  const [tmdbSearching, setTmdbSearching] = useState(false)
  const [showTmdbAlternatives, setShowTmdbAlternatives] = useState(false)
  const [tmdbUrlInput, setTmdbUrlInput] = useState('')
  const [tmdbUrlError, setTmdbUrlError] = useState('')

  // NFO
  const [nfoMode, setNfoMode] = useState<'generate' | 'existing'>('generate')
  const [nfoFilePath, setNfoFilePath] = useState('')
  const [settingsNfoMode, setSettingsNfoMode] = useState<'nfo' | 'mediainfo'>('nfo')
  const [mediaInfoCLIText, setMediaInfoCLIText] = useState('')

  const [torrentProgress, setTorrentProgress] = useState<TorrentProgress | null>(null)

  const dropRef = useRef<HTMLDivElement>(null)
  const [dragging, setDragging] = useState(false)

  useEffect(() => {
    EventsOn('torrent:progress', (data: TorrentProgress) => {
      setTorrentProgress(data)
    })
    AppLoadSettings().then(s => {
      if (s.nfoMode === 'mediainfo') setSettingsNfoMode('mediainfo')
    }).catch(() => {})
    return () => { EventsOff('torrent:progress') }
  }, [])

  const err = (e: unknown) => {
    const msg = e instanceof Error ? e.message : String(e)
    setError(msg)
    setLoading(false)
  }

  // Step 1: Source
  const handlePickFile = async () => {
    try {
      const path = await SelectFile('Sélectionner un fichier vidéo', 'Fichiers vidéo', '*.mkv;*.mp4;*.avi;*.mov;*.ts;*.m2ts;*.wmv')
      if (path) { setSourcePath(path); setIsDir(false); setError('') }
    } catch (e) { err(e) }
  }

  const handlePickDir = async () => {
    try {
      const path = await SelectDirectory('Sélectionner un dossier')
      if (path) { setSourcePath(path); setIsDir(true); setError('') }
    } catch (e) { err(e) }
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setDragging(false)
    const items = Array.from(e.dataTransfer.files)
    if (items.length > 0) {
      // Wails drag-drop provides paths via the file object
      const file = items[0] as any
      const path = file.path || ''
      if (path) {
        setSourcePath(path)
        setIsDir(false)
        setError('')
      }
    }
  }

  const handleNext_Source = async () => {
    if (!sourcePath) { setError('Veuillez sélectionner un fichier ou dossier.'); return }
    setError('')
    setLoading(true)
    setTorrentProgress(null)
    setLoadingMsg('Création du fichier .torrent...')
    try {
      const t = await CreateTorrent(sourcePath)
      setTorrent(t)
      setName(t.name)

      // Try to extract media info (from file or largest video in folder)
      setLoadingMsg('Extraction des informations médias...')
      try {
        const videoPath = await LargestVideoFile(sourcePath)
        const mi = await getMediaInfoJS(videoPath)
        setMediaInfo(mi as any)
        setResolution(mi.resolution || '')
        setVideoCodec(mi.videoCodec || '')
        setAudioCodec(mi.audioCodec || '')
        setAudioLangs(mi.audioLanguages || '')
        setHdrFormat(mi.hdrFormat || '')
        setSource(mi.source || '')
        // Si le mode MediaInfo CLI est actif, pré-génère le contenu NFO
        try {
          const cliText = await getMediaInfoCLIText(videoPath)
          setMediaInfoCLIText(cliText)
          const currentSettings = await AppLoadSettings()
          if (currentSettings.nfoMode === 'mediainfo') {
            setNfoContent(cliText)
          }
        } catch { /* non bloquant */ }
      } catch {
        setMediaInfo(null)
      }
      setStep('media')
    } catch (e) { err(e) }
    setLoading(false)
  }

  // Step 2: Media → auto-lance la recherche TMDB en passant à l'étape suivante
  const handleNext_Media = async () => {
    setError('')
    setTmdbSearchStatus('searching')
    setTmdbDetails(null)
    setTmdbResults([])
    setShowTmdbAlternatives(false)
    setNfoContent('')
    setStep('metadata')

    try {
      const results = await SearchTMDB(name, '')
      setTmdbResults(results || [])
      if (results && results.length > 0) {
        setTmdbSearchStatus('found')
        // Auto-sélectionne le premier résultat
        await selectTMDB(results[0])
      } else {
        setTmdbSearchStatus('not_found')
      }
    } catch {
      setTmdbSearchStatus('not_found')
    }
  }

  // Step 3: recherche manuelle
  const handleTMDBSearch = async () => {
    if (!tmdbQuery.trim()) return
    setTmdbSearching(true)
    setTmdbSearchStatus('searching')
    setError('')
    try {
      const results = await SearchTMDB(tmdbQuery, '')
      setTmdbResults(results || [])
      setTmdbSearchStatus(results && results.length > 0 ? 'found' : 'not_found')
    } catch (e) { err(e) }
    setTmdbSearching(false)
  }

  // Sélectionne un résultat et charge ses détails
  const selectTMDB = async (result: main.TMDBResult) => {
    setTmdbSearching(true)
    try {
      const details = await GetTMDBDetails(result.id, result.mediaType)
      setTmdbDetails(details)
      setTmdbType(result.mediaType)
      if (result.mediaType === 'tv') setCategoryId(2)
      else setCategoryId(1)
      const nfo = await GenerateNFO(details, mediaInfo || {} as main.MediaInfo, mediaInfoCLIText)
      setNfoContent(nfo)
      setShowTmdbAlternatives(false)
    } catch (e) { err(e) }
    setTmdbSearching(false)
  }

  const handleSelectTMDB = (result: main.TMDBResult) => selectTMDB(result)

  // Entrée manuelle par URL TheMovieDB
  const handleTMDBUrl = async () => {
    setTmdbUrlError('')
    const match = tmdbUrlInput.match(/themoviedb\.org\/(movie|tv)\/(\d+)/)
    if (!match) {
      setTmdbUrlError('URL invalide. Format attendu : https://www.themoviedb.org/movie/603 ou /tv/1396')
      return
    }
    const type = match[1]
    const id = parseInt(match[2], 10)
    setTmdbSearching(true)
    try {
      const details = await GetTMDBDetails(id, type)
      setTmdbDetails(details)
      setTmdbType(type)
      if (type === 'tv') setCategoryId(2)
      else setCategoryId(1)
      const nfo = await GenerateNFO(details, mediaInfo || {} as main.MediaInfo, mediaInfoCLIText)
      setNfoContent(nfo)
      setTmdbSearchStatus('found')
      setShowTmdbAlternatives(false)
      setTmdbUrlInput('')
    } catch (e) { err(e) }
    setTmdbSearching(false)
  }

  const handlePickNFO = async () => {
    try {
      const path = await SelectFile('Sélectionner un fichier NFO', 'Fichiers NFO', '*.nfo')
      if (!path) return
      const content = await ReadTextFile(path)
      setNfoFilePath(path)
      setNfoContent(content)
    } catch (e) { err(e) }
  }

  const handleNext_Metadata = () => {
    if (!name) { setError('Veuillez renseigner le nom du torrent.'); return }
    if (!nfoContent) {
      // Generate a minimal NFO
      GenerateNFO(tmdbDetails || {} as main.TMDBDetails, mediaInfo || {} as main.MediaInfo, mediaInfoCLIText)
        .then(nfo => setNfoContent(nfo))
    }
    setError('')
    setStep('upload')
  }

  // Step 4: Upload
  const handleUpload = async () => {
    if (!torrent) { setError('Torrent non créé.'); return }
    if (!name) { setError('Nom requis.'); return }
    setLoading(true)
    setLoadingMsg('Upload en cours...')
    setError('')
    try {
      const nfo = nfoContent || await GenerateNFO(tmdbDetails || {} as main.TMDBDetails, mediaInfo || {} as main.MediaInfo, mediaInfoCLIText)
      const result = await UploadTorrent({
        torrentPath: torrent.filePath,
        nfoContent: nfo,
        name,
        categoryId,
        description,
        tmdbId: tmdbDetails?.id || 0,
        tmdbType,
        resolution,
        videoCodec,
        audioCodec,
        audioLanguages: audioLangs,
        hdrFormat,
        source,
      })
      setUploadResult(result)
      setStep('done')
      if (result.torrent_id) {
        try {
          await DownloadTorrent(result.torrent_id)
        } catch (_) {
          // téléchargement non bloquant
        }
      }
    } catch (e) { err(e) }
    setLoading(false)
  }

  const handleReset = () => {
    setStep('source')
    setSourcePath(''); setIsDir(false); setTorrent(null)
    setMediaInfo(null); setTmdbResults([]); setTmdbDetails(null)
    setTmdbSearchStatus('idle'); setTmdbSearching(false)
    setShowTmdbAlternatives(false); setTmdbUrlInput(''); setTmdbUrlError('')
    setNfoContent(''); setNfoMode('generate'); setNfoFilePath('')
    setUploadResult(null); setName('')
    setResolution(''); setVideoCodec(''); setAudioCodec('')
    setAudioLangs(''); setHdrFormat(''); setSource('')
    setError('')
  }

  return (
    <div className="upload-page">
      <div className="page-header">
        <div>
          <h1 className="page-title">Uploader un Torrent</h1>
          <p className="page-subtitle text-secondary">Créez et uploadez un torrent sur Nexum en quelques étapes</p>
        </div>
      </div>

      {step !== 'done' && (
        <StepIndicator current={step} steps={STEPS} />
      )}

      {error && (
        <div className="alert alert-error" style={{ margin: '0 24px 16px' }}>
          <span>⚠</span>
          <span>{error}</span>
          <button className="btn btn-ghost btn-sm" style={{ marginLeft: 'auto' }} onClick={() => setError('')}>✕</button>
        </div>
      )}

      <div className="step-content">
        {/* Step 1: Source */}
        {step === 'source' && (
          <div className="step-panel">
            <div
              ref={dropRef}
              className={`drop-zone ${dragging ? 'drop-zone-active' : ''} ${sourcePath ? 'drop-zone-filled' : ''}`}
              onDragOver={e => { e.preventDefault(); setDragging(true) }}
              onDragLeave={() => setDragging(false)}
              onDrop={handleDrop}
            >
              {sourcePath ? (
                <div className="drop-zone-selected">
                  <div className="drop-icon">{isDir ? '📁' : '🎬'}</div>
                  <div className="drop-path">{sourcePath}</div>
                  <button className="btn btn-ghost btn-sm" onClick={() => setSourcePath('')}>Changer</button>
                </div>
              ) : (
                <>
                  <div className="drop-icon">📂</div>
                  <p className="drop-title">Glissez un fichier ou dossier ici</p>
                  <p className="drop-sub text-secondary text-sm">ou choisissez manuellement</p>
                </>
              )}
            </div>

            <div className="source-buttons">
              <button className="btn btn-secondary" onClick={handlePickFile}>
                🎬 Sélectionner un fichier
              </button>
              <button className="btn btn-secondary" onClick={handlePickDir}>
                📁 Sélectionner un dossier
              </button>
            </div>

            {torrentProgress && loading && (
              <TorrentProgressBar progress={torrentProgress} />
            )}

            <div className="step-actions">
              <button
                className="btn btn-primary btn-lg"
                onClick={handleNext_Source}
                disabled={!sourcePath || loading}
              >
                {loading ? <><span className="spinner" />{loadingMsg}</> : 'Continuer →'}
              </button>
            </div>
          </div>
        )}

        {/* Step 2: Media Info */}
        {step === 'media' && (
          <div className="step-panel">
            {torrent && (
              <div className="card" style={{ marginBottom: 16 }}>
                <div className="card-header">
                  <span className="card-title">Torrent créé</span>
                  <span className="badge badge-success">✓ OK</span>
                </div>
                <div className="info-grid">
                  <InfoRow label="Nom" value={torrent.name} />
                  <InfoRow label="Taille" value={formatBytes(torrent.size)} />
                  <InfoRow label="Info Hash" value={torrent.infoHash} mono />
                  <InfoRow label="Fichier" value={torrent.filePath} />
                </div>
              </div>
            )}

            <div className="card">
              <div className="card-header">
                <span className="card-title">Informations Média</span>
                {mediaInfo ? (
                  <span className="badge badge-success">Auto-détecté</span>
                ) : (
                  <span className="badge badge-danger">Saisie manuelle</span>
                )}
              </div>

              {!mediaInfo && (
                <div className="alert alert-warning" style={{ marginBottom: 16 }}>
                  <span>ℹ</span>
                  <span>Analyse automatique indisponible. Renseignez les informations manuellement.</span>
                </div>
              )}

              <div className="grid-2" style={{ gap: 12 }}>
                <div className="form-group">
                  <label className="label">Résolution</label>
                  <select className="select" value={resolution} onChange={e => setResolution(e.target.value)}>
                    <option value="">— Sélectionner —</option>
                    {RESOLUTIONS.map(r => <option key={r} value={r}>{r}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <label className="label">Codec Vidéo</label>
                  <select className="select" value={videoCodec} onChange={e => setVideoCodec(e.target.value)}>
                    <option value="">— Sélectionner —</option>
                    {VIDEO_CODECS.map(c => <option key={c} value={c}>{c}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <label className="label">Codec Audio</label>
                  <select className="select" value={audioCodec} onChange={e => setAudioCodec(e.target.value)}>
                    <option value="">— Sélectionner —</option>
                    {AUDIO_CODECS.map(c => <option key={c} value={c}>{c}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <label className="label">Langues Audio</label>
                  <input className="input" value={audioLangs} onChange={e => setAudioLangs(e.target.value)} placeholder="ex: Français (France), Anglais" />
                </div>
                <div className="form-group">
                  <label className="label">Format HDR</label>
                  <select className="select" value={hdrFormat} onChange={e => setHdrFormat(e.target.value)}>
                    <option value="">Aucun</option>
                    {HDR_FORMATS.filter(h => h).map(h => <option key={h} value={h}>{h}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <label className="label">Source</label>
                  <select className="select" value={source} onChange={e => setSource(e.target.value)}>
                    <option value="">— Sélectionner —</option>
                    {SOURCES.map(s => <option key={s} value={s}>{s}</option>)}
                  </select>
                </div>
              </div>

              {mediaInfo && (
                <div style={{ marginTop: 16 }}>
                  <div className="divider" />
                  <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                    {mediaInfo.width > 0 && <span className="tag">{mediaInfo.width}×{mediaInfo.height}</span>}
                    {mediaInfo.duration && <span className="tag">{mediaInfo.duration}</span>}
                    {mediaInfo.frameRate > 0 && <span className="tag">{mediaInfo.frameRate} fps</span>}
                    {mediaInfo.fileSize > 0 && <span className="tag">{formatBytes(mediaInfo.fileSize)}</span>}
                  </div>
                </div>
              )}
            </div>

            <div className="step-actions">
              <button className="btn btn-ghost" onClick={() => setStep('source')}>← Retour</button>
              <button className="btn btn-primary btn-lg" onClick={handleNext_Media}>Continuer →</button>
            </div>
          </div>
        )}

        {/* Step 3: Metadata / TMDB */}
        {step === 'metadata' && (
          <div className="step-panel">

            {/* ── Résultat sélectionné ── */}
            {tmdbDetails ? (
              <div className="card tmdb-details-card" style={{ marginBottom: 16 }}>
                <div className="card-header">
                  <span className="card-title">Métadonnées TMDB</span>
                  <div style={{ display: 'flex', gap: 8 }}>
                    {tmdbSearching && <span className="spinner" />}
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => setShowTmdbAlternatives(v => !v)}
                    >
                      {showTmdbAlternatives ? '✕ Fermer' : '↺ Changer'}
                    </button>
                  </div>
                </div>
                <div className="tmdb-details-inner">
                  {tmdbDetails.posterPath && (
                    <img src={tmdbDetails.posterPath} alt={tmdbDetails.title} className="tmdb-details-poster" />
                  )}
                  <div className="tmdb-details-info">
                    <h3 className="tmdb-details-title">
                      {tmdbDetails.title}
                      {tmdbDetails.year && <span className="text-secondary"> ({tmdbDetails.year})</span>}
                    </h3>
                    <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap', marginBottom: 8 }}>
                      <span className="badge badge-accent">{tmdbDetails.mediaType === 'tv' ? 'Série' : 'Film'}</span>
                      {tmdbDetails.genres?.map(g => <span key={g} className="tag">{g}</span>)}
                      {tmdbDetails.rating > 0 && <span className="tag tag-accent">⭐ {tmdbDetails.rating.toFixed(1)}</span>}
                      {tmdbDetails.runtime > 0 && <span className="tag">{tmdbDetails.runtime} min</span>}
                    </div>
                    {tmdbDetails.overview && (
                      <p className="text-sm text-secondary" style={{ lineHeight: 1.6 }}>{tmdbDetails.overview}</p>
                    )}
                  </div>
                </div>
              </div>
            ) : (
              /* ── Chargement auto ou aucun résultat ── */
              <div className="card" style={{ marginBottom: 16 }}>
                {tmdbSearchStatus === 'searching' ? (
                  <div className="tmdb-auto-searching">
                    <span className="spinner spinner-lg" />
                    <p>Recherche TMDB en cours…</p>
                    <p className="text-secondary text-sm">{name}</p>
                  </div>
                ) : (
                  <div className="tmdb-auto-searching">
                    <span style={{ fontSize: 28 }}>🔍</span>
                    <p className="font-bold">Aucun résultat trouvé</p>
                    <p className="text-secondary text-sm">Recherchez manuellement ou collez un lien TMDB ci-dessous.</p>
                  </div>
                )}
              </div>
            )}

            {/* ── Alternatives / recherche manuelle ── */}
            {(showTmdbAlternatives || tmdbSearchStatus === 'not_found') && (
              <div className="card" style={{ marginBottom: 16 }}>
                <div className="card-header">
                  <span className="card-title">
                    {tmdbSearchStatus === 'not_found' ? 'Recherche manuelle' : 'Choisir une autre entrée'}
                  </span>
                </div>

                {/* Recherche par nom */}
                <div style={{ display: 'flex', gap: 8, marginBottom: 12 }}>
                  <input
                    className="input"
                    value={tmdbQuery}
                    onChange={e => setTmdbQuery(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && handleTMDBSearch()}
                    placeholder={name || 'Nom du film ou de la série...'}
                  />
                  <button
                    className="btn btn-ghost btn-sm"
                    onClick={() => setTmdbQuery(name)}
                    title="Utiliser le nom du torrent"
                  >↗</button>
                  <button
                    className="btn btn-primary"
                    onClick={handleTMDBSearch}
                    disabled={tmdbSearching || !tmdbQuery.trim()}
                  >
                    {tmdbSearching ? <span className="spinner" /> : '🔍'}
                  </button>
                </div>

                {/* Résultats de recherche */}
                {tmdbResults.length > 0 && (
                  <div className="tmdb-results" style={{ marginBottom: 12 }}>
                    {tmdbResults.slice(0, 8).map(r => (
                      <button
                        key={r.id}
                        className={`tmdb-result-item ${tmdbDetails?.id === r.id ? 'tmdb-result-selected' : ''}`}
                        onClick={() => handleSelectTMDB(r)}
                        disabled={tmdbSearching}
                      >
                        <div className="tmdb-poster">
                          {r.posterPath
                            ? <img src={r.posterPath} alt={r.title} />
                            : <div className="tmdb-poster-placeholder">🎬</div>
                          }
                        </div>
                        <div className="tmdb-info">
                          <div className="tmdb-title">{r.title}</div>
                          <div className="tmdb-meta">
                            <span className="badge badge-accent">{r.mediaType === 'tv' ? 'Série' : 'Film'}</span>
                            {r.year && <span className="text-secondary text-sm">{r.year}</span>}
                          </div>
                          {r.overview && <div className="tmdb-overview text-secondary text-sm">{r.overview}</div>}
                        </div>
                      </button>
                    ))}
                  </div>
                )}

                <div className="divider" />

                {/* Lien direct TMDB */}
                <div className="form-group">
                  <label className="label">Ou coller un lien TheMovieDB</label>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <input
                      className="input"
                      value={tmdbUrlInput}
                      onChange={e => { setTmdbUrlInput(e.target.value); setTmdbUrlError('') }}
                      onKeyDown={e => e.key === 'Enter' && handleTMDBUrl()}
                      placeholder="https://www.themoviedb.org/movie/603"
                    />
                    <button
                      className="btn btn-primary"
                      onClick={handleTMDBUrl}
                      disabled={tmdbSearching || !tmdbUrlInput.trim()}
                    >
                      {tmdbSearching ? <span className="spinner" /> : 'OK'}
                    </button>
                  </div>
                  {tmdbUrlError && <span className="text-danger text-xs">{tmdbUrlError}</span>}
                  <span className="text-muted text-xs">
                    Accepte les URLs movie et tv — ex: themoviedb.org/tv/1396
                  </span>
                </div>
              </div>
            )}

            {/* ── Nom du torrent ── */}
            <div className="card" style={{ marginBottom: 16 }}>
              <div className="card-header">
                <span className="card-title">Nom du torrent</span>
              </div>
              <div className="form-group">
                <label className="label">Nom (nommage scene recommandé)</label>
                <input className="input" value={name} onChange={e => setName(e.target.value)} placeholder="Film.2024.MULTI.1080p.WEB-DL.x264-TEAM" />
              </div>
            </div>

            {/* ── NFO ── */}
            <div className="card" style={{ marginBottom: 16 }}>
              <div className="card-header">
                <span className="card-title">Fichier NFO</span>
              </div>

              {/* Toggle */}
              <div className="nfo-mode-toggle">
                <button
                  className={`nfo-mode-btn ${nfoMode === 'generate' ? 'nfo-mode-active' : ''}`}
                  onClick={() => setNfoMode('generate')}
                >
                  ✨ Générer automatiquement
                </button>
                <button
                  className={`nfo-mode-btn ${nfoMode === 'existing' ? 'nfo-mode-active' : ''}`}
                  onClick={() => setNfoMode('existing')}
                >
                  📄 Utiliser un fichier existant
                </button>
              </div>

              {nfoMode === 'existing' ? (
                <div style={{ marginTop: 12 }}>
                  <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                    <button className="btn btn-secondary" onClick={handlePickNFO}>
                      📂 Choisir un .nfo
                    </button>
                    {nfoFilePath && (
                      <span className="text-secondary text-xs truncate" style={{ maxWidth: 320 }}>
                        {nfoFilePath}
                      </span>
                    )}
                  </div>
                  {!nfoFilePath && (
                    <p className="text-muted text-xs" style={{ marginTop: 8 }}>
                      Aucun fichier sélectionné — un NFO généré sera utilisé si vous ne choisissez rien.
                    </p>
                  )}
                </div>
              ) : null}

              {nfoContent && (
                <div style={{ marginTop: 12 }}>
                  {nfoMode === 'existing' && nfoFilePath && (
                    <div className="divider" />
                  )}
                  <pre className="nfo-preview font-mono text-xs">{nfoContent}</pre>
                </div>
              )}
            </div>

            <div className="step-actions">
              <button className="btn btn-ghost" onClick={() => setStep('media')}>← Retour</button>
              <button
                className="btn btn-primary btn-lg"
                onClick={handleNext_Metadata}
                disabled={!name || tmdbSearching}
              >
                Continuer →
              </button>
            </div>
          </div>
        )}

        {/* Step 4: Upload */}
        {step === 'upload' && (
          <div className="step-panel">
            <div className="card" style={{ marginBottom: 16 }}>
              <div className="card-header">
                <span className="card-title">Récapitulatif</span>
              </div>
              <div className="info-grid">
                <InfoRow label="Nom" value={name} />
                {torrent && <InfoRow label="Taille" value={formatBytes(torrent.size)} />}
                {torrent && <InfoRow label="Info Hash" value={torrent.infoHash} mono />}
              </div>
              <div className="divider" />
              <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
                {resolution && <span className="tag tag-accent">{resolution}</span>}
                {videoCodec && <span className="tag tag-accent">{videoCodec}</span>}
                {audioCodec && <span className="tag tag-accent">{audioCodec}</span>}
                {hdrFormat && <span className="tag tag-accent">{hdrFormat}</span>}
                {source && <span className="tag">{source}</span>}
              </div>
            </div>

            <div className="card" style={{ marginBottom: 16 }}>
              <div className="card-header">
                <span className="card-title">Paramètres d'upload</span>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                <div className="form-group">
                  <label className="label">Catégorie</label>
                  <select className="select" value={categoryId} onChange={e => setCategoryId(Number(e.target.value))}>
                    {CATEGORIES.map(c => <option key={c.id} value={c.id}>{c.label}</option>)}
                  </select>
                </div>
                <div className="form-group">
                  <label className="label">Type TMDB</label>
                  <select className="select" value={tmdbType} onChange={e => setTmdbType(e.target.value)}>
                    <option value="movie">Film</option>
                    <option value="tv">Série</option>
                  </select>
                </div>
                <div className="form-group">
                  <label className="label">Description (BBCode supporté, max 5000 car.)</label>
                  <textarea
                    className="textarea"
                    value={description}
                    onChange={e => setDescription(e.target.value.slice(0, 5000))}
                    placeholder="Description optionnelle..."
                    rows={4}
                  />
                  <span className="text-xs text-muted">{description.length}/5000</span>
                </div>
              </div>
            </div>

            {tmdbDetails && (
              <div className="card" style={{ marginBottom: 16 }}>
                <div className="info-grid">
                  <InfoRow label="TMDB ID" value={String(tmdbDetails.id)} />
                  <InfoRow label="Titre" value={tmdbDetails.title} />
                </div>
              </div>
            )}

            <div className="step-actions">
              <button className="btn btn-ghost" onClick={() => setStep('metadata')}>← Retour</button>
              <button className="btn btn-primary btn-lg" onClick={handleUpload} disabled={loading}>
                {loading ? <><span className="spinner" />{loadingMsg}</> : '🚀 Uploader sur Nexum'}
              </button>
            </div>
          </div>
        )}

        {/* Done */}
        {step === 'done' && uploadResult && (
          <div className="step-panel">
            <div className="success-panel">
              <div className="success-icon">✓</div>
              <h2 className="success-title">Upload réussi !</h2>
              <p className="text-secondary">Votre torrent a été uploadé avec succès sur Nexum.</p>
            </div>

            <div className="card" style={{ marginBottom: 16 }}>
              <div className="card-header">
                <span className="card-title">Détails</span>
                <span className={`badge ${uploadResult.status === 'approved' ? 'badge-success' : 'badge-accent'}`}>
                  {uploadResult.status}
                </span>
              </div>
              <div className="info-grid">
                <InfoRow label="ID" value={String(uploadResult.torrent_id)} />
                <InfoRow label="Nom" value={uploadResult.name} />
                <InfoRow label="Taille" value={formatBytes(uploadResult.size)} />
                <InfoRow label="Info Hash" value={uploadResult.info_hash} mono />
              </div>
              {uploadResult.url && (
                <div style={{ marginTop: 12 }}>
                  <button
                    className="btn btn-secondary"
                    onClick={() => BrowserOpenURL(uploadResult.url)}
                  >
                    🔗 Voir sur Nexum
                  </button>
                </div>
              )}
            </div>

            <div className="step-actions">
              <button className="btn btn-primary btn-lg" onClick={handleReset}>
                ↺ Nouveau torrent
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

function TorrentProgressBar({ progress }: { progress: TorrentProgress }) {
  const phaseLabel = progress.phase === 'writing'
    ? 'Écriture du fichier .torrent...'
    : progress.phase === 'start'
    ? 'Initialisation...'
    : 'Hachage des pièces...'

  const pct = Math.min(100, Math.max(0, progress.percent))

  return (
    <div className="torrent-progress">
      <div className="torrent-progress-header">
        <span className="torrent-progress-label">{phaseLabel}</span>
        <span className="torrent-progress-pct">{pct.toFixed(1)}%</span>
      </div>
      <div className="torrent-progress-bar">
        <div className="torrent-progress-fill" style={{ width: `${pct}%` }} />
      </div>
      {progress.currentFile && (
        <div className="torrent-progress-file truncate">{progress.currentFile}</div>
      )}
      {progress.totalBytes > 0 && (
        <div className="torrent-progress-bytes">
          {formatBytes(progress.bytesDone)} / {formatBytes(progress.totalBytes)}
        </div>
      )}
    </div>
  )
}

function InfoRow({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="info-row">
      <span className="info-label">{label}</span>
      <span className={`info-value ${mono ? 'font-mono text-xs' : ''}`}>{value}</span>
    </div>
  )
}
