import { state, persistState } from './state.js';
import { ui } from './ui.js';
import { autoResizeUI, showToast, persistOnUnload } from './utils.js';
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

  // 上传
  if (ui.navUpload && ui.fileUpload) {
    ui.navUpload.addEventListener('click', () => {
      ui.fileUpload.click();
      if (window.innerWidth <= 768 && ui.sidebar?.classList.contains('open')) {
        ui.sidebar.classList.remove('open');
      }
    });
    ui.fileUpload.addEventListener('change', () => {
      const file = ui.fileUpload.files[0]; if (!file) return;
      if (!file.name.match(/\.(mp3|flac|wav|ogg|m4a)$/i)) { showToast('仅支持音频文件'); return; }
      if (ui.uploadModal) ui.uploadModal.classList.add('active');
      if (ui.uploadFileName) ui.uploadFileName.innerText = file.name;
      if (ui.uploadFill) ui.uploadFill.style.width = '0%';
      if (ui.uploadPercent) ui.uploadPercent.innerText = '0%';
      if (ui.uploadMsg) ui.uploadMsg.innerText = '正在上传...';
      if (ui.closeUploadBtn) ui.closeUploadBtn.style.display = 'none';
      const formData = new FormData(); formData.append('file', file);
      const xhr = new XMLHttpRequest();
      xhr.upload.onprogress = (e) => { if (e.lengthComputable) { const percent = Math.round((e.loaded / e.total) * 100); if (ui.uploadFill) ui.uploadFill.style.width = `${percent}%`; if (ui.uploadPercent) ui.uploadPercent.innerText = `${percent}%`; } };
      xhr.onload = () => {
        if (xhr.status === 200) {
          const data = JSON.parse(xhr.responseText);
          if (data.success) {
            if (ui.uploadFill) ui.uploadFill.style.width = '100%';
            if (ui.uploadPercent) ui.uploadPercent.innerText = '100%';
            if (ui.uploadMsg) ui.uploadMsg.innerText = '上传成功!';
            setTimeout(() => { ui.uploadModal?.classList.remove('active'); loadSongs(); }, 1000);
          } else { if (ui.uploadMsg) ui.uploadMsg.innerText = '失败: ' + (data.error || '未知错误'); if (ui.closeUploadBtn) ui.closeUploadBtn.style.display = 'inline-block'; }
        } else { if (ui.uploadMsg) ui.uploadMsg.innerText = '服务器错误'; if (ui.closeUploadBtn) ui.closeUploadBtn.style.display = 'inline-block'; }
      };
      xhr.onerror = () => { if (ui.uploadMsg) ui.uploadMsg.innerText = '网络连接失败'; if (ui.closeUploadBtn) ui.closeUploadBtn.style.display = 'inline-block'; };
      xhr.open('POST', '/api/music/upload', true); xhr.send(formData); ui.fileUpload.value = '';
    });
    if (ui.closeUploadBtn) ui.closeUploadBtn.addEventListener('click', () => ui.uploadModal?.classList.remove('active'));
  }

  // 初始化模块
  initMounts(loadSongs);
  await initPlayer();   // 优先初始化播放器，确保缓存秒开
  await initNetease(loadSongs);
  loadMountPoints();
  startScanPolling(false, loadSongs, loadMountPoints);
});
