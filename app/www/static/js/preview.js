import { api } from './api.js';
import { autoResizeUI, showToast, formatTime } from './utils.js';

// 独立的预览状态
const state = {
    audio: document.getElementById('audio-player'),
    isPlaying: false,
    track: null, // { title, artist, filename, cover, src }
    duration: 0
};

// UI 元素引用 (仅预览页存在的)
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
        bgOverlay: document.getElementById('full-player-overlay') // 用于背景色
    };
    console.log("UI Initialized", ui);
}

// 初始化
document.addEventListener('DOMContentLoaded', async () => {
    initUI(); // Initialize UI first
    autoResizeUI();
    window.addEventListener('resize', () => requestAnimationFrame(autoResizeUI));
    bindEvents();

    // 解析 URL 参数
    const params = new URLSearchParams(window.location.search);
    let path = params.get('path');

    // 尝试其他常见的参数名
    if (!path) path = params.get('file');
    if (!path) path = params.get('filepath');
    if (!path) path = params.get('url');
    if (!path) path = params.get('src');

    if (path) {
        await loadPreviewTrack(path);
    } else {
        ui.title.innerText = "未指定文件";
        console.warn("No path found", window.location.search);
    }
});

function bindEvents() {
    // 播放/暂停
    ui.playBtn?.addEventListener('click', togglePlay);

    // 进度条拖动
    // 辅助：更新滑块填充色
    function updateSliderFill(el) {
        if (!el) return;
        const val = parseFloat(el.value) || 0;
        const min = parseFloat(el.min) || 0;
        const max = parseFloat(el.max) || 100;
        let percent = ((val - min) / (max - min)) * 100;
        if (isNaN(percent)) percent = 0;
        percent = Math.max(0, Math.min(100, percent));
        el.style.backgroundSize = `${percent}% 100%`;
    }

    // 进度条拖动
    if (ui.progressBar) {
        ui.progressBar.addEventListener('input', (e) => {
            const pct = parseFloat(e.target.value);
            updateSliderFill(e.target); // 实时更新视觉
            if (state.duration) {
                state.audio.currentTime = (pct / 100) * state.duration;
                updateTimeDisplay(state.audio.currentTime, state.duration);
            }
        });
        // 初始化一次
        updateSliderFill(ui.progressBar);
    }

    // 歌词点击跳转
    if (ui.lyricsContainer) {
        ui.lyricsContainer.addEventListener('click', (e) => {
            const line = e.target.closest('.lyric-line');
            if (line && line.dataset.index !== undefined) {
                const index = parseInt(line.dataset.index);
                if (lyricsData[index]) {
                    state.audio.currentTime = lyricsData[index].time;
                    if (!state.isPlaying) {
                        state.audio.play().catch(console.error);
                    }
                }
            }
        });
    }

    // Audio 事件
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

    state.audio.addEventListener('play', () => {
        state.isPlaying = true;
        updatePlayBtn();
    });

    state.audio.addEventListener('pause', () => {
        state.isPlaying = false;
        updatePlayBtn();
    });

    state.audio.addEventListener('ended', () => {
        state.isPlaying = false;
        updatePlayBtn();
        // 预览模式单曲播放结束不做动作，或者可以重播
    });

    state.audio.addEventListener('error', (e) => {
        console.error("Audio error", e);
        showToast("播放出错");
    });
}

async function loadPreviewTrack(path) {
    try {
        // 1. 获取基本元数据
        const json = await api.library.externalMeta(path);
        if (!json.success || !json.data) throw new Error("无法读取文件信息");

        const song = json.data;
        state.track = {
            title: song.title || "未知标题",
            artist: song.artist || "未知艺术家",
            filename: song.filename,
            // 初始封面，可能是默认图
            cover: song.album_art || '/static/images/ICON_256.PNG',
            src: `/api/music/external/play?path=${encodeURIComponent(song.filename)}`
        };

        // 2. 更新 UI (初始状态)
        updateTrackUi();

        // 3. 设置源并播放
        state.audio.src = state.track.src;
        try {
            await state.audio.play();
        } catch (e) {
            console.warn("Autoplay blocked", e);
        }

        // 4. 异步获取更详细的元数据 (歌词、高清封面)
        fetchMetadata(state.track);

    } catch (e) {
        console.error(e);
        showToast("加载失败: " + e.message);
        ui.title.innerText = "加载失败";
    }
}

