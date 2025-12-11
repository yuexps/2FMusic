import { state, persistState, saveFavorites, savePlaylist } from './state.js';
import { ui } from './ui.js';
import { api } from './api.js';
import { showToast, showConfirmDialog, hideProgressToast, updateDetailFavButton, formatTime, renderNoLyrics, updateSliderFill, flyToElement, throttle, extractColorFromImage } from './utils.js';
import { startScanPolling, loadMountPoints } from './mounts.js';

// 收藏
function toggleFavorite(song, btnEl) {
  if (state.favorites.has(song.filename)) {
    state.favorites.delete(song.filename);
    if (btnEl) { btnEl.classList.remove('active'); btnEl.innerHTML = '<i class="far fa-heart"></i>'; }
  } else {
    state.favorites.add(song.filename);
    if (btnEl) { btnEl.classList.add('active'); btnEl.innerHTML = '<i class="fas fa-heart"></i>'; }
  }
  saveFavorites();
  const currentPlaying = state.playQueue[state.currentTrackIndex];
  if (currentPlaying && currentPlaying.filename === song.filename) {
    updateDetailFavButton(state.favorites.has(song.filename));
  }
  if (state.currentTab === 'fav' && !state.favorites.has(song.filename)) renderPlaylist();
  persistState(ui.audio);
}

const throttledPersist = throttle(() => persistState(ui.audio), 2000);

// Wake Lock Logic
let wakeLock = null;
async function requestWakeLock() {
  if ('wakeLock' in navigator) {
    try {
      if (document.visibilityState === 'visible') {
        wakeLock = await navigator.wakeLock.request('screen');
        console.log('Wake Lock active');
      }
    } catch (err) {
      console.warn('Wake Lock failed:', err);
    }
  }
}
function releaseWakeLock() {
  if (wakeLock) {
    wakeLock.release().then(() => { wakeLock = null; console.log('Wake Lock released'); });
  }
}

export async function loadSongs(retry = true) {
  // 1. 优先使用缓存显示 (SWR)
  if (state.fullPlaylist.length > 0 && state.playQueue.length === 0) {
    state.playQueue = [...state.fullPlaylist];
    if (state.currentTab === 'local' || state.currentTab === 'fav') renderPlaylist();
    // 尝试立即恢复状态
    if (!ui.audio.src) { initPlayerState(); }
  }

  try {
    const json = await api.library.list();
    if (json.success && json.data) {
      // 2. 合并数据：保留本地缓存的封面和歌词
      const oldMap = new Map(state.fullPlaylist.map(s => [s.filename, s]));
      const newList = json.data.map(item => {
        const old = oldMap.get(item.filename);
        return {
          ...item,
          title: item.title || item.filename,
          artist: item.artist || '未知艺术家',
          // 严格使用 ID 进行播放
          id: item.id,
          src: `/api/music/play/${encodeURIComponent(item.id)}`,
          // 如果缓存中有非默认封面，则保留
          cover: (old && old.cover && !old.cover.includes('ICON_256')) ? old.cover : (item.album_art || '/static/images/ICON_256.PNG'),
          // 如果缓存中有歌词，则保留
          lyrics: (old && old.lyrics) ? old.lyrics : item.lyrics
        };
      });

      state.fullPlaylist = newList;
      savePlaylist(); // 更新缓存

      // 3. 更新播放队列上下文
      if (state.currentTab === 'local') {
        const currentFilename = state.playQueue[state.currentTrackIndex]?.filename;
        state.playQueue = [...state.fullPlaylist];
        if (currentFilename) {
          const newIdx = state.playQueue.findIndex(s => s.filename === currentFilename);
          if (newIdx !== -1) state.currentTrackIndex = newIdx;
        }
        renderPlaylist();
      } else if (state.currentTab === 'fav') {
        // 刷新收藏列表
        renderPlaylist();
      }

      if (state.playQueue.length === 0) state.playQueue = [...state.fullPlaylist];
      if (!ui.audio.src) { await initPlayerState(); }
    } else {
      if (ui.songContainer.children.length === 0)
        ui.songContainer.innerHTML = '<div class="loading">加载失败</div>';
    }
  } catch (e) {
    console.error(e);
    if (retry) setTimeout(() => loadSongs(false), 2000);
  }
}

let isUploadingScan = false;
export function startScan(isUserAction = false) {
  if (isUploadingScan) return;
  isUploadingScan = true;
  startScanPolling(isUserAction, loadSongs);
}

