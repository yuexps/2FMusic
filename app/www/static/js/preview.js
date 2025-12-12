import { api } from './api.js';
import { autoResizeUI, showToast, formatTime } from './utils.js';

// 独立的预览状态
const state = {
    audio: document.getElementById('audio-player'),
    isPlaying: false,
    track: null,
    duration: 0,
    playMode: 0, // 0: Loop, 1: Pause after finish
    favorites: new Set(JSON.parse(localStorage.getItem('2fmusic_favs') || '[]')),
    lyricsData: []
};

let ui = {};

function initUI() {
    ui = {
        cover: document.getElementById('fp-cover'),
        title: document.getElementById('fp-title'),
        artist: document.getElementById('fp-artist'),
        lyricsContainer: document.getElementById('lyrics-container'),
        progressBar: document.getElementById('fp-progress-bar'),
        currentTime: document.getElementById('fp-time-current'),
        totalTime: document.getElementById('fp-time-total'),
        playBtn: document.getElementById('fp-btn-play'),
        modeBtn: document.getElementById('fp-btn-mode'),
        favBtn: document.getElementById('fp-btn-fav'),
        bgOverlay: document.getElementById('full-player-overlay'),
        homeBtn: document.getElementById('open-home-view'),
        menuBtn: document.getElementById('fp-menu-btn'),
        menuOverlay: document.getElementById('action-menu-overlay'),
        actionRematch: document.getElementById('action-rematch'),
        actionCancel: document.getElementById('action-cancel')
    };
}

document.addEventListener('DOMContentLoaded', async () => {
    initUI();
    autoResizeUI();
    updateModeUI(); // Init mode UI
    window.addEventListener('resize', () => requestAnimationFrame(autoResizeUI));
    bindEvents();

    const params = new URLSearchParams(window.location.search);
    let path = params.get('path') || params.get('file') || params.get('filepath') || params.get('url') || params.get('src');

    if (path) {
        await loadPreviewTrack(path);
    } else {
        if (ui.title) ui.title.innerText = "未指定文件";
        showToast("未指定预览文件");
    }
});

