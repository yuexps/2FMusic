import { state, persistState } from './state.js';
import { ui } from './ui.js';
import { autoResizeUI, showToast, persistOnUnload } from './utils.js';
import { api } from './api.js';
import { initNetease } from './netease.js';
import { initMounts, loadMountPoints, startScanPolling } from './mounts.js';
import { initPlayer, loadSongs, performDelete, handleExternalFile, renderPlaylist } from './player.js';

document.addEventListener('DOMContentLoaded', async () => {
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
  const uploadTargetInput = document.getElementById('upload-target-input');
  const uploadDropzone = document.getElementById('upload-dropzone');
  const uploadChooseBtn = document.getElementById('upload-choose-btn');
  const uploadStatus = document.getElementById('upload-status');
  const views = ['view-player', 'view-mount', 'view-netease', 'view-upload'];

  function switchView(targetId) {
    views.forEach(id => {
      const el = document.getElementById(id);
      if (el) el.classList.toggle('hidden', id !== targetId);
    });
  }

  function setActiveNav(targetEl) {
    document.querySelectorAll('aside nav li').forEach(li => li.classList.remove('active'));
    if (targetEl) targetEl.classList.add('active');
  }

  if (ui.navUpload && uploadView && ui.fileUpload) {
    const populateUploadTargets = async () => {
      try {
        const res = await api.mount.list();
        if (res.success && Array.isArray(res.data) && uploadTargetInput) {
          // 清除旧项，保留默认
          uploadTargetInput.innerHTML = '<option value="">默认音乐库</option>';
          res.data.forEach(path => {
            const opt = document.createElement('option');
            opt.value = path;
            opt.innerText = path;
            uploadTargetInput.appendChild(opt);
          });
        }
      } catch (e) { console.error('加载上传目录失败', e); }
    };

    ui.navUpload.addEventListener('click', () => {
      switchView('view-upload');
      setActiveNav(ui.navUpload);
      populateUploadTargets();
      if (window.innerWidth <= 768 && ui.sidebar?.classList.contains('open')) {
        ui.sidebar.classList.remove('open');
      }
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
            const targetLabel = (uploadTargetInput && uploadTargetInput.value) ? uploadTargetInput.options[uploadTargetInput.selectedIndex].text || uploadTargetInput.value : '默认音乐库';
            showToast(`已上传 1 首音乐至 ${targetLabel}`);
            loadSongs();
          } else uploadStatus.innerText = '失败: ' + (data.error || '未知错误');
        } else uploadStatus.innerText = '服务器错误';
        ui.fileUpload.value = '';
      };
      xhr.onerror = () => { uploadStatus.innerText = '网络连接失败'; };
      xhr.open('POST', '/api/music/upload', true);
      xhr.send(formData);
    };

    uploadChooseBtn?.addEventListener('click', () => ui.fileUpload.click());
    ui.fileUpload.addEventListener('change', () => handleFile(ui.fileUpload.files[0]));
    uploadDropzone?.addEventListener('dragover', (e) => { e.preventDefault(); uploadDropzone.classList.add('dragging'); });
    uploadDropzone?.addEventListener('dragleave', () => uploadDropzone.classList.remove('dragging'));
    uploadDropzone?.addEventListener('drop', (e) => {
      e.preventDefault();
      uploadDropzone.classList.remove('dragging');
      const file = e.dataTransfer.files[0];
      handleFile(file);
    });
  }

  // 其他导航：回到播放器或对应视图
  if (ui.navLocal) ui.navLocal.addEventListener('click', () => { switchView('view-player'); setActiveNav(ui.navLocal); });
  if (ui.navFav) ui.navFav.addEventListener('click', () => { switchView('view-player'); setActiveNav(ui.navFav); });
  if (ui.navMount) ui.navMount.addEventListener('click', () => { switchView('view-mount'); setActiveNav(ui.navMount); });
  if (ui.navNetease) ui.navNetease.addEventListener('click', () => { switchView('view-netease'); setActiveNav(ui.navNetease); });

  // 初始化模块
  initMounts(loadSongs);
  await initPlayer();   // 优先初始化播放器，确保缓存秒开
  await initNetease(loadSongs);
  loadMountPoints();
  startScanPolling(false, loadSongs, loadMountPoints);
});