export async function performDelete(songId) {
  const encodedName = encodeURIComponent(songId);
  if (ui.audio.src.includes(encodedName)) { // 简单的匹配检查
    ui.audio.pause();
    ui.audio.removeAttribute('src');
    ui.audio.load();
    ['current-title', 'fp-title'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = '等待播放'; });
    ['current-cover', 'fp-cover'].forEach(id => { const el = document.getElementById(id); if (el) el.src = '/static/images/ICON_256.PNG'; });
    state.isPlaying = false;
    updatePlayState();
    ui.overlay?.classList.remove('active');
  }
  const delay = ms => new Promise(res => setTimeout(res, ms));
  await delay(200);
  try {
    const data = await api.library.deleteFile(songId);
    if (data.success) {
      showToast('删除成功');
      await loadSongs();
      // 删除后保持当前标签页，刷新时不跳走
      switchTab(state.currentTab || 'local');
      ui.actionMenuOverlay?.classList.remove('active');
      ui.confirmModalOverlay?.classList.remove('active');
    }
    else { showToast('删除失败: ' + (data.error || '未知错误')); }
  } catch (err) { console.error('删除错误:', err); showToast('网络请求失败'); }
}

export async function handleExternalFile() {
  const params = new URLSearchParams(window.location.search);
  const externalPath = params.get('path');
  const isImportMode = window.location.pathname === '/import_mode';
  const isPreviewMode = window.location.pathname === '/preview';

  if (!externalPath) return;
  if (isImportMode) {
    ui.uploadModal?.classList.add('active');
    if (ui.uploadFileName) ui.uploadFileName.innerText = '外部文件';
    if (ui.uploadFill) ui.uploadFill.style.width = '100%';
    if (ui.uploadPercent) ui.uploadPercent.innerText = 'Importing...';
    if (ui.uploadMsg) ui.uploadMsg.innerText = '正在导入...';
    if (ui.closeUploadBtn) ui.closeUploadBtn.style.display = 'none';
    try {
      const data = await api.library.importPath(externalPath);
      if (data.success) {
        if (ui.uploadMsg) ui.uploadMsg.innerText = '导入成功'; window.history.replaceState({}, document.title, '/'); await loadSongs(); switchTab('local'); const idx = state.fullPlaylist.findIndex(s => s.filename === data.filename); if (idx !== -1) {
          state.playQueue = [...state.fullPlaylist];
          await playTrack(idx);
          setTimeout(() => ui.uploadModal?.classList.remove('active'), 800);
        }
      } else { throw new Error(data.error); }
    } catch (err) { if (ui.uploadMsg) ui.uploadMsg.innerText = '失败: ' + err.message; if (ui.closeUploadBtn) ui.closeUploadBtn.style.display = 'inline-block'; }
  } else {
    // Preview Mode or Direct Play
    try {
      const json = await api.library.externalMeta(externalPath);
      if (json.success && json.data) {
        const song = json.data;
        const tempSong = { title: song.title, artist: song.artist, album: song.album, filename: song.filename, src: `/api/music/external/play?path=${encodeURIComponent(song.filename)}`, cover: song.album_art || '/static/images/ICON_256.PNG', isExternal: true };
        state.fullPlaylist.unshift(tempSong);
        switchTab('local');
        state.playQueue = [...state.fullPlaylist];
        await playTrack(0);
        ui.overlay?.classList.add('active'); window.history.replaceState({}, document.title, '/');
      }
    } catch (err) { console.error('直接播放失败:', err); }
  }
}

export function renderPlaylist() {
  if (!ui.songContainer) return;
  ui.songContainer.innerHTML = '';
  if (state.currentTab === 'fav') { state.displayPlaylist = state.fullPlaylist.filter(s => state.favorites.has(s.filename)); } else { state.displayPlaylist = state.fullPlaylist; }
  if (state.displayPlaylist.length === 0) {
    ui.songContainer.innerHTML = `<div class="loading-text" style="grid-column: 1/-1; padding: 4rem 0; font-size: 1.1rem; opacity: 0.6;">${state.currentTab === 'fav' ? '暂无收藏歌曲' : '暂无歌曲'}</div>`;
    return;
  }

  const frag = document.createDocumentFragment();
  state.displayPlaylist.forEach((song, index) => {
    const card = document.createElement('div');
    card.className = 'song-card';
    card.dataset.index = index;
    if (song.isExternal) card.style.border = '1px dashed var(--primary)';
    const isFav = state.favorites.has(song.filename);

    let favHtml = `<button class="card-fav-btn ${isFav ? 'active' : ''}" title="收藏"><i class="${isFav ? 'fas' : 'far'} fa-heart"></i></button>`;
    if (song.isExternal) favHtml = '';
    card.innerHTML = `${favHtml}<img src="${song.cover}" loading="lazy"><div class="card-info"><div class="title" title="${song.title}">${song.title}</div><div class="artist">${song.artist}</div></div>`;

    card.addEventListener('click', (e) => {
      if (!e.target.closest('.card-fav-btn')) {
        state.playQueue = [...state.displayPlaylist];
        playTrack(index);
      }
    });

    if (!song.isExternal) {
      const favBtn = card.querySelector('.card-fav-btn');
      favBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        toggleFavorite(song, favBtn);
      });
    }
    frag.appendChild(card);
  });
  ui.songContainer.appendChild(frag);
  highlightCurrentTrack();
}

