import { state } from './state.js';
import { ui } from './ui.js';
import { api } from './api.js';
import { showToast, showConfirmDialog, hideProgressToast, formatTime } from './utils.js';

// 网易云业务
let songRefreshCallback = null;

function renderDownloadTasks() {
  const list = ui.neteaseDownloadList;
  const tasks = state.neteaseDownloadTasks;
  if (!list) return;
  if (!tasks.length) {
    list.innerHTML = '<div class="loading-text" style="padding: 3rem 0; opacity: 0.6; font-size: 0.9rem;">暂无下载记录</div>';
    return;
  }
  list.innerHTML = '';
  const frag = document.createDocumentFragment();
  tasks.forEach(task => {
    const row = document.createElement('div');
    row.className = 'netease-download-row';
    const meta = document.createElement('div');
    meta.className = 'netease-download-meta';
    meta.innerHTML = `<div class="title">${task.title}</div><div class="artist">${task.artist}</div>`;
    const statusEl = document.createElement('div');
    const config = {
      queued: { icon: 'fas fa-clock', text: '等待中', class: 'status-wait' },
      preparing: { icon: 'fas fa-spinner fa-spin', text: '准备中', class: 'status-progress' },
      downloading: { icon: 'fas fa-sync fa-spin', text: '下载中', class: 'status-progress' },
      success: { icon: 'fas fa-check', text: '完成', class: 'status-done' },
      error: { icon: 'fas fa-times', text: '失败', class: 'status-error' }
    }[task.status] || { icon: 'fas fa-question', text: '未知', class: '' };
    statusEl.className = `download-status ${config.class}`;
    if (task.status === 'downloading' || task.status === 'preparing') {
      const p = task.progress || 0;
      statusEl.innerHTML = `
        <div style="display:flex;flex-direction:column;align-items:flex-end;width:8rem;">
            <div style="font-size:0.75rem;margin-bottom:0.2rem;opacity:0.8;">${task.status === 'preparing' ? '准备中...' : p + '%'}</div>
            <div style="width:100%;height:4px;background:rgba(255,255,255,0.1);border-radius:2px;overflow:hidden;">
                <div style="width:${p}%;height:100%;background:var(--primary);transition:width 0.3s;"></div>
            </div>
        </div>
      `;
    } else {
      statusEl.innerHTML = `<i class="${config.icon}"></i> <span>${config.text}</span>`;
    }
    row.appendChild(meta);
    row.appendChild(statusEl);
    frag.appendChild(row);
  });
  list.appendChild(frag);
}

function addDownloadTask(song) {
  const task = {
    id: `dl_${Date.now()}_${Math.random().toString(16).slice(2, 6)}`,
    title: song.title || `歌曲 ${song.id || ''}`,
    artist: song.artist || '',
    songId: song.id,
    status: 'queued'
  };
  state.neteaseDownloadTasks.unshift(task);
  if (state.neteaseDownloadTasks.length > 30) state.neteaseDownloadTasks = state.neteaseDownloadTasks.slice(0, 30);
  renderDownloadTasks();
  return task.id;
}

function updateDownloadTask(id, status) {
  const task = state.neteaseDownloadTasks.find(t => t.id === id);
  if (task) {
    task.status = status;
    renderDownloadTasks();
  }
}

function updateSelectAllState() {
  const total = state.neteaseResults.length;
  const selectedCount = Array.from(state.neteaseSelected).filter(id => state.neteaseResults.some(s => String(s.id) === id)).length;
  if (ui.neteaseSelectAll) {
    ui.neteaseSelectAll.indeterminate = selectedCount > 0 && selectedCount < total;
    ui.neteaseSelectAll.checked = total > 0 && selectedCount === total;
  }
}