function bindEvents() {
    ui.playBtn?.addEventListener('click', togglePlay);
    ui.modeBtn?.addEventListener('click', togglePlayMode);
    ui.favBtn?.addEventListener('click', toggleFavorite);

    // Open Home Page
    ui.homeBtn?.addEventListener('click', () => {
        window.open('/', '_blank');
    });

    // Menu Events
    ui.menuBtn?.addEventListener('click', (e) => { e.stopPropagation(); ui.menuOverlay?.classList.add('active'); });
    ui.actionCancel?.addEventListener('click', () => ui.menuOverlay?.classList.remove('active'));
    ui.menuOverlay?.addEventListener('click', (e) => { if (e.target === ui.menuOverlay) ui.menuOverlay.classList.remove('active'); });

    // Refresh Action
    ui.actionRematch?.addEventListener('click', async () => {
        ui.menuOverlay?.classList.remove('active');
        if (!state.track || !state.track.filename) return;

        try {
            await api.library.clearMetadata(state.track.filename);
            showToast('已清除缓存，正在重新获取...');

            // Reset local display first
            state.track.cover = '/static/images/ICON_256.PNG';
            state.track.lyrics = null;
            updateTrackUi();
            renderNoLyrics('正在搜索歌词...');

            // Re-fetch
            const query = `?title=${encodeURIComponent(state.track.title)}&artist=${encodeURIComponent(state.track.artist)}&filename=${encodeURIComponent(state.track.filename)}`;
            // Short delay to allow backend to clear
            setTimeout(() => fetchMetadata(state.track), 500);

        } catch (e) {
            console.error(e);
            showToast('操作失败: ' + e.message);
        }
    });

    if (ui.progressBar) {
        ui.progressBar.addEventListener('input', (e) => {
            const pct = parseFloat(e.target.value);
            updateSliderFill(e.target);
            if (state.duration) {
                state.audio.currentTime = (pct / 100) * state.duration;
                updateTimeDisplay(state.audio.currentTime, state.duration);
            }
        });
        updateSliderFill(ui.progressBar);
    }

    // Lyrics click seek
    if (ui.lyricsContainer) {
        ui.lyricsContainer.addEventListener('click', (e) => {
            const line = e.target.closest('.lyric-line');
            if (line && line.dataset.time !== undefined) {
                const time = parseFloat(line.dataset.time);
                if (!isNaN(time) && state.duration) {
                    state.audio.currentTime = time;
                    if (!state.isPlaying) state.audio.play().catch(console.error);
                }
            }
        });
    }

    state.audio.addEventListener('timeupdate', () => {
        if (!state.duration) return;
        const pct = (state.audio.currentTime / state.duration) * 100;
        if (ui.progressBar) {
            ui.progressBar.value = pct;
            updateSliderFill(ui.progressBar);
        }
        updateTimeDisplay(state.audio.currentTime, state.duration);
        syncLyrics(state.audio.currentTime);
    });

    state.audio.addEventListener('loadedmetadata', () => {
        state.duration = state.audio.duration;
        updateTimeDisplay(0, state.duration);
    });

    state.audio.addEventListener('play', () => { state.isPlaying = true; updatePlayBtn(); });
    state.audio.addEventListener('pause', () => { state.isPlaying = false; updatePlayBtn(); });
    state.audio.addEventListener('ended', () => {
        if (ui.lyricsContainer) ui.lyricsContainer.scrollTop = 0; // Reset lyrics

        if (state.playMode === 0) {
            // Loop
            state.audio.currentTime = 0;
            state.audio.play();
        } else {
            // Pause (Stop)
            state.isPlaying = false;
            updatePlayBtn();
        }
    });
    state.audio.addEventListener('error', (e) => { console.error(e); showToast("播放出错"); });

    // Real-time Favorite Sync
    window.addEventListener('storage', (e) => {
        if (e.key === '2fmusic_favs' && e.newValue) {
            try {
                const newFavs = JSON.parse(e.newValue);
                state.favorites = new Set(newFavs);
                if (state.track) {
                    updateFavBtn(state.favorites.has(state.track.filename));
                }
            } catch (err) { console.error("Sync favs error", err); }
        }
    });
}

function updateSliderFill(el) {
    if (!el) return;
    const val = parseFloat(el.value) || 0;
    el.style.backgroundSize = `${val}% 100%`;
}

async function loadPreviewTrack(path) {
    try {
        const json = await api.library.externalMeta(path);
        if (!json.success || !json.data) throw new Error("无法读取文件信息");

        const song = json.data;
        state.track = {
            title: song.title || "未知标题",
            artist: song.artist || "未知艺术家",
            filename: song.filename,
            cover: song.album_art || '/static/images/ICON_256.PNG',
            src: `/api/music/external/play?path=${encodeURIComponent(song.filename)}`
        };

        updateTrackUi();
        state.audio.src = state.track.src;
        try { await state.audio.play(); } catch (e) { console.warn("Autoplay blocked"); }

        fetchMetadata(state.track);

    } catch (e) {
        console.error(e);
        showToast("加载失败: " + e.message);
        if (ui.title) ui.title.innerText = "加载失败";
    }
}

function updateTrackUi() {
    if (!state.track) return;
    if (ui.title) ui.title.innerText = state.track.title;
    if (ui.artist) ui.artist.innerText = state.track.artist;
    if (ui.cover) {
        ui.cover.src = state.track.cover;
        if (state.track.cover.includes('ICON_256') && ui.bgOverlay) {
            // Reset gradient if default
            // ui.bgOverlay.style.background = ...; 
        }
    }
    updateFavBtn(state.favorites.has(state.track.filename));
}