function switchTab(tab) {
  state.currentTab = tab;
  ui.navLocal?.classList.remove('active');
  ui.navFav?.classList.remove('active');
  ui.navMount?.classList.remove('active');
  ui.navNetease?.classList.remove('active');

  if (tab === 'local') ui.navLocal?.classList.add('active');
  else if (tab === 'fav') ui.navFav?.classList.add('active');
  else if (tab === 'mount') ui.navMount?.classList.add('active');
  else if (tab === 'netease') ui.navNetease?.classList.add('active');

  if (tab === 'mount') {
    ui.viewPlayer?.classList.add('hidden');
    ui.viewNetease?.classList.add('hidden');
    ui.viewMount?.classList.remove('hidden');
    loadMountPoints();
  } else if (tab === 'netease') {
    ui.viewPlayer?.classList.add('hidden');
    ui.viewMount?.classList.add('hidden');
    ui.viewNetease?.classList.remove('hidden');
  } else {
    // local or fav
    ui.viewMount?.classList.add('hidden');
    ui.viewNetease?.classList.add('hidden');
    ui.viewPlayer?.classList.remove('hidden');

    // Styling hooks for CSS
    if (tab === 'fav') {
      ui.viewPlayer?.classList.add('tab-fav');
      ui.viewPlayer?.classList.remove('tab-local');
    } else {
      ui.viewPlayer?.classList.add('tab-local');
      ui.viewPlayer?.classList.remove('tab-fav');
    }

    renderPlaylist();
    // Update Desktop Title
    const titleEl = document.getElementById('list-title');
    if (titleEl) titleEl.innerText = (tab === 'fav') ? '收藏列表' : '歌曲列表';
  }

  // Update Mobile Title
  const titles = {
    'local': '本地音乐',
    'fav': '我的收藏',
    'mount': '挂载管理',
    'netease': '网易下载'
  };
  if (ui.mobileTitle) ui.mobileTitle.innerText = titles[tab] || '2FMusic';

  // Search Box Visibility: Only for Local Music
  if (ui.searchInput && ui.searchInput.parentElement) {
    if (tab === 'local') {
      ui.searchInput.parentElement.style.display = 'flex';
      // Give a small delay to allow display:flex to apply before opacity transition if we were to keep opacity, 
      // but for now just show it.
      ui.searchInput.parentElement.style.opacity = '1';
    } else {
      ui.searchInput.parentElement.style.display = 'none';
      ui.searchInput.parentElement.style.opacity = '0';
      ui.searchInput.value = ''; // Clear search
      // Trigger input event to reset list if needed, or just relying on renderPlaylist next time
    }
  }
  if (window.innerWidth <= 768 && ui.sidebar?.classList.contains('open')) ui.sidebar.classList.remove('open');
  persistState(ui.audio);
}

async function initPlayerState() {
  const allowedTabs = ['local', 'fav', 'mount', 'netease'];
  const preferredTab = allowedTabs.includes(state.currentTab) ? state.currentTab : null;
  const savedTab = allowedTabs.includes(state.savedState.tab) ? state.savedState.tab : 'local';
  const targetTab = preferredTab || savedTab;

  // Preview Mode: Force fullscreen immediately
  const isPreview = window.location.pathname === '/preview';
  if (isPreview) {
    ui.overlay?.classList.add('active');
  }

  switchTab(targetTab);
  if (state.savedState.volume !== undefined) { ui.audio.volume = state.savedState.volume; updateVolumeUI(ui.audio.volume); }
  if (state.savedState.playMode !== undefined) { state.playMode = state.savedState.playMode; updatePlayModeUI(); }
  if (state.savedState.isFullScreen) { ui.overlay?.classList.add('active'); }

  if (state.savedState.currentFilename) {
    // 优先通过文件名匹配，而不是索引
    const targetFilename = state.savedState.currentFilename;
    // 如果是收藏列表，确保当前队列是基于收藏过滤的? 
    // 上面 switchTab 已经做了，但 state.playQueue 可能还没更新（如果是 cachedPlaylist 刚加载）
    // renderPlaylist 会更新 displayPlaylist，但 playQueue 只有点击才更新?
    // initPlayerState 不应该依赖 playQueue? 
    // playTrack index 必须是 playQueue 的 index。
    // 如果当前 tab 是 local，playQueue 默认是 full。

    // 简单起见，我们先在 fullPlaylist 里找，或者在当前 playQueue 里找
    let idx = state.playQueue.findIndex(s => s.filename === targetFilename);
    // 如果没找到且 playQueue 为空，则重建队列
    if (idx === -1 && state.playQueue.length > 0) {
      // 尝试在 playQueue 找
      idx = state.playQueue.findIndex(s => s.filename === targetFilename);
    }

    if (idx !== -1) {
      state.currentTrackIndex = idx;
      await playTrack(idx, false);

      // 稳健恢复进度
      if (state.savedState.currentTime) {
        const t = state.savedState.currentTime;
        const seek = () => { if (Math.abs(ui.audio.currentTime - t) > 1) ui.audio.currentTime = t; };
        if (ui.audio.readyState >= 1) seek();
        else ui.audio.addEventListener('loadedmetadata', seek, { once: true });

        const pct = (t / (ui.audio.duration || 100)) * 100;
        if (ui.progressBar) { ui.progressBar.value = pct; updateSliderFill(ui.progressBar); }
        if (ui.fpProgressBar) { ui.fpProgressBar.value = pct; updateSliderFill(ui.fpProgressBar); }
      }
    }
  }
  if (!ui.audio.src && state.playQueue.length > 0) {
    state.currentTrackIndex = 0;
    await playTrack(0, false);
  }
}

