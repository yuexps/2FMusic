import { api } from './api.js';
import { autoResizeUI, showToast, formatTime } from './utils.js';

// 独立的预览状态
const state = {
    audio: document.getElementById('audio-player'),
    isPlaying: false,
    track: null,
    duration: 0,
    playMode: 0, // 0: 循环播放, 1: 播完暂停
    favorites: new Set(JSON.parse(localStorage.getItem('2fmusic_favs') || '[]')),
    lyricsData: []
};

let ui = {};

// 辅助函数：通过 CGI 代理转发 API 请求
const PROXY_PREFIX = api.API_BASE;
function toProxyUrl(url) {
    if (!url) return url;
    if (url.startsWith('/api/')) {
        return PROXY_PREFIX + url;
    }
    return url;
}

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
    updateModeUI(); // 初始化模式图标
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

    // 打开主页
    ui.homeBtn?.addEventListener('click', () => {
        // 总是打开网站根目录（主应用入口）
        window.open('/', '_blank');
    });

    // 菜单事件
    ui.menuBtn?.addEventListener('click', (e) => { e.stopPropagation(); ui.menuOverlay?.classList.add('active'); });
    ui.actionCancel?.addEventListener('click', () => ui.menuOverlay?.classList.remove('active'));
    ui.menuOverlay?.addEventListener('click', (e) => { if (e.target === ui.menuOverlay) ui.menuOverlay.classList.remove('active'); });

    // 刷新操作
    ui.actionRematch?.addEventListener('click', async () => {
        ui.menuOverlay?.classList.remove('active');
        if (!state.track || !state.track.filename) return;

        try {
            // filename 先尝试 ID 再尝试路径，调用后端清理
            if (/^\d+$/.test(state.track.filename)) {
                await api.library.clearMetadata(state.track.filename);
            } else {
                await api.library.clearMetadataExternal(state.track.filename);
            }

            showToast('正在刷新数据...');

            // 先重置本地显示
            state.track.cover = `${api.API_BASE}/images/icon_256.png`;
            state.track.lyrics = null;
            updateTrackUi();
            renderNoLyrics('正在搜索歌词...');

            // 重新获取 (延迟以待后台清理)
            setTimeout(() => fetchMetadata(state.track), 500);

        } catch (e) {
            console.error(e);
            showToast('操作失败: ' + e.message);
        }
    });

    state.isDragging = false;

    if (ui.progressBar) {
        const startDrag = () => { state.isDragging = true; };
        const endDrag = (e) => {
            state.isDragging = false;
            // Ensure final value is applied
            const pct = parseFloat(e.target.value);
            if (state.duration) state.audio.currentTime = (pct / 100) * state.duration;
        };

        ui.progressBar.addEventListener('mousedown', startDrag);
        ui.progressBar.addEventListener('touchstart', startDrag, { passive: true });

        ui.progressBar.addEventListener('change', endDrag);
        ui.progressBar.addEventListener('mouseup', endDrag);
        ui.progressBar.addEventListener('touchend', endDrag);

        ui.progressBar.addEventListener('input', (e) => {
            state.isDragging = true; // 正在输入时保持拖拽状态
            const pct = parseFloat(e.target.value);
            updateSliderFill(e.target);
            // 实时拖拽更新时间（防抖会在 endDrag 处理，这里仅做视觉反馈）
            if (state.duration) {
                const time = (pct / 100) * state.duration;
                updateTimeDisplay(time, state.duration);
                // Debouncing seek or doing it only on endDrag is often smoother, 
                // but immediate seek is requested by current code logic.
                // We only update time display here, audio seek can wait or be throttled?
                // Main app likely seeks on input. Let's keep it.
                if (Math.abs(state.audio.currentTime - time) > 1) { // Only seek if diff > 1s to avoid stutter
                    state.audio.currentTime = time;
                }
            }
        });
        updateSliderFill(ui.progressBar);
    }

    // 歌词点击进度跳转
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

        // 仅在未拖拽时更新进度条
        if (!state.isDragging && ui.progressBar) {
            const pct = (state.audio.currentTime / state.duration) * 100;
            ui.progressBar.value = pct;
            updateSliderFill(ui.progressBar);
        }

        // 总是更新时间显示
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
        if (ui.lyricsContainer) ui.lyricsContainer.scrollTop = 0; // 重置歌词位置

        if (state.playMode === 0) {
            // 循环
            state.audio.currentTime = 0;
            state.audio.play();
        } else {
            // 播完暂停
            state.isPlaying = false;
            updatePlayBtn();
        }
    });
    state.audio.addEventListener('error', (e) => { console.error(e); showToast("播放出错"); });

    // 实时同步收藏状态
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
    // 显示加载状态
    const albumArtEl = document.getElementById('fp-cover');
    if (albumArtEl) albumArtEl.style.opacity = '0.5';

    try {
        const json = await api.library.externalMeta(path);
        if (!json.success || !json.data) throw new Error("无法读取文件信息");

        const song = json.data;
        state.track = {
            title: song.title || "未知标题",
            artist: song.artist || "未知艺术家",
            album: song.album || "未知专辑",
            filename: song.filename || path, // 优先使用 ID，否则使用路径
            cover: toProxyUrl(song.album_art) || `${api.API_BASE}/images/icon_256.png`,
            src: `${api.API_BASE}/api/music/external/play?path=${encodeURIComponent(path)}`,
            duration: 0
        };

        updateTrackUi();
        state.audio.src = state.track.src;
        try { await state.audio.play(); } catch (e) {
            if (e.name !== 'AbortError') console.warn("Autoplay blocked/failed", e);
        }

        fetchMetadata(state.track);
        if (albumArtEl) albumArtEl.style.opacity = '1';

    } catch (e) {
        // 若 401，尝试自动登录（若已配置密码）
        if ((e.message === "401" || (e.message && e.message.includes('401'))) && window.PRECONFIGURED_PASSWORD) {
            try {
                await api.login(window.PRECONFIGURED_PASSWORD);
                // 重试一次
                window.PRECONFIGURED_PASSWORD = null; // 防止无限循环
                loadPreviewTrack(path);
                return;
            } catch (loginErr) {
                console.error("Auto-login failed:", loginErr);
            }
        }

        console.error(e);
        showToast("加载失败: " + e.message);
        if (ui.title) ui.title.innerText = "加载失败";
        if (albumArtEl) albumArtEl.style.opacity = '1';
    }
}