function updatePlayBtn() {
    if (!ui.playBtn) return;
    ui.playBtn.innerHTML = state.isPlaying ? '<i class="fas fa-pause"></i>' : '<i class="fas fa-play"></i>';
}

function updateTimeDisplay(current, total) {
    if (ui.currentTime) ui.currentTime.innerText = formatTime(current);
    if (ui.totalTime) ui.totalTime.innerText = formatTime(total);
}

function togglePlay() {
    if (state.audio.paused) state.audio.play();
    else state.audio.pause();
}

async function fetchMetadata(track) {
    const query = `?title=${encodeURIComponent(track.title)}&artist=${encodeURIComponent(track.artist)}&filename=${encodeURIComponent(track.filename)}`;

    // Lyrics
    api.library.lyrics(query).then(d => {
        if (d.success && d.lyrics) renderLyrics(d.lyrics);
        else renderNoLyrics("暂无歌词");
    }).catch(() => renderNoLyrics("歌词加载失败"));

    // Cover (Logic same as main player: if current is default, try fetch)
    if (track.cover.includes('ICON_256')) {
        api.library.albumArt(query).then(d => {
            if (d.success && d.album_art) {
                track.cover = d.album_art;
                if (ui.cover) {
                    ui.cover.src = track.cover + '?t=' + Date.now();
                    // Load color
                    if (window.ColorThief) {
                        ui.cover.onload = () => updateBgColor(ui.cover);
                    }
                }
            }
        });
    } else {
        // Already have cover, load color
        if (window.ColorThief && ui.cover) {
            // Wait for image load if setting src just happened, or check complete
            if (ui.cover.complete) updateBgColor(ui.cover);
            else ui.cover.onload = () => updateBgColor(ui.cover);
        }
    }
}

function renderLyrics(lrc) {
    state.lyricsData = [];
    if (!lrc) { renderNoLyrics("暂无歌词"); return; }

    // Logic from player.js: parseAndRenderLyrics
    const lines = lrc.split('\n');
    const timeRegex = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;

    lines.forEach(line => {
        const match = timeRegex.exec(line);
        if (match) {
            const min = parseInt(match[1]);
            const sec = parseInt(match[2]);
            const ms = parseInt(match[3]);
            // player.js logic: simple calc
            const time = min * 60 + sec + (ms / (match[3].length === 3 ? 1000 : 100));
            const text = line.replace(timeRegex, '').trim();
            if (text) state.lyricsData.push({ time, text });
        }
    });

    if (state.lyricsData.length === 0) {
        renderNoLyrics('纯音乐');
        return;
    }

    if (ui.lyricsContainer) {
        ui.lyricsContainer.classList.remove('no-lyrics');
        ui.lyricsContainer.innerHTML = state.lyricsData.map((l, i) =>
            `<p class="lyric-line" data-index="${i}" data-time="${l.time}">${l.text}</p>`
        ).join('');

        ui.lyricsContainer.scrollTop = 0;

        // Frontend special: highlight first line if exists
        const firstLine = ui.lyricsContainer.querySelector('.lyric-line[data-index="0"]');
        if (firstLine) {
            ui.lyricsContainer.querySelectorAll('.lyric-line.active').forEach(l => l.classList.remove('active'));
            firstLine.classList.add('active');
        }
    }
}

function renderNoLyrics(msg) {
    if (ui.lyricsContainer) {
        ui.lyricsContainer.innerHTML = `<p class="lyric-line active">${msg}</p>`;
        ui.lyricsContainer.classList.add('no-lyrics');
    }
}