export async function playTrack(index, autoPlay = true) {
  if (index < 0 || index >= state.playQueue.length) return;
  state.currentFetchId++;
  state.currentTrackIndex = index;
  const track = state.playQueue[index];
  if (ui.audio.src !== window.location.origin + track.src) ui.audio.src = track.src;
  loadTrackInfo(track);
  checkAndFetchMetadata(track, state.currentFetchId);
  highlightCurrentTrack();
  if (autoPlay) {
    try {
      await ui.audio.play();
      state.isPlaying = true;
      updatePlayState();
      requestWakeLock();
    }
    catch (e) { console.error('Auto-play blocked:', e); state.isPlaying = false; updatePlayState(); }
  }
  persistState(ui.audio);
}

function loadTrackInfo(track) {
  if (!track) return;
  ['current-title', 'fp-title'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = track.title; });
  ['current-artist', 'fp-artist'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = track.artist; });
  const coverSrc = track.cover || '/static/images/ICON_256.PNG';
  ['current-cover', 'fp-cover'].forEach(id => { const el = document.getElementById(id); if (el) el.src = coverSrc; });
  updateDetailFavButton(state.favorites.has(track.filename));
  document.title = `${track.title} - 2FMusic`;
  if (ui.lyricsContainer) ui.lyricsContainer.innerHTML = '';
  if (track.lyrics) parseAndRenderLyrics(track.lyrics); else renderNoLyrics('正在搜索歌词...');
  if ('mediaSession' in navigator) { navigator.mediaSession.metadata = new MediaMetadata({ title: track.title, artist: track.artist, artwork: [{ src: coverSrc, sizes: '512x512', type: 'image/jpeg' }] }); }
  if (ui.fpMenuBtn) { ui.fpMenuBtn.style.display = track.isExternal ? 'none' : 'block'; }
}

function highlightCurrentTrack() {
  if (!ui.audio.src) return;
  const currentSong = state.playQueue[state.currentTrackIndex];
  if (!currentSong) return;
  document.querySelectorAll('.song-card').forEach((card, i) => {
    const track = state.displayPlaylist[i];
    if (track && track.filename === currentSong.filename) card.classList.add('active');
    else card.classList.remove('active');
  });
}

function togglePlayMode() { state.playMode = (state.playMode + 1) % 3; updatePlayModeUI(); persistState(ui.audio); }
function updatePlayModeUI() {
  if (!ui.fpBtnMode) return;
  ui.fpBtnMode.classList.remove('active-mode', 'mode-loop-one');

  // Mode 0: List Loop (Default)
  if (state.playMode === 0) {
    ui.fpBtnMode.innerHTML = '<i class="fas fa-redo"></i>';
    ui.fpBtnMode.title = '列表循环';
  }
  // Mode 1: Shuffle
  else if (state.playMode === 1) {
    ui.fpBtnMode.classList.add('active-mode');
    ui.fpBtnMode.innerHTML = '<i class="fas fa-random"></i>';
    ui.fpBtnMode.title = '随机播放';
  }
  // Mode 2: Single Loop
  else if (state.playMode === 2) {
    ui.fpBtnMode.classList.add('active-mode', 'mode-loop-one');
    ui.fpBtnMode.innerHTML = '<i class="fas fa-redo"></i>';
    ui.fpBtnMode.title = '单曲循环';
  }
}

