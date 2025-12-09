document.addEventListener('DOMContentLoaded', () => {
    // === 核心状态变量 ===
    let fullPlaylist = [], displayPlaylist = [], currentTrackIndex = 0;
    // 新增：playQueue 独立播放队列，不再依赖当前显示的页面
    let playQueue = [];
    let isPlaying = false, playMode = 0, lyricsData = [], currentFetchId = 0;
    let favorites = new Set(JSON.parse(localStorage.getItem('2fmusic_favs') || '[]'));
    const savedState = JSON.parse(localStorage.getItem('2fmusic_state') || '{}');
    let currentTab = 'local';

    // === DOM 元素获取 (完整版) ===
    const audio = document.getElementById('audio-player');
    const songContainer = document.getElementById('song-list-container');
    const overlay = document.getElementById('full-player-overlay');
    const lyricsContainer = document.getElementById('lyrics-container');
    const searchInput = document.querySelector('.search-box input');

    const viewPlayer = document.getElementById('view-player');
    const viewMount = document.getElementById('view-mount');
    const viewNetease = document.getElementById('view-netease');
    const mountListContainer = document.getElementById('mount-list-container');
    const mountPathInput = document.getElementById('mount-path-input');
    const btnAddMount = document.getElementById('btn-add-mount');

    const navLocal = document.getElementById('nav-local');
    const navFav = document.getElementById('nav-fav');
    const navMount = document.getElementById('nav-mount');
    const navNetease = document.getElementById('nav-netease');
    const navUpload = document.getElementById('nav-upload');
    const fileUpload = document.getElementById('file-upload');

    const neteaseKeywordsInput = document.getElementById('netease-keywords');
    const neteaseQualitySelect = document.getElementById('netease-quality');
    const neteaseSearchBtn = document.getElementById('netease-search-btn');
    const neteaseResultList = document.getElementById('netease-result-list');
    const neteaseLoginStatus = document.getElementById('netease-login-status');
    const neteaseLoginCard = document.getElementById('netease-login-card');
    const neteaseLoginDesc = document.getElementById('netease-login-desc');
    const neteaseLoginBtn = document.getElementById('netease-login-btn');
    const neteaseRefreshStatusBtn = document.getElementById('netease-refresh-status');
    const neteaseQrImg = document.getElementById('netease-qr-img');
    const neteaseQrModal = document.getElementById('netease-qr-modal'); // Changed from wrapper to modal
    const closeQrModalBtn = document.getElementById('close-qr-modal'); // New close button
    const neteaseQrHint = document.getElementById('netease-qr-hint');
    const neteaseSongIdInput = document.getElementById('netease-song-id');
    const neteasePlaylistIdInput = document.getElementById('netease-playlist-id');
    const neteaseIdDownloadBtn = document.getElementById('netease-id-download');
    const neteaseDownloadDirInput = document.getElementById('netease-download-dir');
    const neteaseSelectAll = document.getElementById('netease-select-all');
    const neteaseBulkDownloadBtn = document.getElementById('netease-bulk-download');

    // Updated NetEase Elements
    const neteaseApiGateInput = document.getElementById('netease-api-gate-input');
    const neteaseApiGateBtn = document.getElementById('netease-api-gate-btn');
    const neteaseChangeApiBtn = document.getElementById('netease-change-api');

    const neteaseConfigGate = document.getElementById('netease-config-gate');
    const neteaseContent = document.getElementById('netease-content');
    const neteaseOpenConfigBtn = document.getElementById('netease-open-config'); // Deprecated but might exist in old logic
    const neteaseSaveDirBtn = document.getElementById('netease-save-dir');
    const neteaseDownloadList = document.getElementById('netease-download-list');
    const neteaseDownloadToggle = document.getElementById('netease-download-toggle');
    const neteaseDownloadPanel = document.getElementById('netease-download-panel');
    const neteaseDownloadFloating = document.getElementById('netease-download-floating');

    const uploadModal = document.getElementById('upload-modal');
    const uploadFileName = document.getElementById('upload-filename');
    const uploadFill = document.getElementById('upload-progress-fill');
    const uploadPercent = document.getElementById('upload-percent');
    const uploadMsg = document.getElementById('upload-msg');
    const closeUploadBtn = document.getElementById('close-upload-modal');

    const fpMenuBtn = document.getElementById('fp-menu-btn');
    const actionMenuOverlay = document.getElementById('action-menu-overlay');
    const actionDeleteBtn = document.getElementById('action-delete');
    const actionCancelBtn = document.getElementById('action-cancel');
    const confirmModalOverlay = document.getElementById('confirm-modal-overlay');
    const confirmYesBtn = document.getElementById('confirm-yes');
    const confirmNoBtn = document.getElementById('confirm-no');
    const confirmTitle = document.querySelector('.confirm-box h3');
    const confirmText = document.querySelector('.confirm-box p');
    const toastContainer = document.getElementById('toast-container');

    // 播放控制相关的 DOM
    const btnPlay = document.getElementById('btn-play');
    const btnPrev = document.getElementById('btn-prev');
    const btnNext = document.getElementById('btn-next');
    const progressBar = document.getElementById('progress-bar');
    const volumeSlider = document.getElementById('volume-slider');
    const fpBtnPlay = document.getElementById('fp-btn-play');
    const fpBtnPrev = document.getElementById('fp-btn-prev');
    const fpBtnNext = document.getElementById('fp-btn-next');
    const fpProgressBar = document.getElementById('fp-progress-bar');
    const fpBtnMode = document.getElementById('fp-btn-mode');
    const fpBtnFav = document.getElementById('fp-btn-fav');
    const btnMute = document.getElementById('btn-mute');
    const volIcon = document.getElementById('vol-icon');
    const mobileMiniPlay = document.getElementById('mobile-mini-play');
    const menuBtn = document.getElementById('mobile-menu-btn');
    const sidebar = document.getElementById('sidebar');

    // === 1. UI 适配 (修复手机缩放问题) ===
    function autoResizeUI() {
        if (window.innerWidth > 768) {
            let scale = Math.min(Math.max(window.innerWidth / 1440, 0.8), 1.2);
            document.documentElement.style.setProperty('--ui-scale', scale.toFixed(3));
        } else {
            document.documentElement.style.setProperty('--ui-scale', '1.0');
        }
    }
    window.addEventListener('resize', autoResizeUI);
    autoResizeUI();

    // 错误处理与右键屏蔽
    window.addEventListener('error', function (e) { if (e.target.tagName === 'IMG') e.target.src = '/static/images/ICON_256.PNG'; }, true);
    document.addEventListener('contextmenu', (e) => { if (e.target.tagName !== 'INPUT' && e.target.tagName !== 'TEXTAREA') e.preventDefault(); });

    // === 2. 状态保存 ===
    function saveState() {
        // 保存状态时参考的是 playQueue，而不是 displayPlaylist
        const currentSong = playQueue[currentTrackIndex];
        if (currentSong && currentSong.isExternal) return;

        const state = {
            volume: audio.volume,
            playMode: playMode,
            currentTime: audio.currentTime,
            currentFilename: currentSong ? currentSong.filename : null,
            tab: currentTab
        };
        localStorage.setItem('2fmusic_state', JSON.stringify(state));
    }
    window.addEventListener('beforeunload', saveState);

    // === 3. 增强型 Toast (支持进度更新) ===
    let progressToastEl = null;
    function showToast(message, isPersistent = false) {
        if (!isPersistent) {
            const toast = document.createElement('div');
            toast.className = 'toast';
            toast.textContent = message;
            if (toastContainer) toastContainer.appendChild(toast);
            requestAnimationFrame(() => toast.classList.add('show'));
            setTimeout(() => {
                toast.classList.remove('show');
                toast.addEventListener('transitionend', () => toast.remove());
            }, 3000);
            return;
        }

        if (!progressToastEl) {
            progressToastEl = document.createElement('div');
            progressToastEl.className = 'toast progress-toast';
            if (toastContainer) toastContainer.appendChild(progressToastEl);
            requestAnimationFrame(() => progressToastEl.classList.add('show'));
        }
        progressToastEl.innerHTML = `<i class="fas fa-sync fa-spin"></i> ${message}`;
    }

    function hideProgressToast() {
        if (progressToastEl) {
            progressToastEl.classList.remove('show');
            setTimeout(() => {
                if (progressToastEl) progressToastEl.remove();
                progressToastEl = null;
            }, 300);
        }
    }

    // === 4. 确认对话框 ===
    let currentConfirmAction = null;
    function showConfirmDialog(title, message, onConfirm) {
        if (confirmTitle) confirmTitle.innerText = title;
        if (confirmText) confirmText.innerHTML = message;
        currentConfirmAction = onConfirm;
        confirmModalOverlay.classList.add('active');
    }

    // === 5. 收藏功能 ===
    function toggleFavorite(song, btnEl) {
        if (favorites.has(song.filename)) {
            favorites.delete(song.filename);
            if (btnEl) { btnEl.classList.remove('active'); btnEl.innerHTML = '<i class="far fa-heart"></i>'; }
        } else {
            favorites.add(song.filename);
            if (btnEl) { btnEl.classList.add('active'); btnEl.innerHTML = '<i class="fas fa-heart"></i>'; }
        }
        localStorage.setItem('2fmusic_favs', JSON.stringify([...favorites]));

        // 检查当前播放的歌曲是否就是这首，更新播放详情页的爱心
        const currentPlaying = playQueue[currentTrackIndex];
        if (currentPlaying && currentPlaying.filename === song.filename) {
            updateDetailFavButton(favorites.has(song.filename));
        }

        if (currentTab === 'fav' && !favorites.has(song.filename)) renderPlaylist();
        saveState();
    }

    function updateDetailFavButton(isFav) {
        if (!fpBtnFav) return;
        if (isFav) { fpBtnFav.classList.add('active-fav'); fpBtnFav.innerHTML = '<i class="fas fa-heart"></i>'; }
        else { fpBtnFav.classList.remove('active-fav'); fpBtnFav.innerHTML = '<i class="far fa-heart"></i>'; }
    }

    if (fpBtnFav) {
        fpBtnFav.addEventListener('click', () => {
            const s = playQueue[currentTrackIndex]; // 从队列获取当前歌曲
            if (s && !s.isExternal) {
                toggleFavorite(s, null);
                updateDetailFavButton(favorites.has(s.filename));
                // 如果当前在收藏页，可能需要刷新列表
                if (currentTab === 'fav') renderPlaylist();
            }
        });
    }

    // === 6. 加载歌曲与轮询逻辑 ===
    async function loadSongs(retry = true) {
        try {
            const res = await fetch('/api/music');
            const json = await res.json();
            if (json.success && json.data) {
                fullPlaylist = json.data.map(item => ({
                    ...item,
                    title: item.title || item.filename,
                    artist: item.artist || '未知艺术家',
                    src: `/api/music/play/${encodeURIComponent(item.filename)}`,
                    cover: item.album_art || '/static/images/ICON_256.PNG',
                }));

                // 初始化播放队列：如果队列为空，默认载入所有歌曲，确保控制按钮可用
                if (playQueue.length === 0) playQueue = [...fullPlaylist];

                // 仅在本地/收藏页渲染列表
                if (currentTab === 'local' || currentTab === 'fav') renderPlaylist();

                if (!audio.src) { await initPlayerState(); }
            } else {
                if (songContainer.children.length === 0)
                    songContainer.innerHTML = '<div class="loading">加载失败</div>';
            }
        } catch (e) {
            console.error(e);
            if (retry) setTimeout(() => loadSongs(false), 2000);
        }
    }

    // 扫描轮询 (修复版：防止模态框时弹Toast)
    let isPolling = false;
    function startScanPolling(isUserAction = false) {
        if (isPolling) return;
        isPolling = true;
        let hasTrackedScan = false;

        const interval = setInterval(async () => {
            try {
                const res = await fetch('/api/system/status');
                const status = await res.json();

                const isModalOpen = uploadModal && uploadModal.classList.contains('active');

                if (status.scanning) {
                    hasTrackedScan = true;
                    if (!isModalOpen) {
                        const percent = status.total > 0 ? Math.round((status.processed / status.total) * 100) : 0;
                        showToast(`正在处理库... ${status.processed}/${status.total} (${percent}%)`, true);
                    }
                    if (status.processed % 20 === 0) loadSongs(false);
                } else {
                    clearInterval(interval);
                    isPolling = false;
                    hideProgressToast();

                    if ((isUserAction || hasTrackedScan) && !isModalOpen) {
                        showToast("处理完成！");
                        loadSongs();
                        if (currentTab === 'mount') loadMountPoints();
                    }
                }
            } catch (e) {
                console.error("Poll error", e);
                isPolling = false;
                clearInterval(interval);
            }
        }, 1000);
    }

    // === 7. 挂载管理 (特殊字符修复版) ===
    function loadMountPoints() {
        mountListContainer.innerHTML = '<div class="loading-text">加载中...</div>';
        fetch('/api/mount_points')
            .then(res => res.json())
            .then(data => {
                mountListContainer.innerHTML = '';
                if (data.success) {
                    if (data.data.length === 0) {
                        mountListContainer.innerHTML = '<div class="loading-text">暂无挂载目录</div>';
                    } else {
                        const frag = document.createDocumentFragment();
                        data.data.forEach(path => {
                            const card = document.createElement('div');
                            card.className = 'mount-card';

                            const infoDiv = document.createElement('div');
                            infoDiv.className = 'mount-info';
                            const icon = document.createElement('i');
                            icon.className = 'fas fa-folder mount-icon';
                            const pathSpan = document.createElement('span');
                            pathSpan.className = 'mount-path-text';
                            pathSpan.textContent = path;
                            infoDiv.appendChild(icon);
                            infoDiv.appendChild(pathSpan);

                            const btn = document.createElement('button');
                            btn.className = 'btn-remove-mount';
                            btn.textContent = '移除';
                            btn.onclick = () => triggerRemoveMount(path); // 修复：直接绑定函数

                            card.appendChild(infoDiv);
                            card.appendChild(btn);
                            frag.appendChild(card);
                        });
                        mountListContainer.appendChild(frag);
                    }
                } else { mountListContainer.innerHTML = `<div class="loading-text">加载失败: ${data.error}</div>`; }
            })
            .catch(() => mountListContainer.innerHTML = '<div class="loading-text">网络错误</div>');
    }

    window.triggerRemoveMount = function (path) {
        showConfirmDialog("移除挂载", `确定要移除目录<br><b>${path}</b> 吗？`, () => {
            showToast("正在移除...");
            fetch('/api/mount_points', { method: 'DELETE', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path: path }) })
                .then(res => res.json())
                .then(data => {
                    if (data.success) { showToast(data.message); loadMountPoints(); loadSongs(); }
                    else { showToast('移除失败: ' + data.error); }
                });
        });
    };

    // 模态框轮询进度
    function trackMountProgress() {
        const interval = setInterval(async () => {
            try {
                const res = await fetch('/api/system/status');
                const status = await res.json();

                if (status.scanning) {
                    const percent = status.total > 0 ? Math.round((status.processed / status.total) * 100) : 0;
                    if (uploadFill) uploadFill.style.width = `${percent}%`;
                    if (uploadPercent) uploadPercent.innerText = `${percent}%`;
                    if (uploadMsg) uploadMsg.innerText = status.current_file || '处理中...';
                } else {
                    clearInterval(interval);
                    if (uploadFill) uploadFill.style.width = '100%';
                    if (uploadPercent) uploadPercent.innerText = '100%';
                    if (uploadMsg) uploadMsg.innerText = '挂载并索引完成!';
                    setTimeout(() => {
                        uploadModal.classList.remove('active');
                        if (btnAddMount) btnAddMount.disabled = false;
                        if (mountPathInput) mountPathInput.value = '';
                        loadMountPoints();
                        loadSongs();
                    }, 1000);
                }
            } catch (e) { console.error("Mount poll error", e); }
        }, 500);
    }

    if (btnAddMount) {
        btnAddMount.addEventListener('click', () => {
            const path = mountPathInput.value.trim();
            if (!path) { showToast('请输入路径'); return; }

            uploadModal.classList.add('active');
            if (uploadFileName) uploadFileName.innerText = "挂载目录: " + path;
            if (uploadFill) uploadFill.style.width = '0%';
            if (uploadPercent) uploadPercent.innerText = '0%';
            if (uploadMsg) uploadMsg.innerText = '正在提交...';
            if (closeUploadBtn) closeUploadBtn.style.display = 'none';
            btnAddMount.disabled = true;

            fetch('/api/mount_points', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: path })
            })
                .then(res => res.json())
                .then(data => {
                    if (data.success) { trackMountProgress(); }
                    else {
                        if (uploadMsg) uploadMsg.innerText = '添加失败: ' + data.error;
                        if (closeUploadBtn) closeUploadBtn.style.display = 'inline-block';
                        btnAddMount.disabled = false;
                    }
                })
                .catch(() => {
                    if (uploadMsg) uploadMsg.innerText = '网络请求失败';
                    if (closeUploadBtn) closeUploadBtn.style.display = 'inline-block';
                    btnAddMount.disabled = false;
                });
        });
    }

    // === 8. 网易云下载 ===
    let neteaseResults = [];
    let neteasePollingTimer = null;
    let currentLoginKey = null;
    let neteaseDownloadDir = '';
    let neteaseApiBase = '';
    let neteaseSelected = new Set();
    let neteaseDownloadTasks = [];

    function renderDownloadTasks() {
        if (!neteaseDownloadList) return;
        if (!neteaseDownloadTasks.length) {
            neteaseDownloadList.innerHTML = '<div class="loading-text" style="padding: 3rem 0; opacity: 0.6; font-size: 0.9rem;">暂无下载记录</div>';
            return;
        }
        neteaseDownloadList.innerHTML = '';
        const frag = document.createDocumentFragment();

        const statusConfig = {
            queued: { text: '排队', icon: 'fas fa-clock', class: 'queued' },
            downloading: { text: '下载中', icon: 'fas fa-spinner fa-spin', class: 'downloading' },
            success: { text: '完成', icon: 'fas fa-check', class: 'success' },
            error: { text: '失败', icon: 'fas fa-times', class: 'error' },
            waiting: { text: '等待', icon: 'fas fa-hourglass-half', class: 'waiting' }
        };

        neteaseDownloadTasks.forEach(task => {
            const row = document.createElement('div');
            row.className = 'download-item';

            const meta = document.createElement('div');
            meta.className = 'download-meta';
            meta.innerHTML = `<div class="download-title" title="${task.title || '未命名'}">${task.title || '未命名'}</div>
                <div class="download-artist" title="${task.artist || '未知艺术家'}">${task.artist || '未知艺术家'}</div>`;

            const config = statusConfig[task.status] || { text: task.status, icon: 'fas fa-info-circle', class: '' };

            const statusEl = document.createElement('div');
            statusEl.className = `download-status ${config.class}`;
            statusEl.innerHTML = `<i class="${config.icon}"></i> <span>${config.text}</span>`;

            row.appendChild(meta);
            row.appendChild(statusEl);
            frag.appendChild(row);
        });
        neteaseDownloadList.appendChild(frag);
    }

    function addDownloadTask(song) {
        const task = {
            id: `dl_${Date.now()}_${Math.random().toString(16).slice(2, 6)}`,
            title: song.title || `歌曲 ${song.id || ''}`,
            artist: song.artist || '',
            status: 'queued'
        };
        neteaseDownloadTasks.unshift(task);
        if (neteaseDownloadTasks.length > 30) neteaseDownloadTasks = neteaseDownloadTasks.slice(0, 30);
        renderDownloadTasks();
        return task.id;
    }

    function updateDownloadTask(id, status) {
        const task = neteaseDownloadTasks.find(t => t.id === id);
        if (task) {
            task.status = status;
            renderDownloadTasks();
        }
    }

    function updateSelectAllState() {
        if (!neteaseSelectAll) return;
        const total = neteaseResults.length;
        const selectedCount = Array.from(neteaseSelected).filter(id => neteaseResults.some(s => String(s.id) === id)).length;
        neteaseSelectAll.indeterminate = selectedCount > 0 && selectedCount < total;
        neteaseSelectAll.checked = total > 0 && selectedCount === total;
    }

    function renderNeteaseResults() {
        if (!neteaseResultList) return;
        if (!neteaseResults.length) {
            neteaseResultList.innerHTML = '<div class="loading-text">未找到相关歌曲</div>';
            updateSelectAllState();
            return;
        }
        neteaseResultList.innerHTML = '';
        const frag = document.createDocumentFragment();
        neteaseResults.forEach(song => {
            const card = document.createElement('div');
            card.className = 'netease-card';

            const selectWrap = document.createElement('div');
            selectWrap.className = 'netease-select';
            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            const sid = String(song.id);
            checkbox.checked = neteaseSelected.has(sid);
            checkbox.addEventListener('change', () => {
                if (checkbox.checked) neteaseSelected.add(sid);
                else neteaseSelected.delete(sid);
                updateSelectAllState();
            });
            selectWrap.appendChild(checkbox);

            const cover = document.createElement('img');
            cover.src = song.cover || '/static/images/ICON_256.PNG';
            cover.loading = 'lazy';

            const meta = document.createElement('div');
            meta.className = 'netease-meta';
            const levelText = (song.level || 'standard').toUpperCase();
            meta.innerHTML = `<div class="title">${song.title}</div>
                <div class="subtitle">${song.artist}</div>
                <div class="extra"><span class="netease-level-pill">${levelText}</span>${song.album || '未收录专辑'} · ${formatTime(song.duration || 0)}</div>`;

            const actions = document.createElement('div');
            actions.className = 'netease-actions';
            const btn = document.createElement('button');
            btn.className = 'btn-primary';
            btn.innerHTML = '<i class="fas fa-download"></i> 下载';
            btn.addEventListener('click', () => downloadNeteaseSong(song, btn));
            actions.appendChild(btn);

            card.appendChild(selectWrap);
            card.appendChild(cover);
            card.appendChild(meta);
            card.appendChild(actions);
            frag.appendChild(card);
        });
        neteaseResultList.appendChild(frag);
        updateSelectAllState();
    }

    async function searchNeteaseSongs() {
        if (!neteaseKeywordsInput) return;
        const keywords = neteaseKeywordsInput.value.trim();
        if (!keywords) { showToast('请输入关键词'); return; }
        if (neteaseResultList) neteaseResultList.innerHTML = '<div class="loading-text">搜索中...</div>';
        try {
            const res = await fetch(`/api/netease/search?keywords=${encodeURIComponent(keywords)}`);
            const json = await res.json();
            if (json.success) {
                neteaseResults = json.data || [];
                neteaseSelected = new Set();
                renderNeteaseResults();
            } else {
                neteaseResultList.innerHTML = `<div class="loading-text">${json.error || '搜索失败'}</div>`;
            }
        } catch (err) {
            console.error('NetEase search failed', err);
            if (neteaseResultList) neteaseResultList.innerHTML = '<div class="loading-text">搜索失败，请检查 API 服务</div>';
        }
    }

    // === 动画特效 Helper ===
    function flyToElement(startEl, targetEl) {
        if (!startEl || !targetEl) return;
        const startRect = startEl.getBoundingClientRect();
        const targetRect = targetEl.getBoundingClientRect();

        const flyer = document.createElement('div');
        Object.assign(flyer.style, {
            position: 'fixed',
            zIndex: '9999',
            pointerEvents: 'none',
            width: '24px',
            height: '24px',
            borderRadius: '50%',
            backgroundColor: '#1db954', // var(--primary)
            color: '#fff',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            boxShadow: '0 2px 10px rgba(0,0,0,0.3)',
            left: `${startRect.left + startRect.width / 2}px`,
            top: `${startRect.top + startRect.height / 2}px`,
            transform: 'translate(-50%, -50%) scale(1)',
            transition: 'all 0.8s cubic-bezier(0.2, 0.8, 0.2, 1)'
        });
        flyer.innerHTML = '<i class="fas fa-music" style="font-size: 12px;"></i>';

        document.body.appendChild(flyer);

        // 强制重绘
        flyer.getBoundingClientRect();

        requestAnimationFrame(() => {
            const targetX = targetRect.left + targetRect.width / 2;
            const targetY = targetRect.top + targetRect.height / 2;

            Object.assign(flyer.style, {
                left: `${targetX}px`,
                top: `${targetY}px`,
                transform: 'translate(-50%, -50%) scale(0.2)',
                opacity: '0.5'
            });
        });

        flyer.addEventListener('transitionend', () => {
            flyer.remove();
            // 目标震动反馈
            if (targetEl) {
                targetEl.style.transition = 'transform 0.1s';
                targetEl.style.transform = 'scale(1.3)';
                setTimeout(() => {
                    targetEl.style.transform = '';
                    setTimeout(() => targetEl.style.transition = '', 100);
                }, 100);
            }
        });
    }

    async function downloadNeteaseSong(song, btnEl) {
        if (!song || !song.id) return;
        const level = neteaseQualitySelect ? neteaseQualitySelect.value : 'exhigh';

        // 乐观更新：先设置一个临时ID，等后端返回真正ID也可以，如果不涉及复杂逻辑
        const taskId = addDownloadTask(song);
        updateDownloadTask(taskId, 'downloading');

        if (btnEl) {
            btnEl.disabled = true;
            btnEl.innerHTML = '<i class="fas fa-sync fa-spin"></i> 下载中';
            // 触发飞行激画
            if (neteaseDownloadFloating) flyToElement(btnEl, neteaseDownloadFloating);
        }

        let isSuccess = false;
        try {
            const res = await fetch('/api/netease/download', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ...song, level, target_dir: neteaseDownloadDir || undefined }) });
            const json = await res.json();
            if (json.success) {
                isSuccess = true;
                updateDownloadTask(taskId, 'success');
                await loadSongs(false);
            } else {
                updateDownloadTask(taskId, 'error');
            }
        } catch (err) {
            console.error('download netease error', err);
            updateDownloadTask(taskId, 'error');
        } finally {
            if (btnEl) {
                if (isSuccess) {
                    btnEl.disabled = true;
                    btnEl.innerHTML = '<i class="fas fa-check"></i> 已下载';
                    btnEl.classList.add('downloaded'); // 可配合 CSS 使用
                } else {
                    btnEl.disabled = false;
                    btnEl.innerHTML = '<i class="fas fa-download"></i> 下载';
                }
            }
        }
    }

    if (neteaseSearchBtn) neteaseSearchBtn.addEventListener('click', searchNeteaseSongs);
    if (neteaseKeywordsInput) {
        neteaseKeywordsInput.addEventListener('keydown', (e) => { if (e.key === 'Enter') searchNeteaseSongs(); });
    }

    async function loadNeteaseConfig() {
        try {
            const res = await fetch('/api/netease/config');
            const json = await res.json();
            if (json.success) {
                neteaseDownloadDir = json.download_dir || '';
                neteaseApiBase = json.api_base || '';
                if (neteaseDownloadDirInput) neteaseDownloadDirInput.value = neteaseDownloadDir;
                if (neteaseApiGateInput) neteaseApiGateInput.value = neteaseApiBase || 'http://localhost:3000';

                // 改动：验证连接有效性后再显示
                if (neteaseApiBase) {
                    try {
                        const statusRes = await fetch('/api/netease/login/status');
                        const statusJson = await statusRes.json();

                        if (statusJson.success) {
                            toggleNeteaseGate(true); // 连接成功 -> 显示内容
                            // 刷新登录状态 UI
                            refreshLoginStatus();
                        } else {
                            toggleNeteaseGate(false); // 连接失败 -> 显示配置页
                        }
                    } catch (e) {
                        toggleNeteaseGate(false);
                    }
                } else {
                    toggleNeteaseGate(false);
                }
            }
        } catch (err) {
            console.error('config error', err);
            toggleNeteaseGate(false);
        }
    }

    async function bindNeteaseApi() {
        if (!neteaseApiGateInput) return;
        const apiBaseVal = neteaseApiGateInput.value.trim();
        if (!apiBaseVal) { showToast('请输入 API 地址'); return; }

        if (neteaseApiGateBtn) { neteaseApiGateBtn.disabled = true; neteaseApiGateBtn.innerText = "正在检测..."; }

        // 1. Save Config
        try {
            const payload = { api_base: apiBaseVal };
            // If we have a download dir set, include it to avoid clearing it? 
            // The backend likely merges or we should send what we have. 
            // For now, let's just send api_base. 
            // *Wait, if we only send api_base, does backend clear download_dir?*
            // Let's include download_dir if we have it locally.
            if (neteaseDownloadDir) payload.download_dir = neteaseDownloadDir;

            const res = await fetch('/api/netease/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
            const json = await res.json();
            if (json.success) {
                neteaseApiBase = json.api_base;
                // 2. Verify Connectivity
                const statusRes = await fetch('/api/netease/login/status'); // This will use the new API base on backend
                const statusJson = await statusRes.json();

                if (statusJson.success) {
                    showToast('连接成功');
                    toggleNeteaseGate(true);
                    refreshLoginStatus(); // Update UI
                } else {
                    showToast('无法连接到该 API 地址');
                }
            } else {
                showToast(json.error || '保存配置失败');
            }
        } catch (err) {
            console.error('bind error', err);
            showToast('连接失败，请检查地址');
        } finally {
            if (neteaseApiGateBtn) { neteaseApiGateBtn.disabled = false; neteaseApiGateBtn.innerText = "检测并连接"; }
        }
    }

    async function saveNeteaseConfig() {
        // This is for the "Settings" panel inside the content (mostly for download dir now)
        const dir = neteaseDownloadDirInput ? neteaseDownloadDirInput.value.trim() : '';
        const payload = {};
        if (dir) payload.download_dir = dir;
        // We don't change api_base here anymore, use the "Change API" button to go back to gate

        if (!payload.download_dir) { showToast('无需保存空设置'); return; }
        try {
            const res = await fetch('/api/netease/config', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
            const json = await res.json();
            if (json.success) {
                neteaseDownloadDir = json.download_dir;
                if (neteaseDownloadDirInput) neteaseDownloadDirInput.value = neteaseDownloadDir;
                showToast('保存成功');
            } else {
                showToast(json.error || '保存失败');
            }
        } catch (err) {
            console.error('save config error', err);
            showToast('保存失败');
        }
    }

    async function refreshLoginStatus(showToastMsg = false) {
        if (!neteaseLoginStatus) return;
        try {
            const res = await fetch('/api/netease/login/status');
            const json = await res.json();
            if (json.success && json.logged_in) {
                neteaseLoginStatus.innerText = `已登录：${json.nickname || ''}`;
                if (neteaseLoginCard) { neteaseLoginCard.classList.remove('status-bad'); neteaseLoginCard.classList.add('status-ok'); }
                if (neteaseLoginDesc) neteaseLoginDesc.innerText = '可以开始搜索或下载歌曲';
                if (neteaseQrImg) neteaseQrImg.src = '';
                if (neteaseQrModal) neteaseQrModal.classList.remove('active'); // Close modal
                if (showToastMsg) showToast('网易云已登录');
            } else {
                neteaseLoginStatus.innerText = json.error || '未登录';
                if (neteaseLoginCard) { neteaseLoginCard.classList.remove('status-ok'); neteaseLoginCard.classList.add('status-bad'); }
                if (neteaseLoginDesc) neteaseLoginDesc.innerText = '请扫码登录网易云账号';
                if (showToastMsg) showToast(json.error || '未登录');
            }
        } catch (err) {
            console.error('status error', err);
            if (showToastMsg) showToast('状态检查失败');
        }
    }

    async function startNeteaseLogin() {
        if (neteasePollingTimer) { clearInterval(neteasePollingTimer); neteasePollingTimer = null; }
        try {
            const res = await fetch('/api/netease/login/qrcode');
            const json = await res.json();
            if (!json.success) { showToast(json.error || '获取二维码失败'); return; }
            currentLoginKey = json.unikey;
            if (neteaseQrImg) neteaseQrImg.src = json.qrimg;
            if (neteaseQrModal) neteaseQrModal.classList.add('active'); // Show modal
            if (neteaseQrHint) neteaseQrHint.innerText = '使用网易云音乐扫码';
            neteaseLoginStatus.innerText = '等待扫码...';
            if (neteaseLoginCard) { neteaseLoginCard.classList.remove('status-ok'); neteaseLoginCard.classList.add('status-bad'); }
            neteasePollingTimer = setInterval(checkLoginStatus, 2000);
        } catch (err) {
            console.error('login qr error', err);
            showToast('获取二维码失败');
        }
    }

    async function checkLoginStatus() {
        if (!currentLoginKey) return;
        try {
            const res = await fetch(`/api/netease/login/check?key=${encodeURIComponent(currentLoginKey)}`);
            const json = await res.json();
            if (!json.success) return;
            if (json.status === 'authorized') {
                showToast('登录成功');
                if (neteaseLoginStatus) neteaseLoginStatus.innerText = '已登录';
                if (neteaseLoginCard) { neteaseLoginCard.classList.remove('status-bad'); neteaseLoginCard.classList.add('status-ok'); }
                if (neteaseLoginDesc) neteaseLoginDesc.innerText = '可以开始搜索或下载歌曲';
                if (neteaseQrModal) neteaseQrModal.classList.remove('active'); // 修复：关闭二维码弹窗
                refreshLoginStatus();
                if (neteasePollingTimer) { clearInterval(neteasePollingTimer); neteasePollingTimer = null; }
            } else if (json.status === 'expired') {
                showToast('二维码已过期，请重新获取');
                if (neteaseQrHint) neteaseQrHint.innerText = '二维码已过期，请重新获取';
                if (neteasePollingTimer) { clearInterval(neteasePollingTimer); neteasePollingTimer = null; }
            } else if (json.status === 'scanned') {
                neteaseLoginStatus.innerText = '已扫码，等待确认...';
                if (neteaseLoginDesc) neteaseLoginDesc.innerText = '请在网易云确认登录';
            }
        } catch (err) {
            console.error('check login error', err);
        }
    }

    async function downloadByIds() {
        const songId = neteaseSongIdInput ? neteaseSongIdInput.value.trim() : '';
        const playlistId = neteasePlaylistIdInput ? neteasePlaylistIdInput.value.trim() : '';
        if (!songId && !playlistId) { showToast('请输入单曲ID或歌单ID'); return; }
        if (songId) {
            try {
                if (neteaseResultList) neteaseResultList.innerHTML = '<div class="loading-text">解析单曲中...</div>';
                const res = await fetch(`/api/netease/song?id=${encodeURIComponent(songId)}`);
                const json = await res.json();
                if (!json.success) { showToast(json.error || '解析失败'); return; }
                neteaseResults = json.data || [];
                neteaseSelected = new Set(neteaseResults.map(s => String(s.id)));
                renderNeteaseResults();
                if (!neteaseResults.length) {
                    if (neteaseResultList) neteaseResultList.innerHTML = '<div class="loading-text">未找到歌曲</div>';
                } else {
                    showToast(`解析到 ${neteaseResults.length} 首歌曲，可选择下载`);
                }
            } catch (err) {
                console.error('song parse error', err);
                showToast('解析失败');
            }
        } else if (playlistId) {
            try {
                if (neteaseResultList) neteaseResultList.innerHTML = '<div class="loading-text">解析歌单中...</div>';
                const res = await fetch(`/api/netease/playlist?id=${encodeURIComponent(playlistId)}`);
                const json = await res.json();
                if (!json.success) { showToast(json.error || '获取歌单失败'); return; }
                const songs = json.data || [];
                neteaseResults = songs;
                neteaseSelected = new Set(neteaseResults.map(s => String(s.id)));
                renderNeteaseResults();
                if (!neteaseResults.length) {
                    if (neteaseResultList) neteaseResultList.innerHTML = '<div class="loading-text">歌单为空</div>';
                } else {
                    showToast(`解析到 ${neteaseResults.length} 首歌曲，可选择下载`);
                }
            } catch (err) {
                console.error('playlist download error', err);
                showToast('解析失败');
            }
        }
    }

    async function bulkDownloadSelected() {
        const level = neteaseQualitySelect ? neteaseQualitySelect.value : 'exhigh';
        const targets = neteaseResults.filter(s => neteaseSelected.has(String(s.id)));
        if (!targets.length) { showToast('请先选择歌曲'); return; }

        if (neteaseBulkDownloadBtn) {
            neteaseBulkDownloadBtn.disabled = true;
            neteaseBulkDownloadBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> 处理中...';
        }

        let addedCount = 0;
        for (const s of targets) {
            // 这里不 await 每一个请求完成，而是快速添加到下载队列
            // 或者我们可以并行触发，但要注意不要瞬间发起过多请求导致后端堵塞
            // 既然 main.js 的 downloadNeteaseSong 内部有 async fetch，且我们希望看到每个按钮独立变化
            // 但在批量下载时，列表项的按钮可能不可见（因为在列表里），所以我们应该快速遍历触发
            downloadNeteaseSong({ ...s, level }); // 不 await，让其后台进行
            addedCount++;
            await new Promise(r => setTimeout(r, 100)); // 稍微间隔一下，避免卡顿
        }

        showToast(`已开始下载 ${addedCount} 首歌曲`);

        if (neteaseBulkDownloadBtn) {
            neteaseBulkDownloadBtn.disabled = false;
            neteaseBulkDownloadBtn.innerHTML = '<i class="fas fa-download"></i> 下载选中';
        }
    }

    if (neteaseLoginBtn) neteaseLoginBtn.addEventListener('click', startNeteaseLogin);
    if (closeQrModalBtn) closeQrModalBtn.addEventListener('click', () => {
        if (neteaseQrModal) neteaseQrModal.classList.remove('active');
        if (neteasePollingTimer) { clearInterval(neteasePollingTimer); neteasePollingTimer = null; }
    });
    if (neteaseRefreshStatusBtn) neteaseRefreshStatusBtn.addEventListener('click', () => refreshLoginStatus(true));
    if (neteaseIdDownloadBtn) neteaseIdDownloadBtn.addEventListener('click', downloadByIds);
    if (neteaseSaveDirBtn) neteaseSaveDirBtn.addEventListener('click', saveNeteaseConfig);
    if (neteaseSelectAll) neteaseSelectAll.addEventListener('change', (e) => {
        if (e.target.checked) neteaseSelected = new Set(neteaseResults.map(s => String(s.id)));
        else neteaseSelected.clear();
        renderNeteaseResults();
    });
    if (neteaseBulkDownloadBtn) neteaseBulkDownloadBtn.addEventListener('click', bulkDownloadSelected);

    // New Gate Listeners
    if (neteaseApiGateBtn) neteaseApiGateBtn.addEventListener('click', bindNeteaseApi);
    if (neteaseChangeApiBtn) {
        neteaseChangeApiBtn.addEventListener('click', () => {
            toggleNeteaseGate(false); // Show gate, hide content
        });
    }

    if (neteaseDownloadToggle && neteaseDownloadPanel) {
        neteaseDownloadToggle.addEventListener('click', () => {
            neteaseDownloadPanel.classList.add('hidden');
        });
    }
    if (neteaseDownloadFloating && neteaseDownloadPanel) {
        neteaseDownloadFloating.addEventListener('click', () => {
            neteaseDownloadPanel.classList.toggle('hidden');
        });
    }
    // Removed deprecated neteaseOpenConfigBtn listener

    function toggleNeteaseGate(enabled) {
        // enabled = true means API IS configured -> Show Content, Hide Gate
        // enabled = false means API NOT configured -> Hide Content, Show Gate
        if (neteaseConfigGate) neteaseConfigGate.classList.toggle('hidden', enabled);
        if (neteaseContent) neteaseContent.classList.toggle('hidden', !enabled);
    }

    // === 19. 网易云页面 Tab 切换 ===
    const neteaseTabs = document.querySelectorAll('.tab-btn');
    neteaseTabs.forEach(btn => {
        btn.addEventListener('click', () => {
            // 移除所有激活状态
            neteaseTabs.forEach(b => b.classList.remove('active'));
            document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

            // 激活当前
            btn.classList.add('active');
            const targetId = btn.getAttribute('data-target');
            const targetContent = document.getElementById(targetId);
            if (targetContent) targetContent.classList.add('active');
        });
    });

    // === 9. 标签切换 ===
    function switchTab(tab) {
        currentTab = tab;
        navLocal.classList.remove('active');
        navFav.classList.remove('active');
        navMount.classList.remove('active');
        if (navNetease) navNetease.classList.remove('active');

        if (tab === 'local') navLocal.classList.add('active');
        else if (tab === 'fav') navFav.classList.add('active');
        else if (tab === 'mount') navMount.classList.add('active');
        else if (tab === 'netease' && navNetease) navNetease.classList.add('active');

        if (tab === 'mount') {
            viewPlayer.classList.add('hidden');
            if (viewNetease) viewNetease.classList.add('hidden');
            viewMount.classList.remove('hidden');
            if (searchInput) searchInput.parentElement.style.visibility = 'hidden';
            loadMountPoints();
        } else if (tab === 'netease') {
            viewPlayer.classList.add('hidden');
            viewMount.classList.add('hidden');
            if (viewNetease) viewNetease.classList.remove('hidden');
            if (searchInput) searchInput.parentElement.style.visibility = 'hidden';
        } else {
            viewMount.classList.add('hidden');
            if (viewNetease) viewNetease.classList.add('hidden');
            viewPlayer.classList.remove('hidden');
            if (searchInput) { searchInput.parentElement.style.visibility = 'visible'; searchInput.value = ''; }
            renderPlaylist();
        }
        if (window.innerWidth <= 768 && sidebar.classList.contains('open')) sidebar.classList.remove('open');
        saveState();
    }
    navLocal.addEventListener('click', () => switchTab('local'));
    navFav.addEventListener('click', () => switchTab('fav'));
    navMount.addEventListener('click', () => switchTab('mount'));
    if (navNetease) navNetease.addEventListener('click', () => switchTab('netease'));

    // === 10. 状态初始化 ===
    async function initPlayerState() {
        const allowedTabs = ['local', 'fav', 'mount', 'netease'];
        const targetTab = allowedTabs.includes(savedState.tab) ? savedState.tab : 'local';
        switchTab(targetTab);
        if (savedState.volume !== undefined) { audio.volume = savedState.volume; updateVolumeUI(audio.volume); }
        if (savedState.playMode !== undefined) { playMode = savedState.playMode; updatePlayModeUI(); }

        // 恢复播放位置，注意这里使用 playQueue (默认为 fullPlaylist)
        if (savedState.currentFilename) {
            const idx = playQueue.findIndex(s => s.filename === savedState.currentFilename);
            if (idx !== -1) {
                currentTrackIndex = idx;
                await playTrack(idx, false);
                if (savedState.currentTime) {
                    audio.currentTime = savedState.currentTime;
                    const pct = (audio.currentTime / audio.duration) * 100 || 0;
                    if (progressBar) { progressBar.value = pct; updateSliderFill(progressBar); }
                    if (fpProgressBar) { fpProgressBar.value = pct; updateSliderFill(fpProgressBar); }
                }
            }
        }
        if (!audio.src && playQueue.length > 0) {
            currentTrackIndex = 0;
            await playTrack(0, false);
        }
    }

    // === 11. 文件上传 ===
    if (navUpload && fileUpload) {
        navUpload.addEventListener('click', () => { fileUpload.click(); if (window.innerWidth <= 768 && sidebar.classList.contains('open')) { sidebar.classList.remove('open'); } });
        fileUpload.addEventListener('change', () => {
            const file = fileUpload.files[0]; if (!file) return;
            if (!file.name.match(/\.(mp3|flac|wav|ogg|m4a)$/i)) { showToast('仅支持音频文件'); return; }
            uploadModal.classList.add('active'); uploadFileName.innerText = file.name; uploadFill.style.width = '0%'; uploadPercent.innerText = '0%'; uploadMsg.innerText = '正在上传...'; closeUploadBtn.style.display = 'none';
            const formData = new FormData(); formData.append('file', file);
            const xhr = new XMLHttpRequest();
            xhr.upload.onprogress = (e) => { if (e.lengthComputable) { const percent = Math.round((e.loaded / e.total) * 100); uploadFill.style.width = `${percent}%`; uploadPercent.innerText = `${percent}%`; } };
            xhr.onload = () => {
                if (xhr.status === 200) { const data = JSON.parse(xhr.responseText); if (data.success) { uploadFill.style.width = '100%'; uploadPercent.innerText = '100%'; uploadMsg.innerText = '上传成功!'; setTimeout(() => { uploadModal.classList.remove('active'); loadSongs(); }, 1000); } else { uploadMsg.innerText = '失败: ' + (data.error || '未知错误'); closeUploadBtn.style.display = 'inline-block'; } } else { uploadMsg.innerText = '服务器错误'; closeUploadBtn.style.display = 'inline-block'; }
            };
            xhr.onerror = () => { uploadMsg.innerText = '网络连接失败'; closeUploadBtn.style.display = 'inline-block'; };
            xhr.open('POST', '/api/music/upload', true); xhr.send(formData); fileUpload.value = '';
        });
        if (closeUploadBtn) closeUploadBtn.addEventListener('click', () => uploadModal.classList.remove('active'));
    }

    // === 12. 删除功能 ===
    async function performDelete(filename) {
        const encodedName = encodeURIComponent(filename);
        if (audio.src.includes(encodedName)) {
            audio.pause();
            audio.removeAttribute('src');
            audio.load();
            ['current-title', 'fp-title'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = "等待播放"; });
            ['current-cover', 'fp-cover'].forEach(id => { const el = document.getElementById(id); if (el) el.src = "/static/images/ICON_256.PNG"; });
            isPlaying = false;
            updatePlayState();
            if (overlay.classList.contains('active')) overlay.classList.remove('active');
        }
        const delay = ms => new Promise(res => setTimeout(res, ms));
        await delay(200);
        try {
            const res = await fetch(`/api/music/delete/${encodedName}`, { method: 'DELETE' });
            const data = await res.json();
            if (data.success) { showToast('删除成功'); await loadSongs(); if (actionMenuOverlay) actionMenuOverlay.classList.remove('active'); if (confirmModalOverlay) confirmModalOverlay.classList.remove('active'); }
            else { showToast('删除失败: ' + (data.error || '未知错误')); }
        } catch (err) { console.error('删除错误:', err); showToast('网络请求失败'); }
    }

    // === 13. 外部文件处理 ===
    async function handleExternalFile() {
        const params = new URLSearchParams(window.location.search);
        const externalPath = params.get('path');
        const isImportMode = window.location.pathname === '/import_mode';
        if (!externalPath) return;
        if (isImportMode) {
            uploadModal.classList.add('active');
            uploadFileName.innerText = "外部文件";
            uploadFill.style.width = '100%';
            uploadPercent.innerText = 'Importing...';
            uploadMsg.innerText = "正在导入...";
            closeUploadBtn.style.display = 'none';
            try {
                const res = await fetch('/api/music/import_path', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ path: externalPath }) });
                const data = await res.json();
                if (data.success) {
                    uploadMsg.innerText = "导入成功"; window.history.replaceState({}, document.title, '/'); await loadSongs(); switchTab('local'); const idx = fullPlaylist.findIndex(s => s.filename === data.filename); if (idx !== -1) {
                        playQueue = [...fullPlaylist]; // 更新队列
                        await playTrack(idx);
                        setTimeout(() => uploadModal.classList.remove('active'), 800);
                    }
                }
                else { throw new Error(data.error); }
            } catch (err) { uploadMsg.innerText = "失败: " + err.message; closeUploadBtn.style.display = 'inline-block'; }
        } else {
            try {
                const res = await fetch(`/api/music/external/meta?path=${encodeURIComponent(externalPath)}`);
                const json = await res.json();
                if (json.success && json.data) {
                    const song = json.data;
                    const tempSong = { title: song.title, artist: song.artist, album: song.album, filename: song.filename, src: `/api/music/external/play?path=${encodeURIComponent(song.filename)}`, cover: song.album_art || '/static/images/ICON_256.PNG', isExternal: true };
                    fullPlaylist.unshift(tempSong);
                    switchTab('local');
                    playQueue = [...fullPlaylist]; // 更新队列
                    await playTrack(0);
                    overlay.classList.add('active'); window.history.replaceState({}, document.title, '/');
                }
            } catch (err) { console.error("直接播放失败:", err); }
        }
    }

    // === 14. 列表渲染 (DocumentFragment 优化) ===
    function renderPlaylist() {
        songContainer.innerHTML = '';
        if (currentTab === 'fav') { displayPlaylist = fullPlaylist.filter(s => favorites.has(s.filename)); } else { displayPlaylist = fullPlaylist; }
        if (displayPlaylist.length === 0) { songContainer.innerHTML = `<div class="loading" style="grid-column: 1/-1;">${currentTab === 'fav' ? '暂无收藏' : '暂无歌曲'}</div>`; return; }

        const frag = document.createDocumentFragment();
        displayPlaylist.forEach((song, index) => {
            const card = document.createElement('div');
            card.className = 'song-card';
            card.dataset.index = index;
            if (song.isExternal) card.style.border = '1px dashed var(--primary)';
            const isFav = favorites.has(song.filename);

            // 构建卡片 HTML
            let favHtml = `<button class="card-fav-btn ${isFav ? 'active' : ''}" title="收藏"><i class="${isFav ? 'fas' : 'far'} fa-heart"></i></button>`;
            if (song.isExternal) favHtml = '';
            card.innerHTML = `${favHtml}<img src="${song.cover}" loading="lazy"><div class="card-info"><div class="title" title="${song.title}">${song.title}</div><div class="artist">${song.artist}</div></div>`;

            card.addEventListener('click', (e) => {
                if (!e.target.closest('.card-fav-btn')) {
                    // 核心修复：点击列表时，将当前视图的列表作为播放队列
                    playQueue = [...displayPlaylist];
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
        songContainer.appendChild(frag);
        highlightCurrentTrack();
    }

    searchInput.addEventListener('input', (e) => {
        const term = e.target.value.toLowerCase().trim();
        const cards = document.querySelectorAll('.song-card');
        cards.forEach(card => {
            const index = card.dataset.index;
            const song = displayPlaylist[index];
            const match = song.title.toLowerCase().includes(term) || song.artist.toLowerCase().includes(term);
            if (match) card.classList.remove('hidden'); else card.classList.add('hidden');
        });
    });

    // === 15. 播放核心逻辑 (使用 playQueue) ===
    async function playTrack(index, autoPlay = true) {
        // 使用 playQueue 而不是 displayPlaylist，这样即使页面切换也不影响播放
        if (index < 0 || index >= playQueue.length) return;
        currentFetchId++;
        currentTrackIndex = index;
        const track = playQueue[index];
        if (audio.src !== window.location.origin + track.src) audio.src = track.src;
        loadTrackInfo(track);
        checkAndFetchMetadata(track, currentFetchId);
        highlightCurrentTrack();
        if (autoPlay) { try { await audio.play(); isPlaying = true; updatePlayState(); } catch (e) { console.error("Auto-play blocked:", e); isPlaying = false; updatePlayState(); } }
        saveState();
    }

    function loadTrackInfo(track) {
        if (!track) return;
        ['current-title', 'fp-title'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = track.title; });
        ['current-artist', 'fp-artist'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = track.artist; });
        const coverSrc = track.cover || '/static/images/ICON_256.PNG';
        ['current-cover', 'fp-cover'].forEach(id => { const el = document.getElementById(id); if (el) el.src = coverSrc; });
        updateDetailFavButton(favorites.has(track.filename));
        document.title = `${track.title} - 2FMusic`;
        lyricsContainer.innerHTML = '';
        if (track.lyrics) parseAndRenderLyrics(track.lyrics); else renderNoLyrics("正在搜索歌词...");
        if ('mediaSession' in navigator) { navigator.mediaSession.metadata = new MediaMetadata({ title: track.title, artist: track.artist, artwork: [{ src: coverSrc, sizes: '512x512', type: 'image/jpeg' }] }); }
        if (fpMenuBtn) { fpMenuBtn.style.display = track.isExternal ? 'none' : 'block'; }
    }

    function highlightCurrentTrack() {
        const currentSrc = audio.src; if (!currentSrc) return;

        // 使用文件名匹配而不是索引匹配，因为队列和列表顺序可能不同
        const currentSong = playQueue[currentTrackIndex];
        if (!currentSong) return;

        document.querySelectorAll('.song-card').forEach((card, i) => {
            const track = displayPlaylist[i];
            if (track && track.filename === currentSong.filename) card.classList.add('active');
            else card.classList.remove('active');
        });
    }

    // === 16. 模式控制与播放事件 ===
    function togglePlayMode() { playMode = (playMode + 1) % 3; updatePlayModeUI(); saveState(); }
    function updatePlayModeUI() {
        if (!fpBtnMode) return;
        fpBtnMode.classList.remove('active-mode', 'mode-loop-one');
        if (playMode === 0) { fpBtnMode.innerHTML = '<i class="fas fa-redo"></i>'; fpBtnMode.title = "列表循环"; }
        else if (playMode === 1) { fpBtnMode.classList.add('active-mode', 'mode-loop-one'); fpBtnMode.innerHTML = '<i class="fas fa-random"></i>'; fpBtnMode.title = "随机播放"; } // 修复随机图标
        else if (playMode === 2) { fpBtnMode.classList.add('active-mode', 'mode-loop-one'); fpBtnMode.innerHTML = '<i class="fas fa-redo"></i>'; fpBtnMode.title = "单曲循环"; }

        // 修正：playMode=1 实际上应该是随机，之前的代码图标可能有点乱，这里统一规范一下
        // playMode 0: 顺序循环 (Redo)
        // playMode 1: 随机 (Random)
        // playMode 2: 单曲 (Redo + 1)
        if (playMode === 1) { fpBtnMode.classList.add('active-mode'); fpBtnMode.innerHTML = '<i class="fas fa-random"></i>'; fpBtnMode.classList.remove('mode-loop-one'); }
    }

    function nextTrack() {
        if (playQueue.length === 0) return;
        if (playMode === 1) {
            // 随机播放
            let newIndex = Math.floor(Math.random() * playQueue.length);
            while (playQueue.length > 1 && newIndex === currentTrackIndex) newIndex = Math.floor(Math.random() * playQueue.length);
            playTrack(newIndex);
        }
        else {
            let nextIndex = currentTrackIndex + 1;
            if (nextIndex >= playQueue.length) nextIndex = 0;
            playTrack(nextIndex);
        }
    }
    function prevTrack() {
        if (playQueue.length === 0) return;
        if (audio.currentTime > 3) { audio.currentTime = 0; return; }
        if (playMode === 1) playTrack(Math.floor(Math.random() * playQueue.length));
        else { let prevIndex = currentTrackIndex - 1; if (prevIndex < 0) prevIndex = playQueue.length - 1; playTrack(prevIndex); }
    }
    audio.addEventListener('ended', () => { if (playMode === 2) { audio.currentTime = 0; audio.play(); } else nextTrack(); });

    let lastVolume = 1.0;
    function updateVolumeUI(val) {
        if (volumeSlider) { volumeSlider.value = val; updateSliderFill(volumeSlider); }
        if (volIcon) { volIcon.className = ''; if (val === 0) volIcon.className = 'fas fa-volume-mute'; else if (val < 0.5) volIcon.className = 'fas fa-volume-down'; else volIcon.className = 'fas fa-volume-up'; }
    }
    if (volumeSlider) { volumeSlider.addEventListener('input', (e) => { audio.volume = e.target.value; updateVolumeUI(audio.volume); }); volumeSlider.addEventListener('change', saveState); }
    if (btnMute) btnMute.addEventListener('click', () => { if (audio.volume > 0) { lastVolume = audio.volume; audio.volume = 0; updateVolumeUI(0); } else { audio.volume = lastVolume > 0 ? lastVolume : 0.5; updateVolumeUI(audio.volume); } saveState(); });

    function updateSliderFill(el) { if (!el) return; const val = (el.value - el.min) / (el.max - el.min); el.style.backgroundSize = `${val * 100}% 100%`; }
    updateSliderFill(progressBar); updateSliderFill(fpProgressBar); updateSliderFill(volumeSlider);

    audio.addEventListener('timeupdate', () => {
        if (!audio.duration) return;
        const percent = (audio.currentTime / audio.duration) * 100;
        const timeStr = formatTime(audio.currentTime);
        if (progressBar) { progressBar.value = percent; updateSliderFill(progressBar); }
        if (fpProgressBar) { fpProgressBar.value = percent; updateSliderFill(fpProgressBar); }
        ['time-current', 'fp-time-current'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = timeStr; });
        if (lyricsData.length) {
            let idx = lyricsData.findIndex(l => l.time > audio.currentTime);
            idx = idx === -1 ? lyricsData.length - 1 : idx - 1;
            if (idx >= 0) {
                const currentLine = lyricsContainer.querySelector(`.lyric-line[data-index="${idx}"]`);
                if (currentLine && !currentLine.classList.contains('active')) {
                    document.querySelectorAll('.lyric-line.active').forEach(l => l.classList.remove('active'));
                    currentLine.classList.add('active');
                    currentLine.scrollIntoView({ behavior: 'smooth', block: 'center' });
                }
            }
        }
    });
    audio.addEventListener('loadedmetadata', () => { const totalStr = formatTime(audio.duration);['time-total', 'fp-time-total'].forEach(id => { const el = document.getElementById(id); if (el) el.innerText = totalStr; }); });
    function seek(e) { if (audio.duration) audio.currentTime = (e.target.value / 100) * audio.duration; updateSliderFill(e.target); }
    [progressBar, fpProgressBar].forEach(bar => bar?.addEventListener('input', seek));

    // === 17. 元数据与歌词 ===
    async function checkAndFetchMetadata(track, fetchId) {
        const query = `?title=${encodeURIComponent(track.title)}&artist=${encodeURIComponent(track.artist)}&filename=${encodeURIComponent(track.filename)}`;
        if (!track.lyrics) {
            try {
                const res = await fetch(`/api/music/lyrics${query}`);
                const d = await res.json();
                if (fetchId !== currentFetchId) return;
                if (d.success && d.lyrics) { track.lyrics = d.lyrics; parseAndRenderLyrics(d.lyrics); }
                else { renderNoLyrics("暂无歌词"); }
            } catch (e) { if (fetchId === currentFetchId) renderNoLyrics("歌词加载失败"); }
        }
        if (track.cover.includes('ICON_256.PNG')) {
            try {
                const res = await fetch(`/api/music/album-art${query}`);
                const d = await res.json();
                if (fetchId !== currentFetchId) return;
                if (d.success && d.album_art) {
                    track.cover = d.album_art;
                    if (audio.src.includes(encodeURIComponent(track.filename))) { ['current-cover', 'fp-cover'].forEach(id => { const el = document.getElementById(id); if (el) el.src = track.cover; }); }
                    renderPlaylist();
                }
            } catch (e) { }
        }
    }

    function parseAndRenderLyrics(lrc) {
        lyricsData = []; const lines = lrc.split('\n'); const timeRegex = /\[(\d{2}):(\d{2})\.(\d{2,3})\]/;
        lines.forEach(line => { const match = timeRegex.exec(line); if (match) { const min = parseInt(match[1]); const sec = parseInt(match[2]); const ms = parseInt(match[3]); const time = min * 60 + sec + (ms / (match[3].length === 3 ? 1000 : 100)); const text = line.replace(timeRegex, '').trim(); if (text) lyricsData.push({ time, text }); } });
        if (lyricsData.length === 0) { renderNoLyrics("纯音乐"); return; }

        lyricsContainer.innerHTML = lyricsData.map((l, i) =>
            `<p class="lyric-line" data-index="${i}" data-time="${l.time}">${l.text}</p>`
        ).join('');
        document.querySelectorAll('.lyric-line').forEach(line => {
            line.addEventListener('click', (e) => {
                const time = parseFloat(e.target.getAttribute('data-time'));
                if (!isNaN(time) && audio.duration) {
                    audio.currentTime = time;
                    if (isPlaying) audio.play();
                }
            });
        });
    }
    function renderNoLyrics(msg) { lyricsContainer.innerHTML = `<div style="height:100%;display:flex;align-items:center;justify-content:center;color:var(--text-sub);font-size:1.2rem;">${msg}</div>`; }
    function formatTime(s) { if (isNaN(s)) return "0:00"; const min = Math.floor(s / 60); const sec = Math.floor(s % 60); return `${min}:${sec.toString().padStart(2, '0')}`; }

    // === 18. 界面控制事件 ===
    function togglePlay() { if (playQueue.length === 0) return; if (isPlaying) audio.pause(); else { if (!audio.src) playTrack(0); else audio.play(); } isPlaying = !isPlaying; updatePlayState(); }
    function updatePlayState() {
        const icon = isPlaying ? '<i class="fas fa-pause"></i>' : '<i class="fas fa-play"></i>';
        if (btnPlay) btnPlay.innerHTML = icon;
        if (fpBtnPlay) fpBtnPlay.innerHTML = icon;
        if (mobileMiniPlay) mobileMiniPlay.innerHTML = icon;
    }

    [btnPlay, fpBtnPlay, mobileMiniPlay].forEach(btn => btn?.addEventListener('click', (e) => { e.stopPropagation(); togglePlay(); }));
    [btnPrev, fpBtnPrev].forEach(btn => btn?.addEventListener('click', prevTrack));
    [btnNext, fpBtnNext].forEach(btn => btn?.addEventListener('click', nextTrack));
    if (fpBtnMode) fpBtnMode.addEventListener('click', togglePlayMode);

    document.getElementById('open-detail-view')?.addEventListener('click', () => overlay.classList.add('active'));
    document.getElementById('close-detail-view')?.addEventListener('click', () => overlay.classList.remove('active'));

    if (fpMenuBtn) { fpMenuBtn.addEventListener('click', (e) => { e.stopPropagation(); actionMenuOverlay.classList.add('active'); }); }
    if (actionCancelBtn) { actionCancelBtn.addEventListener('click', () => { actionMenuOverlay.classList.remove('active'); }); }
    if (actionDeleteBtn) {
        actionDeleteBtn.addEventListener('click', () => {
            actionMenuOverlay.classList.remove('active');
            const currentSong = playQueue[currentTrackIndex];
            if (currentSong) {
                showConfirmDialog(
                    "危险操作",
                    `确定要永久删除这首歌吗？<br><span style="font-size:0.9rem; opacity:0.7">${currentSong.title}</span>`,
                    () => performDelete(currentSong.filename)
                );
            }
        });
    }
    if (confirmNoBtn) { confirmNoBtn.addEventListener('click', () => { confirmModalOverlay.classList.remove('active'); currentConfirmAction = null; }); }
    if (confirmYesBtn) {
        confirmYesBtn.addEventListener('click', () => {
            if (currentConfirmAction) {
                currentConfirmAction();
                confirmModalOverlay.classList.remove('active');
                currentConfirmAction = null;
            }
        });
    }

    [actionMenuOverlay, confirmModalOverlay].forEach(overlay => {
        if (overlay) { overlay.addEventListener('click', (e) => { if (e.target === overlay) { overlay.classList.remove('active'); currentConfirmAction = null; } }); }
    });

    if (menuBtn) {
        menuBtn.addEventListener('click', (e) => { e.stopPropagation(); sidebar.classList.toggle('open'); });
        document.addEventListener('click', (e) => {
            if (window.innerWidth <= 768 && sidebar.classList.contains('open')) {
                if (!sidebar.contains(e.target) && !menuBtn.contains(e.target)) sidebar.classList.remove('open');
            }
        });
    }

    // === 启动顺序 ===
    startScanPolling(false); // 启动时检查，参数为 false 表示静默
    // refreshLoginStatus(); // 已移至 loadNeteaseConfig 连接成功后调用
    loadNeteaseConfig();
    renderDownloadTasks();
    loadSongs().then(() => {
        handleExternalFile();
    });
});