function renderNeteaseResults() {
  const list = ui.neteaseResultList;
  if (!list) return;
  if (!state.neteaseResults.length) {
    list.innerHTML = '<div class="loading-text">未找到相关歌曲</div>';
    updateSelectAllState();
    return;
  }
  list.innerHTML = '';
  const frag = document.createDocumentFragment();
  state.neteaseResults.forEach(song => {
    const card = document.createElement('div');
    card.className = 'netease-card';

    const selectWrap = document.createElement('div');
    selectWrap.className = 'netease-select';
    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    const sid = String(song.id);
    checkbox.checked = state.neteaseSelected.has(sid);
    checkbox.addEventListener('change', () => {
      if (checkbox.checked) state.neteaseSelected.add(sid);
      else state.neteaseSelected.delete(sid);
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

    // 检查是否已下载
    const isDownloaded = state.fullPlaylist && state.fullPlaylist.some(local =>
      (local.title || '').trim() === (song.title || '').trim() &&
      (local.artist || '').trim() === (song.artist || '').trim()
    );

    if (isDownloaded) {
      btn.className = 'btn-primary';
      btn.disabled = true;
      btn.innerHTML = '<i class="fas fa-check"></i> 已下载';
      btn.style.opacity = '0.7';
      btn.style.cursor = 'default';
    } else {
      btn.className = 'btn-primary';
      btn.innerHTML = '<i class="fas fa-download"></i> 下载';
      btn.addEventListener('click', () => downloadNeteaseSong(song, btn));
    }
    actions.appendChild(btn);

    card.appendChild(selectWrap);
    card.appendChild(cover);
    card.appendChild(meta);
    card.appendChild(actions);
    frag.appendChild(card);
  });
  list.appendChild(frag);
  updateSelectAllState();
}

async function downloadNeteaseSong(song, btnEl) {
  if (!song || !song.id) return;
  const level = ui.neteaseQualitySelect ? ui.neteaseQualitySelect.value : 'exhigh';

  // 检查是否有正在进行的相同任务
  const existingTask = state.neteaseDownloadTasks.find(t => String(t.songId) === String(song.id) && (t.status === 'preparing' || t.status === 'downloading'));
  if (existingTask) { showToast('该任务正在进行中'); return; }

  const taskId = addDownloadTask(song);

  if (btnEl) { btnEl.disabled = true; btnEl.innerHTML = '<i class="fas fa-sync fa-spin"></i> 请求中'; }

  // 自动展开下载列表
  if (ui.neteaseDownloadPanel && ui.neteaseDownloadPanel.classList.contains('hidden')) {
    ui.neteaseDownloadPanel.classList.remove('hidden');
  }

  try {
    const res = await api.netease.download({ ...song, level, target_dir: state.neteaseDownloadDir || undefined });
    if (res.success) {
      const backendTaskId = res.task_id;
      updateDownloadTask(taskId, 'preparing');

      // 保持按钮状态直到下载结束
      if (btnEl) {
        btnEl.disabled = true;
        // 清除 finally 中的恢复逻辑，改为手动恢复
      }

      // 轮询进度
      let failCount = 0;
      const pollTimer = setInterval(async () => {
        try {
          const taskRes = await api.netease.task(backendTaskId);
          if (taskRes.success) {
            failCount = 0; // 重置错误计数
            const tData = taskRes.data;
            const currentTask = state.neteaseDownloadTasks.find(t => t.id === taskId);

            // 更新按钮进度
            if (btnEl) {
              if (tData.status === 'downloading') {
                btnEl.innerHTML = `<i class="fas fa-circle-notch fa-spin"></i> ${tData.progress}%`;
              } else if (tData.status === 'preparing') {
                btnEl.innerHTML = `<i class="fas fa-spinner fa-spin"></i> 准备...`;
              }
            }

            if (currentTask) {
              // 状态映射
              let newStatus = tData.status;
              if (newStatus === 'pending') newStatus = 'queued';
              if (newStatus === 'preparing') newStatus = 'preparing';

              currentTask.status = newStatus;
              currentTask.progress = tData.progress;
              renderDownloadTasks();

              if (newStatus === 'success' || newStatus === 'error') {
                clearInterval(pollTimer);
                if (btnEl) {
                  btnEl.disabled = false;
                  btnEl.innerHTML = newStatus === 'success' ? '<i class="fas fa-check"></i> 完成' : '<i class="fas fa-redo"></i> 重试';
                  // 3秒后恢复默认文字或切换为已下载
                  setTimeout(() => {
                    if (btnEl.innerHTML.includes('重试')) {
                      btnEl.innerHTML = '<i class="fas fa-download"></i> 下载';
                    } else if (btnEl.innerHTML.includes('完成')) {
                      // 成功后变为已下载状态
                      btnEl.className = 'btn-primary';
                      btnEl.disabled = true;
                      btnEl.innerHTML = '<i class="fas fa-check"></i> 已下载';
                      btnEl.style.opacity = '0.7';
                      btnEl.style.cursor = 'default';
                    }
                  }, 3000);
                }

                if (newStatus === 'success') {
                  // showToast(`下载完成: ${tData.title}`);
                  if (songRefreshCallback) songRefreshCallback();
                } else {
                  showToast(`下载失败: ${tData.message || '未知错误'}`);
                }
              }
            } else {
              clearInterval(pollTimer); // 任务在前端被移除了
            }
          } else {
            // 任务在后端不存在 (可能因为重启丢失)
            updateDownloadTask(taskId, 'error');
            clearInterval(pollTimer);
            showToast('任务已失效 (服务器可能已重启)');
            if (btnEl) { btnEl.disabled = false; btnEl.innerHTML = '<i class="fas fa-redo"></i> 重试'; }
          }
        } catch (e) {
          console.error(e);
          failCount++;
          if (failCount > 10) { // 连续失败2秒
            clearInterval(pollTimer);
            updateDownloadTask(taskId, 'error');
            showToast('网络连接丢失，停止轮询');
            if (btnEl) { btnEl.disabled = false; btnEl.innerHTML = '<i class="fas fa-redo"></i> 重试'; }
          }
        }
      }, 200);

    } else {
      updateDownloadTask(taskId, 'error');
      showToast(res.error || '请求失败');
      if (btnEl) { btnEl.disabled = false; btnEl.innerHTML = '<i class="fas fa-download"></i> 下载'; }
    }
  } catch (err) {
    console.error('download netease error', err);
    updateDownloadTask(taskId, 'error');
    if (btnEl) { btnEl.disabled = false; btnEl.innerHTML = '<i class="fas fa-download"></i> 下载'; }
  }
}

async function searchNeteaseSongs() {
  if (!ui.neteaseKeywordsInput) return;
  const keywords = ui.neteaseKeywordsInput.value.trim();
  if (!keywords) { showToast('请输入关键词'); return; }
  if (ui.neteaseResultList) ui.neteaseResultList.innerHTML = '<div class="loading-text">搜索中...</div>';
  try {
    const json = await api.netease.search(keywords);
    if (json.success) {
      state.neteaseResults = json.data || [];
      state.neteaseSelected = new Set();
      renderNeteaseResults();
    } else {
      ui.neteaseResultList.innerHTML = `<div class="loading-text">${json.error || '搜索失败'}</div>`;
    }
  } catch (err) {
    console.error('NetEase search failed', err);
    if (ui.neteaseResultList) ui.neteaseResultList.innerHTML = '<div class="loading-text">搜索失败，请检查 API 服务</div>';
  }
}

async function loadNeteaseConfig() {
  let apiBase = 'http://localhost:23236'; // Default

  try {
    const json = await api.netease.configGet();
    if (json.success) {
      if (json.api_base) apiBase = json.api_base;
      state.neteaseDownloadDir = json.download_dir || '';
      if (ui.neteaseDownloadDirInput) ui.neteaseDownloadDirInput.value = state.neteaseDownloadDir;
    }
  } catch (err) {
    console.warn('Config load failed, utilizing default:', err);
  }

  // Update State & UI
  state.neteaseApiBase = apiBase;
  if (ui.neteaseApiGateInput) ui.neteaseApiGateInput.value = apiBase;

  // Auto-Connect Attempt
  try {
    const statusJson = await api.netease.loginStatus();
    if (statusJson.success) {
      toggleNeteaseGate(true);
      refreshLoginStatus();
    } else {
      // Connection failed or not logged in
      if (state.neteaseUser) {
        refreshLoginStatus(); // check if just session expired
      } else {
        toggleNeteaseGate(false);
      }
    }
  } catch (e) {
    // Network error or container down
    if (!state.neteaseUser) toggleNeteaseGate(false);
  }
}

async function saveNeteaseConfig() {
  const dir = ui.neteaseDownloadDirInput ? ui.neteaseDownloadDirInput.value.trim() : '';
  const apiBaseVal = ui.neteaseApiGateInput ? ui.neteaseApiGateInput.value.trim() : state.neteaseApiBase;
  const payload = {};
  if (dir || state.neteaseDownloadDir) payload.download_dir = dir || state.neteaseDownloadDir;
  if (apiBaseVal) payload.api_base = apiBaseVal;
  if (!payload.download_dir && !payload.api_base) { showToast('请输入下载目录或API地址'); return; }
  try {
    const json = await api.netease.configSave(payload);
    if (json.success) {
      state.neteaseDownloadDir = json.download_dir;
      state.neteaseApiBase = json.api_base || '';
      if (ui.neteaseApiGateInput) ui.neteaseApiGateInput.value = state.neteaseApiBase || 'http://localhost:23236';
      toggleNeteaseGate(!!state.neteaseApiBase);
      showToast('保存成功');
    } else {
      showToast(json.error || '保存失败');
    }
  } catch (err) {
    console.error('save config error', err);
    showToast('保存失败');
  }
}

async function bindNeteaseApi() {
  if (!ui.neteaseApiGateInput) return;
  const apiBaseVal = ui.neteaseApiGateInput.value.trim();
  if (!apiBaseVal) { showToast('请输入 API 地址'); return; }
  if (ui.neteaseApiGateBtn) { ui.neteaseApiGateBtn.disabled = true; ui.neteaseApiGateBtn.innerText = '正在检测...'; }
  try {
    const payload = { api_base: apiBaseVal };
    if (state.neteaseDownloadDir) payload.download_dir = state.neteaseDownloadDir;
    const json = await api.netease.configSave(payload);
    if (json.success) {
      state.neteaseApiBase = json.api_base;
      const statusJson = await api.netease.loginStatus();
      if (statusJson.success) {
        showToast('连接成功');
        toggleNeteaseGate(true);
        refreshLoginStatus();
      } else {
        showToast('无法连接到该 API 地址');
      }
    } else {
      showToast(json.error || '保存配置失败');
    }
  } catch (err) {
    console.error('bind error', err);
    showToast('连接失败');
  } finally {
    if (ui.neteaseApiGateBtn) { ui.neteaseApiGateBtn.disabled = false; ui.neteaseApiGateBtn.innerText = '连接'; }
  }
}

function renderLoginSuccessUI(user) {
  if (ui.neteaseLoginStatus) ui.neteaseLoginStatus.innerText = `已登录：${user.nickname || ''}`;
  ui.neteaseLoginCard?.classList.remove('status-bad');
  ui.neteaseLoginCard?.classList.add('status-ok');
  if (ui.neteaseLoginDesc) ui.neteaseLoginDesc.innerText = '可以开始搜索或下载歌曲';
  if (ui.neteaseQrImg) ui.neteaseQrImg.src = '';
  ui.neteaseQrModal?.classList.remove('active');
  if (ui.neteaseLoginBtn) ui.neteaseLoginBtn.style.display = 'none';
}

async function refreshLoginStatus(showToastMsg = false) {
  if (!ui.neteaseLoginStatus) return;
  try {
    const json = await api.netease.loginStatus();
    if (json.success && json.logged_in) {
      const user = { nickname: json.nickname, avatar: json.avatar };
      state.neteaseUser = user;
      localStorage.setItem('2fmusic_netease_user', JSON.stringify(user));

      renderLoginSuccessUI(user);
      if (showToastMsg) showToast('网易云已登录');
    } else {
      state.neteaseUser = null;
      localStorage.removeItem('2fmusic_netease_user');

      ui.neteaseLoginStatus.innerText = json.error || '未登录';
      ui.neteaseLoginCard?.classList.remove('status-ok');
      ui.neteaseLoginCard?.classList.add('status-bad');
      ui.neteaseLoginCard?.classList.add('status-bad');
      if (ui.neteaseLoginDesc) ui.neteaseLoginDesc.innerText = '请扫码登录网易云账号';
      if (ui.neteaseLoginBtn) ui.neteaseLoginBtn.style.display = '';
      if (showToastMsg) showToast(json.error || '未登录');
    }
  } catch (err) {
    console.error('status error', err);
    if (showToastMsg) showToast('状态检查失败');
  }
}

async function startNeteaseLogin() {
  if (state.neteasePollingTimer) { clearInterval(state.neteasePollingTimer); state.neteasePollingTimer = null; }
  try {
    const json = await api.netease.loginQr();
    if (!json.success) { showToast(json.error || '获取二维码失败'); return; }
    state.currentLoginKey = json.unikey;
    if (ui.neteaseQrImg) ui.neteaseQrImg.src = json.qrimg;
    ui.neteaseQrModal?.classList.add('active');
    if (ui.neteaseQrHint) ui.neteaseQrHint.innerText = '使用网易云音乐扫码';
    if (ui.neteaseLoginStatus) ui.neteaseLoginStatus.innerText = '等待扫码...';
    ui.neteaseLoginCard?.classList.remove('status-ok');
    ui.neteaseLoginCard?.classList.add('status-bad');
    state.neteasePollingTimer = setInterval(checkLoginStatus, 2000);
  } catch (err) {
    console.error('login qr error', err);
    showToast('获取二维码失败');
  }
}

async function checkLoginStatus() {
  if (!state.currentLoginKey) return;
  try {
    const json = await api.netease.loginCheck(state.currentLoginKey);
    if (!json.success) return;
    if (json.status === 'authorized') {
      showToast('登录成功');
      if (ui.neteaseLoginStatus) ui.neteaseLoginStatus.innerText = '已登录';
      ui.neteaseLoginCard?.classList.remove('status-bad');
      ui.neteaseLoginCard?.classList.add('status-ok');
      if (ui.neteaseLoginDesc) ui.neteaseLoginDesc.innerText = '可以开始搜索或下载歌曲';
      ui.neteaseQrModal?.classList.remove('active');
      refreshLoginStatus();
      if (state.neteasePollingTimer) { clearInterval(state.neteasePollingTimer); state.neteasePollingTimer = null; }
    } else if (json.status === 'expired') {
      showToast('二维码已过期，请重新获取');
      if (ui.neteaseQrHint) ui.neteaseQrHint.innerText = '二维码已过期，请重新获取';
      if (state.neteasePollingTimer) { clearInterval(state.neteasePollingTimer); state.neteasePollingTimer = null; }
    } else if (json.status === 'scanned') {
      if (ui.neteaseLoginStatus) ui.neteaseLoginStatus.innerText = '已扫码，等待确认...';
      if (ui.neteaseLoginDesc) ui.neteaseLoginDesc.innerText = '请在网易云确认登录';
    }
  } catch (err) {
    console.error('check login error', err);
  }
}

async function parseNeteaseLink() {
  const linkVal = ui.neteaseLinkInput ? ui.neteaseLinkInput.value.trim() : '';
  if (!linkVal) { showToast('请输入网易云链接或ID'); return; }
  if (ui.neteaseResultList) ui.neteaseResultList.innerHTML = '<div class="loading-text">解析中...</div>';
  try {
    const json = await api.netease.resolve(linkVal);
    if (!json.success) {
      showToast(json.error || '解析失败');
      if (ui.neteaseResultList) ui.neteaseResultList.innerHTML = `<div class="loading-text">${json.error || '解析失败'}</div>`;
      return;
    }
    state.neteaseResults = json.data || [];
    state.neteaseSelected = new Set(state.neteaseResults.map(s => String(s.id)));
    renderNeteaseResults();
    if (!state.neteaseResults.length) {
      if (ui.neteaseResultList) ui.neteaseResultList.innerHTML = '<div class="loading-text">未找到歌曲</div>';
    } else {
      const msg = json.type === 'playlist'
        ? `已解析歌单${json.name ? `：${json.name}` : ''}（${state.neteaseResults.length} 首）`
        : `解析到 ${state.neteaseResults.length} 首歌曲，可选择下载`;
      showToast(msg);
    }
  } catch (err) {
    console.error('parse link error', err);
    showToast('解析失败');
    if (ui.neteaseResultList) ui.neteaseResultList.innerHTML = '<div class="loading-text">解析失败</div>';
  }
}

async function bulkDownloadSelected() {
  const level = ui.neteaseQualitySelect ? ui.neteaseQualitySelect.value : 'exhigh';
  const targets = state.neteaseResults.filter(s => state.neteaseSelected.has(String(s.id)));
  if (!targets.length) { showToast('请先选择歌曲'); return; }
  for (const s of targets) {
    await downloadNeteaseSong({ ...s, level });
  }
}

function toggleNeteaseGate(enabled) {
  ui.neteaseConfigGate?.classList.toggle('hidden', enabled);
  ui.neteaseContent?.classList.toggle('hidden', !enabled);
}

function bindEvents() {
  ui.neteaseSearchBtn?.addEventListener('click', searchNeteaseSongs);
  ui.neteaseKeywordsInput?.addEventListener('keydown', (e) => { if (e.key === 'Enter') searchNeteaseSongs(); });
  ui.neteaseLoginBtn?.addEventListener('click', startNeteaseLogin);
  ui.closeQrModalBtn?.addEventListener('click', () => {
    ui.neteaseQrModal?.classList.remove('active');
    if (state.neteasePollingTimer) { clearInterval(state.neteasePollingTimer); state.neteasePollingTimer = null; }
  });
  ui.neteaseRefreshStatusBtn?.addEventListener('click', () => refreshLoginStatus(true));
  ui.neteaseIdDownloadBtn?.addEventListener('click', parseNeteaseLink);
  ui.neteaseLinkInput?.addEventListener('keydown', (e) => { if (e.key === 'Enter') parseNeteaseLink(); });
  ui.neteaseSaveDirBtn?.addEventListener('click', saveNeteaseConfig);
  if (ui.neteaseSelectAll) ui.neteaseSelectAll.addEventListener('change', (e) => {
    if (e.target.checked) state.neteaseSelected = new Set(state.neteaseResults.map(s => String(s.id)));
    else state.neteaseSelected.clear();
    renderNeteaseResults();
  });
  ui.neteaseBulkDownloadBtn?.addEventListener('click', bulkDownloadSelected);
  ui.neteaseDownloadToggle && ui.neteaseDownloadPanel && ui.neteaseDownloadToggle.addEventListener('click', () => {
    ui.neteaseDownloadPanel.classList.add('hidden');
  });
  ui.neteaseDownloadFloating && ui.neteaseDownloadPanel && ui.neteaseDownloadFloating.addEventListener('click', () => {
    ui.neteaseDownloadPanel.classList.toggle('hidden');
  });
  ui.neteaseApiGateBtn?.addEventListener('click', bindNeteaseApi);
  if (ui.neteaseChangeApiBtn) ui.neteaseChangeApiBtn.addEventListener('click', () => toggleNeteaseGate(false));
  ui.neteaseOpenConfigBtn.addEventListener('click', () => {
    ui.neteaseApiGateInput.focus();
    ui.neteaseApiGateInput.scrollIntoView({ behavior: 'smooth', block: 'center' });
  });
}

// 自动安装按钮事件
// 自动安装按钮事件
// 自动安装按钮事件
const installBtn = document.getElementById('netease-api-install-btn');
const progressContainer = document.getElementById('install-progress-container');
const progressBar = document.getElementById('install-progress-bar');
const stepText = document.getElementById('install-step-text');
const percentText = document.getElementById('install-percent-text');

if (installBtn) {
  installBtn.addEventListener('click', () => {
    // 使用自定义确认框
    showConfirmDialog(
      '确认安装',
      '确定要尝试安装并启动 API 服务容器吗？。',
      async () => {
        // Confirm Callback
        installBtn.disabled = true;
        installBtn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> 正在启动...';
        if (progressContainer) progressContainer.classList.remove('hidden');

        try {
          const res = await api.netease.installService();
          if (res.success) {
            // 开始轮询进度
            const pollTimer = setInterval(async () => {
              try {
                const statusRes = await api.netease.getInstallStatus();
                const { status, progress, step, error } = statusRes;

                // 更新 UI
                if (progressBar) progressBar.style.width = `${progress}%`;
                if (percentText) percentText.innerText = `${progress}%`;
                if (stepText) stepText.innerText = step || '进行中...';

                if (status === 'success') {
                  clearInterval(pollTimer);
                  installBtn.innerHTML = '<i class="fas fa-check"></i> 安装完成';
                  showToast('服务已就绪，正在自动连接...', 'success');

                  // 自动填充地址并连接
                  if (ui.neteaseApiGateInput) ui.neteaseApiGateInput.value = 'http://localhost:23236';
                  setTimeout(() => {
                    if (ui.neteaseApiGateBtn) ui.neteaseApiGateBtn.click();
                  }, 1000);

                } else if (status === 'error') {
                  clearInterval(pollTimer);
                  installBtn.disabled = false;
                  installBtn.innerHTML = '<i class="fas fa-magic"></i> 重试安装';
                  showToast(`安装出错: ${error}`, 'error');
                }
              } catch (e) {
                console.error("轮询状态失败", e);
              }
            }, 1000);
          } else {
            showToast(res.error || '请求失败', 'error');
            installBtn.disabled = false;
            installBtn.innerHTML = '<i class="fas fa-magic"></i> 一键安装 & 连接';
            if (progressContainer) progressContainer.classList.add('hidden');
          }
        } catch (e) {
          console.error(e);
          showToast('请求异常', 'error');
          installBtn.disabled = false;
          installBtn.innerHTML = '<i class="fas fa-magic"></i> 一键安装 & 连接';
          if (progressContainer) progressContainer.classList.add('hidden');
        }
      }
    );
  });
}

export async function initNetease(onRefreshSongs) {
  songRefreshCallback = onRefreshSongs;
  bindEvents();
  await loadNeteaseConfig();
  renderDownloadTasks();
}