function nextTrack() {
  if (state.playQueue.length === 0) return;
  if (state.playMode === 1) {
    let newIndex = Math.floor(Math.random() * state.playQueue.length);
    while (state.playQueue.length > 1 && newIndex === state.currentTrackIndex) newIndex = Math.floor(Math.random() * state.playQueue.length);
    playTrack(newIndex);
  } else {
    let nextIndex = state.currentTrackIndex + 1;
    if (nextIndex >= state.playQueue.length) nextIndex = 0;
    playTrack(nextIndex);
  }
}
function prevTrack() {
  if (state.playQueue.length === 0) return;
  if (ui.audio.currentTime > 3) { ui.audio.currentTime = 0; return; }
  if (state.playMode === 1) playTrack(Math.floor(Math.random() * state.playQueue.length));
  else { let prevIndex = state.currentTrackIndex - 1; if (prevIndex < 0) prevIndex = state.playQueue.length - 1; playTrack(prevIndex); }
}
// 滑动状态
let isDragging = false;

ui.audio.addEventListener('ended', () => {
  // 结束时将最后一句歌词居中显示
  if (state.lyricsData.length && ui.lyricsContainer) {
    const lastIdx = state.lyricsData.length - 1;
    document.querySelectorAll('.lyric-line.active').forEach(l => l.classList.remove('active'));
    const lastLine = ui.lyricsContainer.querySelector(`.lyric-line[data-index="${lastIdx}"]`);
    if (lastLine) {
      lastLine.classList.add('active');
      lastLine.scrollIntoView({ behavior: 'smooth', block: 'center' });
    }
  }
  if (state.playMode === 2) { ui.audio.currentTime = 0; ui.audio.play(); } else nextTrack();
});

let lastVolume = 1.0;
function updateVolumeUI(val) {
  if (ui.volumeSlider) { ui.volumeSlider.value = val; updateSliderFill(ui.volumeSlider); }
  updateVolumeIcon(val);
}

function updateVolumeIcon(val) {
  if (ui.volIcon) { ui.volIcon.className = ''; if (val === 0) ui.volIcon.className = 'fas fa-volume-mute'; else if (val < 0.5) ui.volIcon.className = 'fas fa-volume-down'; else ui.volIcon.className = 'fas fa-volume-up'; }
}

function toggleMute() {
  if (ui.audio.volume > 0) { lastVolume = ui.audio.volume; ui.audio.volume = 0; updateVolumeUI(0); } else { ui.audio.volume = lastVolume > 0 ? lastVolume : 0.5; updateVolumeUI(ui.audio.volume); }
  persistState(ui.audio);
}

updateSliderFill(ui.progressBar); updateSliderFill(ui.fpProgressBar); updateSliderFill(ui.volumeSlider);
ui.audio.addEventListener('pause', () => persistState(ui.audio)); // 暂停时保存
ui.audio.addEventListener('timeupdate', () => {
  throttledPersist(); // 定期保存
  if (!ui.audio.duration) return;

  // Process lyrics always
  if (state.lyricsData.length) {
    let idx = state.lyricsData.findIndex(l => l.time > ui.audio.currentTime);
    idx = idx === -1 ? state.lyricsData.length - 1 : idx - 1;
    if (idx >= 0) {
      const currentLine = ui.lyricsContainer?.querySelector(`.lyric-line[data-index="${idx}"]`);
      if (currentLine && !currentLine.classList.contains('active')) {
        document.querySelectorAll('.lyric-line.active').forEach(l => l.classList.remove('active'));
        currentLine.classList.add('active');
        currentLine.scrollIntoView({ behavior: 'smooth', block: 'center' });
      }
    }
  }

  // Only update sliders if not dragging
  if (!isDragging) {
    const percent = (ui.audio.currentTime / ui.audio.duration) * 100;
    const timeStr = formatTime(ui.audio.currentTime);

    if (ui.progressBar) { ui.progressBar.value = percent; updateSliderFill(ui.progressBar); }
    if (ui.fpProgressBar) { ui.fpProgressBar.value = percent; updateSliderFill(ui.fpProgressBar); }
    ['time-current', 'fp-time-current'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = timeStr; });
  }
});

ui.audio.addEventListener('loadedmetadata', () => {
  const totalStr = formatTime(ui.audio.duration);
  ['time-total', 'fp-time-total'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = totalStr; });
});

// Progress Bar Logic
function onProgressInput(e) {
  isDragging = true;
  const val = e.target.value;
  updateSliderFill(e.target);

  // Sync Visuals
  if (e.target === ui.progressBar && ui.fpProgressBar) { ui.fpProgressBar.value = val; updateSliderFill(ui.fpProgressBar); }
  if (e.target === ui.fpProgressBar && ui.progressBar) { ui.progressBar.value = val; updateSliderFill(ui.progressBar); }

  // Update Time Text Visuals
  if (ui.audio.duration) {
    const time = (val / 100) * ui.audio.duration;
    const timeStr = formatTime(time);
    ['time-current', 'fp-time-current'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = timeStr; });
  }
}

function onProgressChange(e) {
  isDragging = false;
  if (ui.audio.duration) {
    ui.audio.currentTime = (e.target.value / 100) * ui.audio.duration;
  }
  updateSliderFill(e.target);
}

