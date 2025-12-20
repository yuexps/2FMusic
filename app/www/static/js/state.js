import { ui } from './ui.js';

const cachedNeteaseUser = JSON.parse(localStorage.getItem('2fmusic_netease_user') || 'null');

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
  selectedPlaylistId: JSON.parse(localStorage.getItem('2fmusic_state') || '{}').selectedPlaylistId || null,
  // 收藏夹缓存
  cachedPlaylists: JSON.parse(localStorage.getItem('2fmusic_cached_playlists') || '[]'),
  cachedPlaylistsTime: parseInt(localStorage.getItem('2fmusic_cached_playlists_time') || '0'),
  // 收藏夹歌曲缓存，key为playlistId，value为歌曲ID数组
  cachedPlaylistSongs: JSON.parse(localStorage.getItem('2fmusic_cached_playlist_songs') || '{}'),
  neteaseResults: [],
  neteaseRecommendations: [],
  neteaseResultSource: 'recommend',
  neteasePollingTimer: null,
  currentLoginKey: null,
  neteaseDownloadDir: '',
  neteaseApiBase: '',
  neteaseSelected: new Set(),
  neteaseUser: cachedNeteaseUser,
  neteaseIsVip: cachedNeteaseUser?.isVip || false,
  neteaseDownloadTasks: [],
  neteasePendingQueue: [],
  neteaseQueueToastShown: false,
  neteaseMaxConcurrent: 20,
  isPolling: false,
  progressToastEl: null,
  currentConfirmAction: null,
  libraryVersion: 0,
};

// 保存收藏夹缓存
export function saveCachedPlaylists(playlists) {
  state.cachedPlaylists = playlists;
  state.cachedPlaylistsTime = Date.now();
  localStorage.setItem('2fmusic_cached_playlists', JSON.stringify(playlists));
  localStorage.setItem('2fmusic_cached_playlists_time', state.cachedPlaylistsTime.toString());
}

// 保存收藏夹歌曲缓存
export function saveCachedPlaylistSongs(playlistId, songs) {
  state.cachedPlaylistSongs[playlistId] = songs;
  localStorage.setItem('2fmusic_cached_playlist_songs', JSON.stringify(state.cachedPlaylistSongs));
}

export function persistState(audio) {
  const { playQueue, currentTrackIndex, playMode, currentTab, selectedPlaylistId } = state;
  const currentSong = playQueue[currentTrackIndex];
  if (currentSong && currentSong.isExternal) return;

  const nextState = {
    volume: audio?.volume ?? 1,
    playMode,
    currentTime: audio?.currentTime ?? 0,
    currentFilename: currentSong ? currentSong.filename : null,
    tab: currentTab,
    selectedPlaylistId,
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