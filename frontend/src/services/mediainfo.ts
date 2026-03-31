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

// ---------------------------------------------------------------------------
// CLI-style formatter
// ---------------------------------------------------------------------------

function fmtField(label: string, value: string): string {
  if (!value) return ''
  const pad = 40
  const labelPadded = label.padEnd(pad, ' ')
  return `${labelPadded}: ${value}\n`
}

function fmtBytes(bytes: number): string {
  if (!bytes) return ''
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB']
  let v = bytes
  let i = 0
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++ }
  return `${v.toFixed(i > 0 ? 2 : 0)} ${units[i]} (${bytes.toLocaleString('en-US')} bytes)`
}

function fmtDuration(seconds: number): string {
  if (!seconds) return ''
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  const ms = Math.round((seconds % 1) * 1000)
  const parts = []
  if (h) parts.push(`${h} h`)
  if (m) parts.push(`${m} min`)
  if (s || ms) parts.push(`${s} s ${String(ms).padStart(3, '0')} ms`)
  return parts.join(' ')
}

function fmtBitrate(bps: number): string {
  if (!bps) return ''
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} Mb/s`
  if (bps >= 1_000) return `${(bps / 1_000).toFixed(0)} kb/s`
  return `${bps} b/s`
}

function buildGeneralSection(g: any): string {
  let out = 'General\n'
  out += fmtField('Complete name', g.CompleteName ?? g.FileName ?? '')
  out += fmtField('Format', g.Format ?? '')
  out += fmtField('Format version', g.Format_Version ?? '')
  const fs = parseInt(g.FileSize ?? '0')
  if (fs) out += fmtField('File size', fmtBytes(fs))
  const dur = parseFloat(g.Duration ?? '0')
  if (dur) out += fmtField('Duration', fmtDuration(dur))
  out += fmtField('Overall bit rate mode', g.OverallBitRate_Mode ?? '')
  const br = parseInt(g.OverallBitRate ?? '0')
  if (br) out += fmtField('Overall bit rate', fmtBitrate(br))
  out += fmtField('Frame rate', g.FrameRate ? `${parseFloat(g.FrameRate).toFixed(3)} FPS` : '')
  out += fmtField('Writing application', g.Encoded_Application ?? '')
  out += fmtField('Writing library', g.Encoded_Library ?? '')
  return out
}

function buildVideoSection(v: any, idx: number): string {
  const label = idx > 0 ? `Video #${idx + 1}` : 'Video'
  let out = `\n${label}\n`
  out += fmtField('ID', v.ID ?? '')
  out += fmtField('Format', v.Format ?? '')
  out += fmtField('Format/Info', v.Format_Info ?? '')
  out += fmtField('Format profile', v.Format_Profile ?? '')
  out += fmtField('HDR format', v.HDR_Format ?? '')
  out += fmtField('HDR format compatibility', v.HDR_Format_Compatibility ?? '')
  out += fmtField('Codec ID', v.CodecID ?? '')
  const dur = parseFloat(v.Duration ?? '0')
  if (dur) out += fmtField('Duration', fmtDuration(dur))
  const br = parseInt(v.BitRate ?? '0')
  if (br) out += fmtField('Bit rate', fmtBitrate(br))
  const w = parseInt(v.Width ?? '0')
  const h = parseInt(v.Height ?? '0')
  if (w) out += fmtField('Width', `${w.toLocaleString('en-US')} pixels`)
  if (h) out += fmtField('Height', `${h.toLocaleString('en-US')} pixels`)
  out += fmtField('Display aspect ratio', v.DisplayAspectRatio_String ?? v.DisplayAspectRatio ?? '')
  out += fmtField('Frame rate mode', v.FrameRate_Mode ?? '')
  if (v.FrameRate) {
    const fps = parseFloat(v.FrameRate)
    const orig = v.FrameRate_Original ? ` (${v.FrameRate_Original})` : ''
    out += fmtField('Frame rate', `${fps.toFixed(3)}${orig} FPS`)
  }
  out += fmtField('Color space', v.ColorSpace ?? '')
  out += fmtField('Chroma subsampling', v.ChromaSubsampling ?? '')
  out += fmtField('Bit depth', v.BitDepth ? `${v.BitDepth} bits` : '')
  out += fmtField('Scan type', v.ScanType ?? '')
  out += fmtField('Writing library', v.Encoded_Library ?? '')
  return out
}

function buildAudioSection(a: any, idx: number): string {
  const label = idx > 0 ? `Audio #${idx + 1}` : 'Audio'
  let out = `\n${label}\n`
  out += fmtField('ID', a.ID ?? '')
  out += fmtField('Format', a.Format ?? '')
  out += fmtField('Format profile', a.Format_Profile ?? '')
  out += fmtField('Format settings', a.Format_Settings ?? '')
  out += fmtField('Codec ID', a.CodecID ?? '')
  const dur = parseFloat(a.Duration ?? '0')
  if (dur) out += fmtField('Duration', fmtDuration(dur))
  out += fmtField('Bit rate mode', a.BitRate_Mode ?? '')
  const br = parseInt(a.BitRate ?? '0')
  if (br) out += fmtField('Bit rate', fmtBitrate(br))
  out += fmtField('Channel(s)', a.Channels ? `${a.Channels} channel${parseInt(a.Channels) > 1 ? 's' : ''}` : '')
  out += fmtField('Channel layout', a.ChannelLayout ?? '')
  out += fmtField('Sampling rate', a.SamplingRate ? `${(parseInt(a.SamplingRate) / 1000).toFixed(1)} kHz` : '')
  out += fmtField('Frame rate', a.FrameRate ? `${parseFloat(a.FrameRate).toFixed(3)} FPS` : '')
  out += fmtField('Bit depth', a.BitDepth ? `${a.BitDepth} bits` : '')
  out += fmtField('Compression mode', a.Compression_Mode ?? '')
  out += fmtField('Language', (a.Language_String ?? normalizeLang(a.Language ?? '')) || '')
  out += fmtField('Default', a.Default ?? '')
  out += fmtField('Forced', a.Forced ?? '')
  return out
}

function buildTextSection(t: any, idx: number): string {
  const label = idx > 0 ? `Text #${idx + 1}` : 'Text'
  let out = `\n${label}\n`
  out += fmtField('ID', t.ID ?? '')
  out += fmtField('Format', t.Format ?? '')
  out += fmtField('Codec ID', t.CodecID ?? '')
  out += fmtField('Language', (t.Language_String ?? normalizeLang(t.Language ?? '')) || '')
  out += fmtField('Default', t.Default ?? '')
  out += fmtField('Forced', t.Forced ?? '')
  return out
}

export async function getMediaInfoCLIText(filePath: string): Promise<string> {
  const result = await analyzeFile(filePath)
  const tracks: any[] = result?.media?.track ?? []

  const general = tracks.find((t: any) => t['@type'] === 'General') ?? {}
  const videoTracks = tracks.filter((t: any) => t['@type'] === 'Video')
  const audioTracks = tracks.filter((t: any) => t['@type'] === 'Audio')
  const textTracks = tracks.filter((t: any) => t['@type'] === 'Text')

  let out = buildGeneralSection(general)
  videoTracks.forEach((v, i) => { out += buildVideoSection(v, i) })
  audioTracks.forEach((a, i) => { out += buildAudioSection(a, i) })
  textTracks.forEach((t, i) => { out += buildTextSection(t, i) })

  return out.trimEnd() + '\n'
}

// ---------------------------------------------------------------------------

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
