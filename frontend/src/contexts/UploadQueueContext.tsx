import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import {
  CreateTorrent, SearchTMDB, GetTMDBDetails, GenerateNFO, SaveNFO,
  UploadTorrent, DownloadTorrent, LargestVideoFile,
  AppLoadSettings, GenerateBBCode, CheckDuplicate, SaveHistoryEntry,
} from '../../wailsjs/go/main/App'
import { getMediaInfoJS, getMediaInfoCLIText } from '../services/mediainfo'
import { OnFileDrop, OnFileDropOff } from '../../wailsjs/runtime/runtime'
import type { main } from '../../wailsjs/go/models'
import type { ItemOverrides } from '../components/ItemEditor'

export type QueueItemStatus = 'pending' | 'processing' | 'done' | 'error'

export interface QueueItem {
  id: string
  path: string
  name: string
  status: QueueItemStatus
  step?: string
  error?: string
  uploadResult?: main.UploadResponse
  duplicateWarning?: { id: number; name: string; url: string }
  overrides?: ItemOverrides
}

interface UploadQueueContextValue {
  queue: QueueItem[]
  running: boolean
  dragging: boolean
  setDragging: (v: boolean) => void
  addPaths: (paths: string[]) => void
  updateItem: (id: string, patch: Partial<QueueItem>) => void
  remove: (id: string) => void
  clearDone: () => void
  clearAll: () => void
  start: () => Promise<void>
  stop: () => void
}

const UploadQueueContext = createContext<UploadQueueContextValue | null>(null)

let _queueIdCounter = 0
function makeQueueItem(path: string): QueueItem {
  return {
    id: String(++_queueIdCounter),
    path,
    name: path.split(/[/\\]/).pop() ?? path,
    status: 'pending',
  }
}

export function UploadQueueProvider({ children }: { children: React.ReactNode }) {
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
    const fresh = paths.filter(p => p && !existing.has(p)).map(makeQueueItem)
    if (fresh.length === 0) return
    setQueue(q => [...q, ...fresh])
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

  // Global file drop — actif même hors de la page uploader
  useEffect(() => {
    OnFileDrop((_x, _y, paths) => {
      if (paths.length === 0) return
      addPaths(paths)
      setDragging(false)
    }, true)
    return () => OnFileDropOff()
  }, [addPaths])

  const processItem = async (item: QueueItem, nfoMode: string) => {
    const upd = (patch: Partial<QueueItem>) => updateItem(item.id, patch)
    upd({ status: 'processing', error: undefined, step: 'Création du torrent…' })

    const ov = item.overrides || {}
    let catId = ov.categoryId ?? 1
    try {
      const torrent = await CreateTorrent(item.path)
      const releaseName = ov.name || torrent.name
      upd({ name: releaseName, step: 'Analyse média…' })

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
      let tmdbType = ov.tmdbType || 'movie'
      if (ov.tmdbId && ov.tmdbId > 0) {
        try { tmdb = await GetTMDBDetails(ov.tmdbId, tmdbType) } catch { /* */ }
      } else {
        try {
          const results = await SearchTMDB(releaseName, '')
          if (results && results.length > 0) {
            const details = await GetTMDBDetails(results[0].id, results[0].mediaType)
            tmdb = details
            tmdbType = results[0].mediaType
            if (ov.categoryId === undefined) catId = tmdbType === 'tv' ? 2 : 1
          }
        } catch { /* non bloquant */ }
      }

      upd({ step: 'Génération NFO…' })
      let nfo = ''
      if (nfoMode === 'mediainfo' && cliText) {
        nfo = cliText
      } else {
        nfo = await GenerateNFO(tmdb, mi, cliText)
      }
      try { await SaveNFO(nfo, releaseName) } catch { /* */ }

      upd({ step: 'Vérification doublon…' })
      const dup = await CheckDuplicate(releaseName).catch(() => null)
      if (dup && dup.found) {
        throw new Error(`Doublon sur nexum : ${dup.name} (ID #${dup.id})`)
      }

      upd({ step: 'Upload…' })
      let desc = ov.description
      if (!desc) {
        desc = await GenerateBBCode(releaseName, cliText).catch(() => '') || tmdb.overview || ''
      }
      const result = await UploadTorrent({
        torrentPath: torrent.filePath,
        nfoContent: nfo,
        name: releaseName,
        categoryId: catId,
        description: desc,
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
        releaseName, torrentPath: torrent.filePath, nfoPath: '',
        infoHash: torrent.infoHash, size: torrent.size,
        categoryId: catId, categoryName: '',
        tmdbId: tmdb.id || 0, tmdbType, tmdbTitle: tmdb.title || '',
        uploadUrl: result.url, uploadId: result.torrent_id,
        status: 'done', errorMsg: '', noUpload: false,
      } as main.HistoryEntry).catch(() => {})

      upd({ status: 'done', step: undefined, uploadResult: result, name: releaseName })
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

  const remove = (id: string) => setQueue(q => q.filter(i => i.id !== id))
  const clearDone = () => setQueue(q => q.filter(i => i.status !== 'done'))
  const clearAll = () => setQueue([])

  return (
    <UploadQueueContext.Provider value={{
      queue, running, dragging, setDragging,
      addPaths, updateItem, remove, clearDone, clearAll, start, stop,
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
