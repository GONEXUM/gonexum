import MediaInfoFactory from 'mediainfo.js'
import { GetFileSize, ReadFileChunk } from '../../wailsjs/go/main/App'

export interface ParsedMediaInfo {
  resolution: string
  videoCodec: string
  audioCodec: string
  audioLanguages: string
  hdrFormat: string
  source: string
  duration: string
  fileSize: number
  width: number
  height: number
  bitrate: number
  frameRate: number
}

const CHUNK_SIZE = 256 * 1024 // 256 KB

async function analyzeFile(filePath: string): Promise<any> {
  const fileSize = await GetFileSize(filePath)

  const mi = await MediaInfoFactory({ format: 'object', locateFile: () => 'mediainfo.wasm' })

  const readChunk = async (size: number, offset: number): Promise<Uint8Array> => {
    const b64 = await ReadFileChunk(filePath, offset, size)
    const bin = atob(b64)
    const arr = new Uint8Array(bin.length)
    for (let i = 0; i < bin.length; i++) arr[i] = bin.charCodeAt(i)
    return arr
  }

  const result = await mi.analyzeData(() => fileSize, readChunk)
  mi.close()
  return result
}

function pickBestAudioCodec(tracks: any[]): string {
  const priority: Record<string, number> = {
    'TrueHD Atmos': 10, 'TrueHD': 9,
    'DTS-HD MA': 8, 'DTS-HD': 7, 'DTS': 6,
    'E-AC-3': 5, 'AC-3': 4, 'FLAC': 3, 'AAC': 2, 'MP3': 1,
  }
  let best = ''
  let bestPrio = -1
  for (const t of tracks) {
    const codec = t.Format || ''
    const addl = t.Format_AdditionalFeatures || ''
    let name = codec
    if (codec === 'TrueHD' && addl.includes('Atmos')) name = 'TrueHD Atmos'
    else if (codec === 'MLP FBA' && addl.includes('Atmos')) name = 'TrueHD Atmos'
    else if (codec === 'DTS' && t.Format_Profile?.includes('MA')) name = 'DTS-HD MA'
    else if (codec === 'DTS' && t.Format_Profile?.includes('HRA')) name = 'DTS-HD'
    const prio = priority[name] ?? 0
    if (prio > bestPrio) { bestPrio = prio; best = name }
  }
  return normalizeAudioCodec(best)
}

function normalizeAudioCodec(codec: string): string {
  const map: Record<string, string> = {
    'TrueHD Atmos': 'Atmos', 'TrueHD': 'TrueHD',
    'DTS-HD MA': 'DTS-HD MA', 'DTS-HD': 'DTS-HD', 'DTS': 'DTS',
    'E-AC-3': 'EAC3', 'AC-3': 'AC3', 'FLAC': 'FLAC', 'AAC': 'AAC', 'MP3': 'MP3',
  }
  return map[codec] ?? codec
}

const langNames: Record<string, string> = {
  fr: 'Français', fre: 'Français', fra: 'Français',
  en: 'Anglais', eng: 'Anglais',
  de: 'Allemand', ger: 'Allemand', deu: 'Allemand',
  es: 'Espagnol', spa: 'Espagnol',
  it: 'Italien', ita: 'Italien',
  ja: 'Japonais', jpn: 'Japonais',
  pt: 'Portugais', por: 'Portugais',
  zh: 'Chinois', chi: 'Chinois', zho: 'Chinois',
  ko: 'Coréen', kor: 'Coréen',
  ru: 'Russe', rus: 'Russe',
  ar: 'Arabe', ara: 'Arabe',
}

function normalizeLang(lang: string): string {
  return langNames[lang?.toLowerCase()] ?? lang ?? ''
}

