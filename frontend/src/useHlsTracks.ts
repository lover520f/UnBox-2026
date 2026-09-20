import { Events } from 'hls.js'
import type Hls from 'hls.js'
import { reactive } from 'vue'

export interface TrackItem { index: number; label: string }
export interface TrackState {
  levels: TrackItem[]
  currentLevel: number
  audioTracks: TrackItem[]
  currentAudio: number
  subtitleTracks: TrackItem[]
  currentSubtitle: number
  selectLevel(i: number): void
  selectAudio(i: number): void
  selectSubtitle(i: number): void
  detach(): void
}

// levelLabel 生成清晰度文案：优先真实分辨率（宽×高），其次高度、名称，
// 最后才退回码率。hls.js 的 level 有时只给 width、bitrate 也可能是 0，
// 直接按“码率 0k”显示会让用户误以为流有问题。
function levelLabel(l: any, i: number): string {
  const width = Number(l?.width) || 0
  const height = Number(l?.height) || 0
  if (width && height) return `${width}×${height}`
  if (height) return `${height}p`
  if (width) return `${width}×?`
  if (l?.name) return String(l.name)
  const kbps = Math.round((Number(l?.bitrate) || 0) / 1000)
  return kbps > 0 ? `码率 ${kbps}k` : `清晰度${i + 1}`
}

export function useHlsTracks(hls: Hls): TrackState {
  const state = reactive({
    levels: [{ index: -1, label: '自动' }] as TrackItem[],
    currentLevel: -1,
    audioTracks: [] as TrackItem[],
    currentAudio: -1,
    subtitleTracks: [] as TrackItem[],
    currentSubtitle: -1,
  })

  function refreshLevels() {
    state.levels = [{ index: -1, label: '自动' }, ...hls.levels.map((l, i) => ({ index: i, label: levelLabel(l, i) }))]
    state.currentLevel = hls.currentLevel
  }
  function refreshAudio() {
    state.audioTracks = hls.audioTracks.map((t, i) => ({ index: i, label: t.name || t.lang || `音轨${i + 1}` }))
    state.currentAudio = hls.audioTrack
  }
  function refreshSubtitle() {
    state.subtitleTracks = hls.subtitleTracks.map((t, i) => ({ index: i, label: t.name || t.lang || `字幕${i + 1}` }))
    state.currentSubtitle = hls.subtitleTrack
  }

  function onManifest() { refreshLevels(); refreshAudio(); refreshSubtitle() }
  function onAudioTracks() { refreshAudio() }
  function onSubtitleTracks() { refreshSubtitle() }
  function onLevelSwitched() { state.currentLevel = hls.currentLevel }
  function onAudioSwitched() { state.currentAudio = hls.audioTrack }
  function onSubtitleSwitched() { state.currentSubtitle = hls.subtitleTrack }

  hls.on(Events.MANIFEST_PARSED, onManifest)
  hls.on(Events.AUDIO_TRACKS_UPDATED, onAudioTracks)
  hls.on(Events.SUBTITLE_TRACKS_UPDATED, onSubtitleTracks)
  hls.on(Events.LEVEL_SWITCHED, onLevelSwitched)
  hls.on(Events.AUDIO_TRACK_SWITCHED, onAudioSwitched)
  hls.on(Events.SUBTITLE_TRACK_SWITCH, onSubtitleSwitched)

  return Object.assign(state, {
    selectLevel(i: number) { hls.currentLevel = i; state.currentLevel = i },
    selectAudio(i: number) { hls.audioTrack = i; state.currentAudio = i },
    selectSubtitle(i: number) { hls.subtitleTrack = i; state.currentSubtitle = i },
    detach() {
      hls.off(Events.MANIFEST_PARSED, onManifest)
      hls.off(Events.AUDIO_TRACKS_UPDATED, onAudioTracks)
      hls.off(Events.SUBTITLE_TRACKS_UPDATED, onSubtitleTracks)
      hls.off(Events.LEVEL_SWITCHED, onLevelSwitched)
      hls.off(Events.AUDIO_TRACK_SWITCHED, onAudioSwitched)
      hls.off(Events.SUBTITLE_TRACK_SWITCH, onSubtitleSwitched)
    },
  })
}