// Bind Events replaces the old simple binding
if (ui.progressBar) {
  ui.progressBar.addEventListener('input', onProgressInput);
  ui.progressBar.addEventListener('change', onProgressChange);
}
if (ui.fpProgressBar) {
  ui.fpProgressBar.addEventListener('input', onProgressInput);
  ui.fpProgressBar.addEventListener('change', onProgressChange);
}

async function checkAndFetchMetadata(track, fetchId) {
  const query = `?title=${encodeURIComponent(track.title)}&artist=${encodeURIComponent(track.artist)}&filename=${encodeURIComponent(track.filename)}`;

  const fetchLyrics = async () => {
    if (track.lyrics) return;
    try {
      const d = await api.library.lyrics(query);
      if (fetchId !== state.currentFetchId) return;
      if (d.success && d.lyrics) { track.lyrics = d.lyrics; savePlaylist(); parseAndRenderLyrics(d.lyrics); }
      else { renderNoLyrics('暂无歌词'); }
    } catch (e) { if (fetchId === state.currentFetchId) renderNoLyrics('歌词加载失败'); }
  };

  const fetchCover = async () => {
    if (!track.cover.includes('ICON_256.PNG')) return;
    try {
      const d = await api.library.albumArt(query);
      if (fetchId !== state.currentFetchId) return;
      if (d.success && d.album_art) {
        track.cover = d.album_art;
        savePlaylist(); // 保存封面更新
        if (ui.audio.src.includes(encodeURIComponent(track.id))) { ['current-cover', 'fp-cover'].forEach(id => { const el = document.getElementById(id); if (el) el.src = track.cover; }); }
        renderPlaylist();
      }
    } catch (e) { }
  };

  await Promise.all([fetchLyrics(), fetchCover()]);
}

function parseAndRenderLyrics(lrc) {
  state.lyricsData = [];
  const lines = lrc.split('\n'); const timeRegex = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;
  lines.forEach(line => { const match = timeRegex.exec(line); if (match) { const min = parseInt(match[1]); const sec = parseInt(match[2]); const ms = parseInt(match[3]); const time = min * 60 + sec + (ms / (match[3].length === 3 ? 1000 : 100)); const text = line.replace(timeRegex, '').trim(); if (text) state.lyricsData.push({ time, text }); } });
  if (state.lyricsData.length === 0) { renderNoLyrics('纯音乐'); return; }

  if (ui.lyricsContainer) {
    ui.lyricsContainer.innerHTML = state.lyricsData.map((l, i) =>
      `<p class="lyric-line" data-index="${i}" data-time="${l.time}">${l.text}</p>`
    ).join('');
    ui.lyricsContainer.querySelectorAll('.lyric-line').forEach(line => {
      line.addEventListener('click', (e) => {
        const time = parseFloat(e.target.getAttribute('data-time'));
        if (!isNaN(time) && ui.audio.duration) {
          ui.audio.currentTime = time;
          if (state.isPlaying) ui.audio.play();
        }
      });
    });
    // 前奏阶段预先将第一句居中显示
    const firstLine = ui.lyricsContainer.querySelector('.lyric-line[data-index="0"]');
    if (firstLine) {
      document.querySelectorAll('.lyric-line.active').forEach(l => l.classList.remove('active'));
      firstLine.classList.add('active');
      firstLine.scrollIntoView({ behavior: 'auto', block: 'center' });
    }
  }
}

async function togglePlay() {
  if (state.playQueue.length === 0) return;

  // 如果正在播放，尝试暂停
  if (state.isPlaying) {
    ui.audio.pause();
    state.isPlaying = false;
    updatePlayState();
    releaseWakeLock();
    return;
  }

  // 如果原本是暂停状态，尝试播放
  try {
    if (!ui.audio.src) {
      await playTrack(0);
    } else {
      await ui.audio.play();
    }
    state.isPlaying = true;
    updatePlayState();
    requestWakeLock();
  } catch (err) {
    console.error('Play failed:', err);
    state.isPlaying = false;
    updatePlayState();
    releaseWakeLock();

    // 针对移动端后台恢复时的特定错误处理
    if (err.name === 'NotAllowedError') {
      showToast('播放被阻止，请手动点击播放');
    } else if (err.name === 'AbortError') {
      // 可能是之前的加载被中断，尝试重新加载当前进度
      const curTime = ui.audio.currentTime;
      ui.audio.load();
      ui.audio.currentTime = curTime;
      // 不自动播放，等待用户再次点击，避免死循环
      showToast('音频加载中断，已重置，请重试');
    } else {
      showToast('无法播放: ' + err.message);
    }
  }
}

