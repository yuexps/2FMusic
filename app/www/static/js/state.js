import { ui } from './ui.js';

// 状态集中管理
export const state = {
  fullPlaylist: JSON.parse(localStorage.getItem('2fmusic_playlist') || '[]'),
  displayPlaylist: [],
  playQueue: [],
  currentTrackIndex: 0,
  isPlaying: false,
  playMode: 0,
  lyricsData: [],
  currentFetchId: 0,
  favorites: new Set(JSON.parse(localStorage.getItem('2fmusic_favs') || '[]')),
  savedState: JSON.parse(localStorage.getItem('2fmusic_state') || '{}'),
  currentTab: JSON.parse(localStorage.getItem('2fmusic_state') || '{}').tab || 'local',
  neteaseResults: [],
  neteaseRecommendations: [],
  neteaseResultSource: 'recommend',
  neteasePollingTimer: null,
  currentLoginKey: null,
  neteaseDownloadDir: '',
  neteaseApiBase: '',
  neteaseSelected: new Set(),
  neteaseUser: JSON.parse(localStorage.getItem('2fmusic_netease_user') || 'null'),
  neteaseDownloadTasks: [],
  neteasePendingQueue: [],
  neteaseQueueToastShown: false,
  neteaseMaxConcurrent: 5,
  isPolling: false,
  progressToastEl: null,
  currentConfirmAction: null,
  libraryVersion: 0,
};

export function persistState(audio) {
  const { playQueue, currentTrackIndex, playMode, currentTab } = state;
  const currentSong = playQueue[currentTrackIndex];
  if (currentSong && currentSong.isExternal) return;

  const nextState = {
    volume: audio?.volume ?? 1,
    playMode,
    currentTime: audio?.currentTime ?? 0,
    currentFilename: currentSong ? currentSong.filename : null,
    tab: currentTab,
    isFullScreen: ui.overlay ? ui.overlay.classList.contains('active') : false
  };
  localStorage.setItem('2fmusic_state', JSON.stringify(nextState));
}

export function saveFavorites() {
  localStorage.setItem('2fmusic_favs', JSON.stringify([...state.favorites]));
}

export function savePlaylist() {
  localStorage.setItem('2fmusic_playlist', JSON.stringify(state.fullPlaylist));
}
