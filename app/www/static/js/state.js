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
  // 排序状态
  currentSort: JSON.parse(localStorage.getItem('2fmusic_state') || '{}').currentSort || 'title',
  sortOrder: JSON.parse(localStorage.getItem('2fmusic_state') || '{}').sortOrder || 'asc',
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

export function persistState(audio, sortState = {}) {
  const { playQueue, currentTrackIndex, playMode, currentTab, selectedPlaylistId, currentSort, sortOrder } = state;
  const currentSong = playQueue[currentTrackIndex];
  
  // 如果有排序状态需要保存，即使当前歌曲是外部文件，也保存排序状态
  if (sortState.currentSort || sortState.sortOrder) {
    // 获取当前保存的状态
    const savedState = JSON.parse(localStorage.getItem('2fmusic_state') || '{}');
    
    // 更新排序状态
    const updatedState = {
      ...savedState,
      currentSort: sortState.currentSort || currentSort || 'title',
      sortOrder: sortState.sortOrder || sortOrder || 'asc'
    };
    
    // 保存更新后的状态
    localStorage.setItem('2fmusic_state', JSON.stringify(updatedState));
    return;
  }
  
  // 如果没有排序状态需要保存，并且当前歌曲是外部文件，则不保存任何状态
  if (currentSong && currentSong.isExternal) return;

  const nextState = {
    volume: audio?.volume ?? 1,
    playMode,
    currentTime: audio?.currentTime ?? 0,
    currentFilename: currentSong ? currentSong.filename : null,
    tab: currentTab,
    selectedPlaylistId,
    isFullScreen: ui.overlay ? ui.overlay.classList.contains('active') : false,
    // 保存排序状态
    currentSort: currentSort || 'title',
    sortOrder: sortOrder || 'asc'
  };
  localStorage.setItem('2fmusic_state', JSON.stringify(nextState));
}

export function saveFavorites() {
  localStorage.setItem('2fmusic_favs', JSON.stringify([...state.favorites]));
}

export function savePlaylist() {
  localStorage.setItem('2fmusic_playlist', JSON.stringify(state.fullPlaylist));
}