function updatePlayState() {
  const icon = state.isPlaying ? '<i class="fas fa-pause"></i>' : '<i class="fas fa-play"></i>';
  if (ui.btnPlay) ui.btnPlay.innerHTML = icon;
  if (ui.fpBtnPlay) ui.fpBtnPlay.innerHTML = icon;
  if (ui.mobileMiniPlay) ui.mobileMiniPlay.innerHTML = icon;
}

export function bindPlayerEvents() {
  [ui.btnPlay, ui.fpBtnPlay, ui.mobileMiniPlay].forEach(btn => btn?.addEventListener('click', (e) => { e.stopPropagation(); togglePlay(); }));
  [ui.btnPrev, ui.fpBtnPrev].forEach(btn => btn?.addEventListener('click', prevTrack));
  [ui.btnNext, ui.fpBtnNext].forEach(btn => btn?.addEventListener('click', nextTrack));
  ui.fpBtnMode?.addEventListener('click', togglePlayMode);

  document.getElementById('open-detail-view')?.addEventListener('click', () => ui.overlay?.classList.add('active'));
  document.getElementById('close-detail-view')?.addEventListener('click', () => ui.overlay?.classList.remove('active'));

  if (ui.fpBtnFav) {
    ui.fpBtnFav.addEventListener('click', () => {
      const s = state.playQueue[state.currentTrackIndex];
      if (s && !s.isExternal) {
        toggleFavorite(s, null);
        updateDetailFavButton(state.favorites.has(s.filename));
        if (state.currentTab === 'fav') renderPlaylist();
      }
    });
  }

  ui.searchInput?.addEventListener('input', (e) => {
    const term = e.target.value.toLowerCase().trim();
    document.querySelectorAll('.song-card').forEach(card => {
      const index = card.dataset.index;
      const song = state.displayPlaylist[index];
      const match = song.title.toLowerCase().includes(term) || song.artist.toLowerCase().includes(term);
      if (match) card.classList.remove('hidden'); else card.classList.add('hidden');
    });
  });

  if (ui.volumeSlider) {
    ui.volumeSlider.addEventListener('input', (e) => {
      const val = parseFloat(e.target.value);
      ui.audio.volume = val;
      updateSliderFill(ui.volumeSlider);
      updateVolumeIcon(val);
    });
    ui.volumeSlider.addEventListener('change', () => persistState(ui.audio));
  }
  ui.btnMute?.addEventListener('click', toggleMute);
  updatePlayModeUI();
  updateVolumeUI(ui.audio.volume);

  // 增强：MediaSession 控制 (支持锁屏控制/后台控制)
  if ('mediaSession' in navigator) {
    const actionHandlers = [
      ['play', () => { if (!state.isPlaying) togglePlay(); }],
      ['pause', () => { if (state.isPlaying) togglePlay(); }],
      ['previoustrack', prevTrack],
      ['nexttrack', nextTrack],
      ['stop', () => { ui.audio.pause(); state.isPlaying = false; updatePlayState(); }]
    ];
    for (const [action, handler] of actionHandlers) {
      try { navigator.mediaSession.setActionHandler(action, handler); } catch (e) { }
    }
  }

  // 增强：音频错误监听
  ui.audio.addEventListener('error', (e) => {
    console.error('Audio Error:', e);
    state.isPlaying = false;
    updatePlayState();
    // 尝试获取错误详情
    const err = ui.audio.error;
    let msg = '未知音频错误';
    if (err) {
      switch (err.code) {
        case MediaError.MEDIA_ERR_ABORTED: msg = '加载被中断'; break;
        case MediaError.MEDIA_ERR_NETWORK: msg = '网络错误'; break;
        case MediaError.MEDIA_ERR_DECODE: msg = '解码错误'; break;
        case MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED: msg = '不支持的音频格式'; break;
      }
    }
    // 只有在用户试图播放时才频繁打扰，或者只记录
    if (state.playQueue.length > 0) {
      showToast('播放出错: ' + msg);
    }
  });

  // 增强：页面可见性恢复检查
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible') {
      // 检查 audio 是否丢失了状态
      if (ui.audio.error && state.playQueue.length > 0) {
        console.log('Restoring audio state after visibility mismatch...');
        const curTime = ui.audio.currentTime;
        ui.audio.load();
        ui.audio.currentTime = curTime;
        // 不自动 play，以免此时浏览器策略限制自动播放
        updatePlayState();
      }
      // 恢复 Wake Lock (如果应该播放)
      if (state.isPlaying) requestWakeLock();
    } else {
      // 页面不可见时（如手动锁屏），Wake Lock 会自动释放，这里显式调用清理变量
      // 只有 Screen Wake Lock 在页面隐藏时会自动释放，但我们最好保持状态同步
      if (wakeLock) { wakeLock = null; }
    }
  });
}

