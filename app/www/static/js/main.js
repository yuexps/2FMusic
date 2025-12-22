import { state, persistState } from './state.js';
import { ui } from './ui.js';
import { autoResizeUI, showToast, persistOnUnload, showConfirmDialog } from './utils.js';
import { api } from './api.js';
import { initNetease } from './netease.js';
import { initMounts, loadMountPoints, startScanPolling } from './mounts.js';
import { initPlayer, loadSongs, performDelete, handleExternalFile, renderPlaylist, switchTab } from './player.js';

document.addEventListener('DOMContentLoaded', async () => {
  // 版本检查
  try {
    const VERSION_STORAGE_KEY = 'app_version';
    const currentVersion = localStorage.getItem(VERSION_STORAGE_KEY);
    const response = await api.system.versionCheck();
    
    if (response && response.version) {
      // 如果是首次访问或版本号变化
      if (!currentVersion || currentVersion !== response.version) {
        console.log('前端已过时！\n最新版本：' + response.version, '\n当前版本：' + currentVersion);
        showToast('检测到新版本，正在更新页面...');
        
        // 更新本地存储的版本号
        localStorage.setItem(VERSION_STORAGE_KEY, response.version);
        
        // 强制清除浏览器缓存并刷新页面
        window.location.reload(true);
      }
      else {
        console.log('前端已是最新！\n最新版本：' + response.version, '\n当前版本：' + currentVersion);
      }
    }
  } catch (error) {
    console.error('版本检查失败:', error);
    showToast('版本检查失败，请刷新页面重试', 'error');
  }
  
  // UI 适配与基础防护
  autoResizeUI();
  window.addEventListener('resize', () => {
    requestAnimationFrame(autoResizeUI);
  });
  window.addEventListener('error', function (e) { if (e.target.tagName === 'IMG') e.target.src = '/static/images/ICON_256.PNG'; }, true);
  document.addEventListener('contextmenu', (e) => { if (e.target.tagName !== 'INPUT' && e.target.tagName !== 'TEXTAREA') e.preventDefault(); });
  persistOnUnload(ui.audio);

  // 网易云 Tab 切换
  document.querySelectorAll('.tab-btn').forEach(btn => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));
      btn.classList.add('active');
      const targetId = btn.getAttribute('data-target');
      const targetContent = document.getElementById(targetId);
      if (targetContent) targetContent.classList.add('active');
    });
  });

  // 上传页面
  const uploadView = document.getElementById('view-upload');
  const uploadTargetInput = document.getElementById('upload-target-input'); // hidden value
  const uploadTargetSelect = document.getElementById('upload-target-select');
  const uploadTargetCurrent = document.getElementById('upload-target-current');
  const uploadTargetList = document.getElementById('upload-target-list');
  const uploadDropzone = document.getElementById('upload-dropzone');
  const uploadChooseBtn = document.getElementById('upload-choose-btn');
  const uploadStatus = document.getElementById('upload-status');
  // const views = ['view-player', 'view-mount', 'view-netease', 'view-upload', 'view-settings']; - REMOVED

  function setUploadTarget(value, label) {
    if (uploadTargetInput) uploadTargetInput.value = value || '';
    if (uploadTargetCurrent) {
      uploadTargetCurrent.dataset.value = value || '';
      uploadTargetCurrent.textContent = label || '默认音乐库';
    }
    if (uploadTargetList) uploadTargetList.classList.add('hidden');
    uploadTargetSelect?.classList.remove('open');
  }

  if (ui.navUpload && uploadView && ui.fileUpload) {
    const populateUploadTargets = async () => {
      try {
        const res = await api.mount.list();
        if (res.success && Array.isArray(res.data) && uploadTargetList) {
          uploadTargetList.innerHTML = '';
          const all = [{ value: '', label: '默认音乐库' }, ...res.data.map(p => ({ value: p, label: p }))];
          all.forEach(item => {
            const option = document.createElement('div');
            option.className = 'upload-select-option';
            option.dataset.value = item.value;
            option.innerText = item.label;
            option.onclick = (e) => { e.stopPropagation(); setUploadTarget(item.value, item.label); };
            uploadTargetList.appendChild(option);
          });
        }
      } catch (e) { console.error('加载上传目录失败', e); }
    };

    ui.navUpload.addEventListener('click', () => {
      switchTab('upload'); // Used switchTab
      populateUploadTargets();
      // Mobile sidebar handled in switchTab
    });

    const handleFile = (file) => {
      if (!file) return;
      if (!file.name.match(/\.(mp3|flac|wav|ogg|m4a)$/i)) { showToast('仅支持音频文件'); return; }
      const formData = new FormData();
      formData.append('file', file);
      if (uploadTargetInput?.value) formData.append('target_dir', uploadTargetInput.value.trim());
      uploadStatus.innerText = '上传中...';
      const xhr = new XMLHttpRequest();
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable && uploadStatus) {
          const percent = Math.round((e.loaded / e.total) * 100);
          uploadStatus.innerText = `上传中... ${percent}%`;
        }
      };
      xhr.onload = () => {
        if (xhr.status === 200) {
          const data = JSON.parse(xhr.responseText);
          if (data.success) {
            uploadStatus.innerText = '上传成功';
            const targetLabel = uploadTargetCurrent ? uploadTargetCurrent.textContent : '默认音乐库';
            showToast(`已上传 1 首音乐至 ${targetLabel}`);
            loadSongs(true, false);
          } else uploadStatus.innerText = '失败: ' + (data.error || '未知错误');
        } else uploadStatus.innerText = '服务器错误';
        ui.fileUpload.value = '';
      };
      xhr.onerror = () => { uploadStatus.innerText = '网络连接失败'; };
      xhr.open('POST', '/api/music/upload', true);
      xhr.send(formData);
    };

    uploadChooseBtn?.addEventListener('click', () => ui.fileUpload?.click());
    uploadDropzone?.addEventListener('dragover', (e) => { e.preventDefault(); uploadDropzone?.classList.add('drag-over'); });
    uploadDropzone?.addEventListener('dragleave', () => uploadDropzone?.classList.remove('drag-over'));
    uploadDropzone?.addEventListener('drop', (e) => { 
      e.preventDefault(); 
      uploadDropzone?.classList.remove('drag-over'); 
      if (e.dataTransfer?.files?.[0]) handleFile(e.dataTransfer.files[0]); 
    });
    ui.fileUpload?.addEventListener('change', (e) => { 
      if (e.target?.files?.[0]) handleFile(e.target.files[0]); 
    });

    // 自定义下拉选择行为
    uploadTargetSelect?.addEventListener('click', (e) => {
      e.stopPropagation();
      uploadTargetList?.classList.toggle('hidden');
      uploadTargetSelect.classList.toggle('open');
    });
    document.addEventListener('click', () => {
      uploadTargetList?.classList.add('hidden');
      uploadTargetSelect?.classList.remove('open');
    });
    // 初始化默认
    setUploadTarget('', '默认音乐库');
  }

  // 其他导航：回到播放器或对应视图
  if (ui.navLocal) ui.navLocal.addEventListener('click', () => { switchTab('local'); });
  if (ui.navFav) ui.navFav.addEventListener('click', () => { switchTab('fav'); });
  if (ui.navMount) ui.navMount.addEventListener('click', () => { switchTab('mount'); });
  if (ui.navNetease) ui.navNetease.addEventListener('click', () => { switchTab('netease'); });
  if (ui.navSettings) ui.navSettings.addEventListener('click', () => { switchTab('settings'); });

  // Settings Logic
  function initSettings() {
    if (!ui.scaleInput) return;

    const updateSliderVisual = (val) => {
      // Map 0.6-1.4 to 0-100%
      const min = 0.6, max = 1.4;
      const pct = ((val - min) / (max - min)) * 100;
      ui.scaleInput.style.backgroundSize = `${pct}% 100%`;
    };

    const updateLabel = (val) => {
      if (ui.scaleValue) ui.scaleValue.innerText = val ? parseFloat(val).toFixed(2) : '自动';
      if (val) updateSliderVisual(parseFloat(val));
    };

    // Load initial
    const saved = localStorage.getItem('2fmusic_ui_scale');
    if (saved) {
      ui.scaleInput.value = saved;
      updateLabel(saved);
    } else {
      // If auto, set slider to computed or 1.0 (approximated)
      const current = getComputedStyle(document.documentElement).getPropertyValue('--ui-scale').trim();
      const val = parseFloat(current) || 1.0;
      ui.scaleInput.value = val;
      updateSliderVisual(val);
      if (ui.scaleValue) ui.scaleValue.innerText = '自动';
    }

    // 拖动过程中只更新滑块视觉效果，不应用缩放
    ui.scaleInput.addEventListener('input', (e) => {
      const val = e.target.value;
      // 只更新滑块视觉效果
      updateLabel(val);
    });

    // 用户放下拖动条时应用缩放并保存设置
    ui.scaleInput.addEventListener('change', (e) => {
      const val = e.target.value;
      // 应用缩放效果
      document.documentElement.style.setProperty('--ui-scale', val);
      // 保存到本地存储
      localStorage.setItem('2fmusic_ui_scale', val);
    });

    ui.scaleReset?.addEventListener('click', () => {
      localStorage.removeItem('2fmusic_ui_scale');
      if (window.applyScale) window.applyScale(); // Re-trigger auto calc
      const current = getComputedStyle(document.documentElement).getPropertyValue('--ui-scale').trim();
      const val = parseFloat(current);
      ui.scaleInput.value = val;
      updateSliderVisual(val);
      if (ui.scaleValue) ui.scaleValue.innerText = '自动';
      if (ui.scaleValue) ui.scaleValue.innerText = '自动';
      showToast('已重置为自动缩放');
    });

    // Clear Cache
    document.getElementById('setting-clear-cache')?.addEventListener('click', () => {
      showConfirmDialog('彻底清除数据', '确定要删除本网站的所有本地数据吗？<br>包括缓存、Cookie、偏好设置等。页面将重新加载。', async () => {
        // 1. Clear Storage
        localStorage.clear();
        sessionStorage.clear();

        // 2. Clear Cookies
        document.cookie.split(";").forEach((c) => {
          document.cookie = c.replace(/^ +/, "").replace(/=.*/, "=;expires=" + new Date().toUTCString() + ";path=/");
        });

        // 3. Clear Cache Storage (Service Workers)
        if ('caches' in window) {
          try {
            const keys = await caches.keys();
            await Promise.all(keys.map(key => caches.delete(key)));
          } catch (e) {
            console.error("Failed to clear caches:", e);
          }
        }

        // 4. Force Reload
        location.reload(true);
      });
    });

    // Logout
    document.getElementById('setting-logout')?.addEventListener('click', () => {
      showConfirmDialog('退出登录', '确定要退出当前登录吗？', () => {
        window.location.href = '/logout';
      });
    });
  }
  initSettings();

  // 初始化模块
  initMounts(loadSongs);
  
  try {
    await initPlayer();
    console.log('[Main] Player initialized');
  } catch (e) {
    console.error('[Main] Failed to initialize player:', e);
  }

  // 检查是否需要特别处理收藏夹详情页
  if (state.currentTab === 'fav' && state.selectedPlaylistId) {
    // 如果在收藏夹详情页，延迟一点时间再渲染，确保DOM完全加载
    setTimeout(() => {
      renderPlaylist();
    }, 100);
  }

  await initNetease(loadSongs);
  loadMountPoints();
  startScanPolling(false, (r) => loadSongs(r, false), loadMountPoints);
});