function detectHDR(videoTrack: any): string {
  const transfer = videoTrack.transfer_characteristics_Original || videoTrack.transfer_characteristics || ''
  const hdrFormat = videoTrack.HDR_Format || ''
  const hdrCompat = videoTrack.HDR_Format_Compatibility || ''

  const isDV = hdrFormat.toLowerCase().includes('dolby vision')
  const isHDR10Plus = hdrFormat.toLowerCase().includes('hdr10+') || hdrCompat.toLowerCase().includes('hdr10+')
  const isHDR10 = transfer.includes('PQ') || transfer.includes('SMPTE ST 2084')
  const isHLG = transfer.includes('HLG') || transfer.includes('ARIB STD B67')

  if (isDV && isHDR10) return 'HDR DV'
  if (isDV) return 'DV'
  if (isHDR10Plus) return 'HDR10+'
  if (isHDR10) return 'HDR10'
  if (isHLG) return 'HDR'
  return ''
}

function normalizeVideoCodec(format: string, profile: string): string {
  const f = format?.toLowerCase() ?? ''
  if (f.includes('hevc') || f === 'h.265') return 'H.265'
  if (f.includes('avc') || f === 'h.264') return 'H.264'
  if (f === 'av1') return 'AV1'
  if (f === 'vp9') return 'VP9'
  if (f.includes('xvid') || f.includes('divx') || f.includes('mpeg-4')) return 'XviD'
  return format ?? ''
}

function detectResolution(w: number, h: number): string {
  if (h >= 2160 || w >= 3840) return '2160p'
  if (h >= 1080 || w >= 1920) return '1080p'
  if (h >= 720 || w >= 1280) return '720p'
  if (w > 0 || h > 0) return 'SD'
  return ''
}

function guessSource(filePath: string): string {
  const name = filePath.toLowerCase()
  if (name.includes('bluray') || name.includes('blu-ray') || name.includes('bdremux') || name.includes('bdrip')) return 'BluRay'
  if (name.includes('web-dl') || name.includes('webdl')) return 'WEB-DL'
  if (name.includes('webrip')) return 'WEBRip'
  if (name.includes('hdtv')) return 'HDTV'
  if (name.includes('dvdrip') || name.includes('dvd')) return 'DVDRip'
  if (name.includes('dcp')) return 'DCP'
  return ''
}

export async function getMediaInfoJS(filePath: string): Promise<ParsedMediaInfo> {
  const result = await analyzeFile(filePath)
  const tracks: any[] = result?.media?.track ?? []

  const general = tracks.find((t: any) => t['@type'] === 'General') ?? {}
  const video = tracks.find((t: any) => t['@type'] === 'Video') ?? {}
  const audioTracks = tracks.filter((t: any) => t['@type'] === 'Audio')

  const w = parseInt(video.Width ?? '0')
  const h = parseInt(video.Height ?? '0')

  // Duration
  let duration = ''
  const dur = parseFloat(general.Duration ?? '0')
  if (dur > 0) {
    const h_ = Math.floor(dur / 3600)
    const m = Math.floor((dur % 3600) / 60)
    const s = Math.floor(dur % 60)
    duration = `${String(h_).padStart(2, '0')}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  }

  // Audio languages
  const seen = new Set<string>()
  const langs: string[] = []
  for (const t of audioTracks) {
    const lang = normalizeLang(t.Language ?? '')
    if (lang && !seen.has(lang)) { seen.add(lang); langs.push(lang) }
  }

  // Frame rate
  const fps = parseFloat(video.FrameRate ?? '0')

  return {
    resolution: detectResolution(w, h),
    videoCodec: normalizeVideoCodec(video.Format ?? '', video.Format_Profile ?? ''),
    audioCodec: pickBestAudioCodec(audioTracks),
    audioLanguages: langs.join(', '),
    hdrFormat: detectHDR(video),
    source: guessSource(filePath),
    duration,
    fileSize: parseInt(general.FileSize ?? '0'),
    width: w,
    height: h,
    bitrate: parseInt(general.OverallBitRate ?? '0'),
    frameRate: Math.round(fps * 100) / 100,
  }
}