export function bindUiControls() {
  if (ui.navLocal) ui.navLocal.addEventListener('click', () => switchTab('local'));
  if (ui.navFav) ui.navFav.addEventListener('click', () => switchTab('fav'));
  if (ui.navMount) ui.navMount.addEventListener('click', () => switchTab('mount'));
  if (ui.navNetease) ui.navNetease.addEventListener('click', () => switchTab('netease'));

  if (ui.fpMenuBtn) ui.fpMenuBtn.addEventListener('click', (e) => { e.stopPropagation(); ui.actionMenuOverlay?.classList.add('active'); });
  ui.actionCancelBtn?.addEventListener('click', () => ui.actionMenuOverlay?.classList.remove('active'));

  if (ui.actionDownloadBtn) {
    ui.actionDownloadBtn.addEventListener('click', () => {
      ui.actionMenuOverlay?.classList.remove('active');
      const currentSong = state.playQueue[state.currentTrackIndex];
      if (currentSong) {
        showToast('开始下载...');
        const a = document.createElement('a');
        a.href = currentSong.src;
        // Construct filename: Artist - Title.ext
        // Note: browser might respect Content-Disposition header over this, but it's good practice.
        // We need extension from src or default to mp3
        let ext = 'mp3';
        if (currentSong.filename) {
          const match = currentSong.filename.match(/\.(\w+)$/);
          if (match) ext = match[1];
        }
        a.download = `${currentSong.artist} - ${currentSong.title}.${ext}`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
      }
    });
  }
  if (ui.actionDeleteBtn) {
    ui.actionDeleteBtn.addEventListener('click', () => {
      ui.actionMenuOverlay?.classList.remove('active');
      const currentSong = state.playQueue[state.currentTrackIndex];
      if (currentSong) {
        showConfirmDialog('危险操作', `确定要永久删除这首歌吗？<br><span style="font-size:0.9rem; opacity:0.7">${currentSong.title}</span>`, () => performDelete(currentSong.id));
      }
    });
  }
  ui.confirmNoBtn?.addEventListener('click', () => { ui.confirmModalOverlay?.classList.remove('active'); state.currentConfirmAction = null; });
  ui.confirmYesBtn?.addEventListener('click', () => {
    if (state.currentConfirmAction) {
      state.currentConfirmAction();
      ui.confirmModalOverlay?.classList.remove('active');
      state.currentConfirmAction = null;
    }
  });
  [ui.actionMenuOverlay, ui.confirmModalOverlay].forEach(overlay => {
    overlay?.addEventListener('click', (e) => { if (e.target === overlay) { overlay.classList.remove('active'); state.currentConfirmAction = null; } });
  });

  if (ui.menuBtn) {
    const toggleMenu = (e) => { e.stopPropagation(); ui.sidebar?.classList.toggle('open'); };
    ui.menuBtn.addEventListener('click', toggleMenu);
    if (ui.mobileTitle) ui.mobileTitle.addEventListener('click', toggleMenu);

    document.addEventListener('click', (e) => {
      if (window.innerWidth <= 768 && ui.sidebar?.classList.contains('open')) {
        const outsideSidebar = !ui.sidebar.contains(e.target);
        const outsideBtn = !ui.menuBtn.contains(e.target);
        const outsideTitle = !ui.mobileTitle || !ui.mobileTitle.contains(e.target);

        if (outsideSidebar && outsideBtn && outsideTitle) ui.sidebar.classList.remove('open');
      }
    });
  }

  // 封面自适应背景色
  const fpCover = document.getElementById('fp-cover');
  if (fpCover) {
    // 允许跨域图片提取颜色
    fpCover.crossOrigin = "Anonymous";

    const updateBg = () => {
      // 简单判断是否是默认图
      if (fpCover.src.indexOf('ICON_256.PNG') !== -1) {
        if (ui.fullPlayerOverlay) ui.fullPlayerOverlay.style.background = 'rgba(0, 0, 0, 0.85)';
      } else {
        const color = extractColorFromImage(fpCover);
        if (color) {
          // 1. 设置全屏背景渐变
          ui.fullPlayerOverlay.style.background = `linear-gradient(to bottom, ${color.toString()} 0%, #000 120%)`;

          // 2. 设置动态菜单背景色 (使用提取的 RGB + 0.6 透明度)
          // 这样 Action Menu 就有了跟随封面的半透明背景
          document.documentElement.style.setProperty('--dynamic-glass-color', `rgba(${color.r}, ${color.g}, ${color.b}, 0.6)`);
        }
      }
    };

    fpCover.addEventListener('load', updateBg);
    // 关键修正：如果图片已经加载完成（比如来自缓存），手动触发一次
    if (fpCover.complete && fpCover.naturalWidth > 0) {
      updateBg();
    }
  }
}

export async function initPlayer() {
  bindUiControls();
  bindPlayerEvents();
  // 立即恢复 Tab 状态，避免刷新时闪烁到默认页面
  switchTab(state.currentTab);
  await loadSongs();
  await handleExternalFile();
}