function updateTrackUi() {
    if (!state.track) return;
    if (ui.title) ui.title.innerText = state.track.title;
    if (ui.artist) ui.artist.innerText = state.track.artist;
    if (ui.cover) ui.cover.src = state.track.cover;

    // 背景色适配 (简单实现)
    if (ui.bgOverlay && state.track.cover.includes('ICON_256')) {
        ui.bgOverlay.style.background = 'var(--bg-color)';
    }
}

function updatePlayBtn() {
    if (!ui.playBtn) return;
    ui.playBtn.innerHTML = state.isPlaying ? '<i class="fas fa-pause"></i>' : '<i class="fas fa-play"></i>';

    // Toggle breathing animation class
    const overlay = document.getElementById('full-player-overlay');
    if (overlay) {
        if (state.isPlaying) overlay.classList.add('playing');
        else overlay.classList.remove('playing');
    }
}

function updateTimeDisplay(current, total) {
    if (ui.currentTime) ui.currentTime.innerText = formatTime(current);
    if (ui.totalTime) ui.totalTime.innerText = formatTime(total);
}

function togglePlay() {
    if (state.audio.paused) state.audio.play();
    else state.audio.pause();
}

// --- 独立的元数据获取逻辑 ---

async function fetchMetadata(track) {
    const query = `?title=${encodeURIComponent(track.title)}&artist=${encodeURIComponent(track.artist)}&filename=${encodeURIComponent(track.filename)}`;

    // 歌词
    api.library.lyrics(query).then(d => {
        if (d.success && d.lyrics) renderLyrics(d.lyrics);
        else renderNoLyrics("暂无歌词");
    }).catch(() => renderNoLyrics("歌词加载失败"));

    // 封面 (仅当默认图标时才去获取，或者总是尝试获取更好画质?)
    // 逻辑：如果当前通过 externalMeta 拿到的只是默认图（通常如此，除非已经缓存），则尝试获取
    if (track.cover.includes('ICON_256')) {
        api.library.albumArt(query).then(d => {
            if (d.success && d.album_art) {
                // 强制更新 UI，带时间戳
                track.cover = d.album_art;
                if (ui.cover) {
                    ui.cover.src = track.cover + '?t=' + new Date().getTime();
                }
                // 尝试提取颜色更新背景 (可选，需要 color-thief)
                if (window.ColorThief && ui.cover) {
                    ui.cover.onload = () => updateBgColor(ui.cover);
                }
            }
        });
    }
}

// --- 简化的歌词逻辑 ---
let lyricsData = [];
function renderLyrics(lrcText) {
    lyricsData = [];
    const lines = lrcText.split('\n');
    const timeRegex = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;

    lines.forEach(line => {
        const match = timeRegex.exec(line);
        if (match) {
            const min = parseInt(match[1]);
            const sec = parseInt(match[2]);
            const ms = parseInt(match[3]);
            const time = min * 60 + sec + (ms / (match[3].length === 3 ? 1000 : 100));
            const text = line.replace(timeRegex, '').trim();
            if (text) lyricsData.push({ time, text });
        }
    });

    if (ui.lyricsContainer) {
        ui.lyricsContainer.innerHTML = lyricsData.map((l, i) =>
            `<p class="lyric-line" data-index="${i}">${l.text}</p>`
        ).join('');
    }
}

function renderNoLyrics(msg) {
    if (ui.lyricsContainer) ui.lyricsContainer.innerHTML = `<p class="lyric-line">${msg}</p>`;
}

function syncLyrics(time) {
    if (!lyricsData.length || !ui.lyricsContainer) return;

    // 找到当前行
    let activeIndex = -1;
    for (let i = 0; i < lyricsData.length; i++) {
        if (time >= lyricsData[i].time) activeIndex = i;
        else break;
    }

    if (activeIndex !== -1) {
        const lines = ui.lyricsContainer.children;
        // 清除旧高亮
        const currentActive = ui.lyricsContainer.querySelector('.active');
        if (currentActive) currentActive.classList.remove('active');

        // 设置新高亮
        if (lines[activeIndex]) {
            lines[activeIndex].classList.add('active');
            lines[activeIndex].scrollIntoView({ behavior: 'smooth', block: 'center' });
        }
    }
}

function updateBgColor(imgEl) {
    if (!ui.bgOverlay) return;
    try {
        const colorThief = new ColorThief();
        const color = colorThief.getColor(imgEl);
        if (color) {
            ui.bgOverlay.style.background = `linear-gradient(to bottom, rgb(${color[0]},${color[1]},${color[2]}) 0%, #000 120%)`;
        }
    } catch (e) { }
}