function syncLyrics(time) {
    if (!state.lyricsData.length || !ui.lyricsContainer) return;

    let activeIndex = -1;
    for (let i = 0; i < state.lyricsData.length; i++) {
        if (time >= state.lyricsData[i].time) activeIndex = i;
        else break;
    }
    // Force first line active if before first timestamp
    if (activeIndex === -1 && state.lyricsData.length > 0) activeIndex = 0;

    const lines = ui.lyricsContainer.getElementsByClassName('lyric-line');
    // Clear old active
    const oldActive = ui.lyricsContainer.querySelector('.active');
    if (oldActive) oldActive.classList.remove('active');

    if (activeIndex !== -1 && lines[activeIndex]) {
        const line = lines[activeIndex];
        line.classList.add('active');
        line.scrollIntoView({ behavior: 'smooth', block: 'center' }); // Keep center when playing
    }
}

function updateBgColor(imgEl) {
    if (!ui.bgOverlay) return;
    try {
        const colorThief = new ColorThief();
        const color = colorThief.getColor(imgEl);
        if (color) {
            // Match main player gradient style
            ui.bgOverlay.style.background = `linear-gradient(to bottom, rgb(${color[0]},${color[1]},${color[2]}) 0%, #1a1a1a 100%)`;
        }
    } catch (e) { console.warn(e); }
}

function togglePlayMode() {
    // Switch between 0 (Loop) and 1 (Pause after finish)
    state.playMode = state.playMode === 0 ? 1 : 0;
    updateModeUI();
}

function updateModeUI() {
    if (!ui.modeBtn) return;
    if (state.playMode === 0) {
        // Loop
        ui.modeBtn.innerHTML = '<i class="fas fa-redo"></i>';
        ui.modeBtn.title = "循环播放";
        ui.modeBtn.style.opacity = '1';
        ui.modeBtn.classList.add('active-mode');
    } else {
        // Pause/Once
        ui.modeBtn.innerHTML = '<i class="fas fa-long-arrow-alt-right"></i>';
        ui.modeBtn.title = "播完暂停";
        ui.modeBtn.style.opacity = '0.7';
        ui.modeBtn.classList.remove('active-mode');
    }
}

async function toggleFavorite() {
    if (!state.track) return;
    const btn = ui.favBtn;

    // 1. Check if favored
    if (state.favorites.has(state.track.filename)) {
        // Remove
        state.favorites.delete(state.track.filename);
        localStorage.setItem('2fmusic_favs', JSON.stringify([...state.favorites]));
        updateFavBtn(false);
        showToast("已取消收藏");
    } else {
        // Add
        // 2. Check source: External?
        if (state.track.src && state.track.src.includes('/api/music/external/play')) {
            showToast("正在导入并收藏...");
            try {
                const params = new URLSearchParams(window.location.search);
                const originPath = params.get('path') || params.get('file') || params.get('filepath') || params.get('src');

                if (!originPath) { throw new Error("无法获取源文件路径"); }

                const res = await api.library.importPath(originPath);
                if (res.success && res.filename) {
                    // Update track filename to the local one
                    state.track.filename = res.filename;
                    state.favorites.add(res.filename);
                    localStorage.setItem('2fmusic_favs', JSON.stringify([...state.favorites]));
                    updateFavBtn(true);
                    showToast("已导入并收藏");
                } else {
                    throw new Error(res.error || "导入失败");
                }
            } catch (e) {
                console.error(e);
                showToast("收藏失败: " + e.message);
            }
        } else {
            // Internal file
            state.favorites.add(state.track.filename);
            localStorage.setItem('2fmusic_favs', JSON.stringify([...state.favorites]));
            updateFavBtn(true);
            showToast("已收藏");
        }
    }
}

function updateFavBtn(isFav) {
    if (!ui.favBtn) return;
    if (isFav) {
        ui.favBtn.innerHTML = '<i class="fas fa-heart"></i>';
        ui.favBtn.classList.add('active');
        ui.favBtn.style.color = 'var(--primary-color)';
    } else {
        ui.favBtn.innerHTML = '<i class="far fa-heart"></i>';
        ui.favBtn.classList.remove('active');
        ui.favBtn.style.color = '';
    }
}