function updateTrackUi() {
    if (!state.track) return;
    if (ui.title) ui.title.innerText = state.track.title;
    if (ui.artist) ui.artist.innerText = state.track.artist;
    if (ui.cover) {
        ui.cover.src = state.track.cover;
        if (state.track.cover.includes('icon_256') && ui.bgOverlay) {
            // Reset gradient if default (no-op) 
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

    // 封面（同主程序逻辑：如果是默认图标则尝试获取封面）
    if (track.cover.includes('icon_256')) {
        api.library.albumArt(query).then(d => {
            if (d.success && d.album_art) {
                track.cover = toProxyUrl(d.album_art);
                if (ui.cover) {
                    ui.cover.src = track.cover + '&t=' + Date.now();
                    // 提取颜色
                    if (window.ColorThief) {
                        ui.cover.onload = () => updateBgColor(ui.cover);
                    }
                }
            }
        });
    } else {
        // 已有封面，直接提取颜色
        if (window.ColorThief && ui.cover) {
            // 确保图片加载完成后提取颜色
            if (ui.cover.complete) updateBgColor(ui.cover);
            else ui.cover.onload = () => updateBgColor(ui.cover);
        }
    }
}

function renderLyrics(lrc) {
    state.lyricsData = [];
    if (!lrc) { renderNoLyrics("暂无歌词"); return; }

    // 解析歌词（同主程序逻辑）
    const lines = lrc.split('\n');
    const timeRegex = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;

    lines.forEach(line => {
        const match = timeRegex.exec(line);
        if (match) {
            const min = parseInt(match[1]);
            const sec = parseInt(match[2]);
            const ms = parseInt(match[3]);
            // 简单时间计算
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

        // 高亮显示第一句
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
    // 如果在第一句之前，强制高亮第一句
    if (activeIndex === -1 && state.lyricsData.length > 0) activeIndex = 0;

    const lines = ui.lyricsContainer.getElementsByClassName('lyric-line');
    // 清除旧高亮
    const oldActive = ui.lyricsContainer.querySelector('.active');
    if (oldActive) oldActive.classList.remove('active');

    if (activeIndex !== -1 && lines[activeIndex]) {
        const line = lines[activeIndex];
        line.classList.add('active');
        line.scrollIntoView({ behavior: 'smooth', block: 'center' }); // 保持居中
    }
}

function updateBgColor(imgEl) {
    if (!ui.bgOverlay || !imgEl || imgEl.naturalWidth === 0) return;
    try {
        const colorThief = new window.ColorThief();
        const color = colorThief.getColor(imgEl);
        if (color) {
            // 匹配主播放器的渐变风格
            ui.bgOverlay.style.background = `linear-gradient(to bottom, rgb(${color[0]},${color[1]},${color[2]}) 0%, #1a1a1a 100%)`;
        }
    } catch (e) { console.warn(e); }
}

function togglePlayMode() {
    // 切换 循环 / 播完暂停
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
        // 播完暂停
        ui.modeBtn.innerHTML = '<i class="fas fa-long-arrow-alt-right"></i>';
        ui.modeBtn.title = "播完暂停";
        ui.modeBtn.style.opacity = '0.7';
        ui.modeBtn.classList.remove('active-mode');
    }
}

async function toggleFavorite() {
    if (!state.track) return;
    const btn = ui.favBtn;

    // 1. 检查是否已收藏
    if (state.favorites.has(state.track.filename)) {
        // 移除收藏
        state.favorites.delete(state.track.filename);
        localStorage.setItem('2fmusic_favs', JSON.stringify([...state.favorites]));
        updateFavBtn(false);
        showToast("已取消收藏");
    } else {
        // 添加收藏
        // 2. 检查来源：如果是外部文件
        if (state.track.src && state.track.src.includes(`${api.API_BASE}/api/music/external/play`)) {
            showToast("正在导入并收藏...");
            try {
                const params = new URLSearchParams(window.location.search);
                const originPath = params.get('path') || params.get('file') || params.get('filepath') || params.get('src');

                if (!originPath) { throw new Error("无法获取源文件路径"); }

                const res = await api.library.importPath(originPath);
                if (res.success && res.filename) {
                    // 更新为本地文件名
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
            // 内部文件
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
