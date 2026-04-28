import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import {
  CreateTorrent, SearchTMDB, GetTMDBDetails, GenerateNFO, SaveNFO,
  UploadTorrent, DownloadTorrent, LargestVideoFile,
  AppLoadSettings, GenerateBBCode, CheckDuplicate, SaveHistoryEntry,
  GetCategories, GetLastTMDBSource,
} from '../../wailsjs/go/main/App'
import { getMediaInfoJS, getMediaInfoCLIText } from '../services/mediainfo'
import { OnFileDrop, OnFileDropOff, EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'
import type { main } from '../../wailsjs/go/models'
import type { ItemOverrides } from '../components/ItemEditor'

export type QueueItemStatus =
  | 'analyzing'   // analyse auto en cours
  | 'review'      // analyse terminée, attend validation
  | 'ready'       // validé, prêt à traiter
  | 'processing'  // traitement en cours
  | 'done'        // terminé
  | 'error'       // erreur

export interface AnalysisResult {
  releaseName: string
  mediaInfo: main.MediaInfo
  mediaInfoCLI: string
  tmdb: main.TMDBDetails  // vide si aucun match
  tmdbType: string
  tmdbSource: string  // 'proxy' | 'direct' | 'none'
  categoryId: number
  categoryName: string
  bbcodeDescription: string
}

export interface QueueItem {
  id: string
  path: string
  name: string
  status: QueueItemStatus
  step?: string
  progress?: number  // 0..100 pendant le hashing
  error?: string
  uploadResult?: main.UploadResponse
  duplicateWarning?: { id: number; name: string; url: string }
  overrides?: ItemOverrides
  analysis?: AnalysisResult
}

interface UploadQueueContextValue {
  queue: QueueItem[]
  running: boolean
  dragging: boolean
  categories: { id: number; name: string }[]
  setDragging: (v: boolean) => void
  addPaths: (paths: string[]) => void
  updateItem: (id: string, patch: Partial<QueueItem>) => void
  validate: (id: string) => void
  unvalidate: (id: string) => void
  validateAll: () => void
  remove: (id: string) => void
  clearDone: () => void
  clearAll: () => void
  start: () => Promise<void>
  stop: () => void
  reanalyze: (id: string) => void
}

const UploadQueueContext = createContext<UploadQueueContextValue | null>(null)

let _queueIdCounter = 0
function makeQueueItem(path: string): QueueItem {
  return {
    id: String(++_queueIdCounter),
    path,
    name: path.split(/[/\\]/).pop() ?? path,
    status: 'analyzing',
  }
}

const DEFAULT_CATS = [
  { id: 1, name: 'Films' }, { id: 2, name: 'Séries' },
  { id: 3, name: 'Documentaires' }, { id: 4, name: 'Animés' },
]

export function UploadQueueProvider({ children }: { children: React.ReactNode }) {
  const [queue, setQueue] = useState<QueueItem[]>([])
  const [running, setRunning] = useState(false)
  const [dragging, setDragging] = useState(false)
  const [categories, setCategories] = useState(DEFAULT_CATS)
  const queueRef = useRef<QueueItem[]>([])
  const runningRef = useRef(false)
  const currentItemIdRef = useRef<string | null>(null)

  useEffect(() => { queueRef.current = queue }, [queue])

  useEffect(() => {
    GetCategories().then(c => { if (c?.length) setCategories(c) }).catch(() => {})
  }, [])

  const updateItem = useCallback((id: string, patch: Partial<QueueItem>) => {
    setQueue(q => q.map(it => it.id === id ? { ...it, ...patch } : it))
  }, [])

  // ─── Analyse automatique ────────────────────────────────────────────────
  const analyzeItem = useCallback(async (id: string, path: string, fallbackName: string) => {
    try {
      // Media info
      let mediaInfo: main.MediaInfo = {} as main.MediaInfo
      let mediaInfoCLI = ''
      try {
        const videoPath = await LargestVideoFile(path)
        const parsed = await getMediaInfoJS(videoPath)
        mediaInfo = parsed as any
        try { mediaInfoCLI = await getMediaInfoCLIText(videoPath) } catch { /* */ }
      } catch { /* */ }

      // Determine release name without extension
      const releaseName = fallbackName.replace(/\.(mkv|mp4|avi|mov|ts|m2ts|wmv)$/i, '')

      // TMDB auto-search
      let tmdb: main.TMDBDetails = {} as main.TMDBDetails
      let tmdbType = 'movie'
      let tmdbSource = 'none'
      try {
        const results = await SearchTMDB(releaseName, '')
        tmdbSource = await GetLastTMDBSource().catch(() => 'none')
        if (results && results.length > 0) {
          tmdb = await GetTMDBDetails(results[0].id, results[0].mediaType)
          tmdbType = results[0].mediaType
        }
      } catch { /* */ }

      const categoryId = tmdbType === 'tv' ? 2 : 1
      const category = categories.find(c => c.id === categoryId)?.name
        || DEFAULT_CATS.find(c => c.id === categoryId)?.name
        || ''

      // BBCode description
      const bbcode = await GenerateBBCode(releaseName, mediaInfoCLI).catch(() => '')
      const description = bbcode || tmdb.overview || ''

      updateItem(id, {
        status: 'review',
        analysis: {
          releaseName,
          mediaInfo, mediaInfoCLI,
          tmdb, tmdbType, tmdbSource,
          categoryId, categoryName: category,
          bbcodeDescription: description,
        },
      })

      // Duplicate pre-check
      CheckDuplicate(releaseName).then(dup => {
        if (dup && dup.found) {
          updateItem(id, {
            duplicateWarning: { id: dup.id || 0, name: dup.name || '', url: dup.url || '' },
          })
        }
      }).catch(() => {})
    } catch (e) {
      updateItem(id, { status: 'error', error: 'Analyse échouée : ' + String(e) })
    }
  }, [categories, updateItem])

  const addPaths = useCallback((paths: string[]) => {
    const existing = new Set(queueRef.current.map(i => i.path))
    const fresh = paths.filter(p => p && !existing.has(p)).map(makeQueueItem)
    if (fresh.length === 0) return
    setQueue(q => [...q, ...fresh])
    // Lance l'analyse en parallèle pour chaque nouvel item
    fresh.forEach(item => { analyzeItem(item.id, item.path, item.name) })
  }, [analyzeItem])

  const reanalyze = useCallback((id: string) => {
    const item = queueRef.current.find(i => i.id === id)
    if (!item) return
    updateItem(id, { status: 'analyzing', analysis: undefined, overrides: undefined })
    analyzeItem(id, item.path, item.name)
  }, [analyzeItem, updateItem])

  // ─── Validation ─────────────────────────────────────────────────────────
  const validate = useCallback((id: string) => {
    updateItem(id, { status: 'ready' })
  }, [updateItem])

  const unvalidate = useCallback((id: string) => {
    updateItem(id, { status: 'review' })
  }, [updateItem])

  const validateAll = useCallback(() => {
    setQueue(q => q.map(it => it.status === 'review' ? { ...it, status: 'ready' } : it))
  }, [])

  // Global file drop
  useEffect(() => {
    OnFileDrop((_x, _y, paths) => {
      if (paths.length === 0) return
      addPaths(paths)
      setDragging(false)
    }, true)
    return () => OnFileDropOff()
  }, [addPaths])

  // Suivi de la progression du hashing (event émis par CreateTorrent côté Go)
  useEffect(() => {
    EventsOn('torrent:progress', (data: any) => {
      const id = currentItemIdRef.current
      if (!id) return
      const pct = typeof data?.percent === 'number' ? data.percent : 0
      const phase = data?.phase as string
      let step = 'Création du torrent…'
      if (phase === 'hashing') {
        step = `Hashing ${pct.toFixed(1)}%`
      } else if (phase === 'writing') {
        step = 'Écriture du torrent…'
      }
      updateItem(id, { step, progress: pct })
    })
    return () => EventsOff('torrent:progress')
  }, [updateItem])

  // ─── Traitement ─────────────────────────────────────────────────────────
  const processItem = async (item: QueueItem, nfoMode: string) => {
    const upd = (patch: Partial<QueueItem>) => updateItem(item.id, patch)
    currentItemIdRef.current = item.id
    upd({ status: 'processing', error: undefined, step: 'Création du torrent…', progress: 0 })

    const ov = item.overrides || {}
    const a = item.analysis
    if (!a) { upd({ status: 'error', error: 'Analyse manquante' }); return }

    const releaseName = ov.name || a.releaseName
    const catId = ov.categoryId ?? a.categoryId
    const tmdbType = ov.tmdbType || a.tmdbType
    let tmdb: main.TMDBDetails = a.tmdb

    try {
      // If user picked a different TMDB via override, fetch its details
      if (ov.tmdbId && ov.tmdbId > 0 && ov.tmdbId !== a.tmdb?.id) {
        try { tmdb = await GetTMDBDetails(ov.tmdbId, tmdbType) } catch { /* */ }
      }

      const torrent = await CreateTorrent(item.path)
      upd({ name: releaseName, step: 'Génération NFO…' })

      let nfo = ''
      if (nfoMode === 'mediainfo' && a.mediaInfoCLI) {
        nfo = a.mediaInfoCLI
      } else {
        nfo = await GenerateNFO(tmdb, a.mediaInfo, a.mediaInfoCLI)
      }
      try { await SaveNFO(nfo, releaseName) } catch { /* */ }

      upd({ step: 'Vérification doublon…' })
      const dup = await CheckDuplicate(releaseName).catch(() => null)
      if (dup && dup.found) {
        throw new Error(`Doublon sur nexum : ${dup.name} (ID #${dup.id})`)
      }

      upd({ step: 'Upload…' })
      const desc = ov.description || a.bbcodeDescription

      const result = await UploadTorrent({
        torrentPath: torrent.filePath,
        nfoContent: nfo,
        name: releaseName,
        categoryId: catId,
        description: desc,
        tmdbId: tmdb?.id || 0,
        tmdbType,
        resolution: (a.mediaInfo as any).resolution || '',
        videoCodec: (a.mediaInfo as any).videoCodec || '',
        audioCodec: (a.mediaInfo as any).audioCodec || '',
        audioLanguages: (a.mediaInfo as any).audioLanguages || '',
        subtitleLanguages: (a.mediaInfo as any).subtitleLanguages || '',
        hdrFormat: (a.mediaInfo as any).hdrFormat || '',
        source: (a.mediaInfo as any).source || '',
      })

      if (result.torrent_id) {
        try { await DownloadTorrent(result.torrent_id) } catch { /* */ }
      }

      SaveHistoryEntry({
        id: 0, createdAt: new Date().toISOString(), sourcePath: item.path,
        releaseName, torrentPath: torrent.filePath, nfoPath: '',
        infoHash: torrent.infoHash, size: torrent.size,
        categoryId: catId, categoryName: a.categoryName,
        tmdbId: tmdb?.id || 0, tmdbType, tmdbTitle: tmdb?.title || '',
        uploadUrl: result.url, uploadId: result.torrent_id,
        status: 'done', errorMsg: '', noUpload: false,
      } as main.HistoryEntry).catch(e => console.error('SaveHistoryEntry failed:', e))

      currentItemIdRef.current = null
      upd({ status: 'done', step: undefined, progress: undefined, uploadResult: result, name: releaseName })
    } catch (e) {
      currentItemIdRef.current = null
      SaveHistoryEntry({
        id: 0, createdAt: new Date().toISOString(), sourcePath: item.path,
        releaseName, torrentPath: '', nfoPath: '',
        infoHash: '', size: 0, categoryId: catId, categoryName: a.categoryName,
        tmdbId: tmdb?.id || 0, tmdbType, tmdbTitle: tmdb?.title || '',
        uploadUrl: '', uploadId: 0,
        status: 'error', errorMsg: String(e), noUpload: false,
      } as main.HistoryEntry).catch(e => console.error('SaveHistoryEntry failed:', e))
      upd({ status: 'error', step: undefined, progress: undefined, error: String(e) })
    }
  }

  const start = async () => {
    if (runningRef.current) return
    runningRef.current = true
    setRunning(true)
    const settings = await AppLoadSettings().catch(() => ({ nfoMode: 'nfo' } as any))
    while (runningRef.current) {
      const next = queueRef.current.find(i => i.status === 'ready')
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

  const remove = (id: string) => setQueue(q => q.filter(i => i.id !== id))
  const clearDone = () => setQueue(q => q.filter(i => i.status !== 'done'))
  const clearAll = () => setQueue([])

  return (
    <UploadQueueContext.Provider value={{
      queue, running, dragging, categories, setDragging,
      addPaths, updateItem, validate, unvalidate, validateAll,
      remove, clearDone, clearAll, start, stop, reanalyze,
    }}>
      {children}
    </UploadQueueContext.Provider>
  )
}

export function useUploadQueue(): UploadQueueContextValue {
  const ctx = useContext(UploadQueueContext)
  if (!ctx) throw new Error('useUploadQueue must be used within UploadQueueProvider')
  return ctx
}
