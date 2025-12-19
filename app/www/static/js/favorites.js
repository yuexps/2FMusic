import { state, saveFavorites } from './state.js';
import { ui } from './ui.js';
import { api } from './api.js';
import { showToast, updateDetailFavButton, extractColorFromImage } from './utils.js';
import { renderPlaylist } from './player.js';

// 加载收藏夹筛选列表
export async function loadPlaylistFilter() {
  if (!ui.playlistFilter) return;
  
  try {
    const res = await api.favoritePlaylists.list();
    if (res.success && res.data) {
      // 去重处理
      const uniquePlaylists = [];
      const seenIds = new Set();
      
      res.data.forEach(playlist => {
        if (!seenIds.has(playlist.id)) {
          seenIds.add(playlist.id);
          uniquePlaylists.push(playlist);
        }
      });
      
      // 清空现有选项，保留"所有收藏夹"
      ui.playlistFilter.innerHTML = '<option value="">所有收藏夹</option>';
      
      // 添加收藏夹选项
      uniquePlaylists.forEach(playlist => {
        const option = document.createElement('option');
        option.value = playlist.id;
        option.textContent = playlist.name + (playlist.is_default ? ' (默认)' : '');
        ui.playlistFilter.appendChild(option);
      });
    }
  } catch (e) {
    console.error('加载收藏夹列表失败:', e);
  }
}

// 处理收藏夹筛选变化
export function handlePlaylistFilterChange() {
  if (!ui.playlistFilter) return;
  
  const selectedPlaylistId = ui.playlistFilter.value;
  renderPlaylist(); // 注意：renderPlaylist 仍在 player.js 中定义
}

// 加载指定收藏夹的歌曲
export async function loadPlaylistSongs(playlistId) {
  if (!ui.songContainer) return;
  
  try {
    const res = await api.favoritePlaylists.getSongs(playlistId);
    if (res.success && res.data) {
      // 获取收藏夹中的歌曲ID列表
      const playlistSongIds = new Set(res.data);
      
      // 这里的逻辑需要根据实际情况调整，可能需要与 player.js 中的 renderPlaylist 配合使用
      // 暂时保留这个函数的骨架
    }
  } catch (e) {
    console.error('加载收藏夹歌曲失败:', e);
  }
}

// 显示收藏夹选择对话框
export function showPlaylistSelectDialog(song, btnEl) {
  // 辅助函数：加深颜色
  function darkenColor(color, amount = 0.3) {
    return {
      r: Math.max(0, Math.floor(color.r * (1 - amount))),
      g: Math.max(0, Math.floor(color.g * (1 - amount))),
      b: Math.max(0, Math.floor(color.b * (1 - amount))),
      toString: function() {
        return `rgba(${this.r}, ${this.g}, ${this.b}, 0.9)`;
      }
    };
  }
  
  // 创建独立的收藏夹选择对话框
  const dialog = document.createElement('div');
  dialog.className = 'playlist-select-dialog';
  
  dialog.innerHTML = `
    <div class="dialog-content">
      <div class="dialog-header">
        <h3>选择收藏夹</h3>
        <button id="close-btn" class="close-btn">&times;</button>
      </div>
      <div class="playlists-container"></div>
      <div class="dialog-actions">
        <button id="confirm-btn" class="btn-primary">确定</button>
      </div>
    </div>
  `;
  
  // 自动从歌曲封面提取颜色并应用到对话框背景
  const coverUrl = song.cover || '/static/images/ICON_256.PNG';
  const img = new Image();
  img.crossOrigin = 'Anonymous';
  
  // 先获取收藏夹列表，然后再应用颜色并显示对话框
  api.favoritePlaylists.list().then(res => {
    if (res.success) {
      // 去重处理
      const uniquePlaylists = [];
      const seenIds = new Set();
      
      res.data.forEach(playlist => {
        if (!seenIds.has(playlist.id)) {
          seenIds.add(playlist.id);
          uniquePlaylists.push(playlist);
        }
      });
      
      const container = dialog.querySelector('.playlists-container');
      container.innerHTML = '';
      
      uniquePlaylists.forEach(playlist => {
        const item = document.createElement('div');
        item.className = 'playlist-item';
        item.innerHTML = `
          <input type="radio" name="playlist" id="playlist-${playlist.id}" value="${playlist.id}" ${playlist.is_default ? 'checked' : ''}>
          <label for="playlist-${playlist.id}">${playlist.name} ${playlist.is_default ? '(默认)' : ''}</label>
        `;
        container.appendChild(item);
      });
    }
    
    // 列表加载完成后，开始加载封面图片并应用颜色
    img.onload = () => {
      try {
        const color = extractColorFromImage(img);
        if (color) {
          // 应用提取的颜色到对话框内容区域背景
          const dialogContent = dialog.querySelector('.dialog-content');
          dialogContent.style.background = `rgba(${color.r}, ${color.g}, ${color.b}, 0.9)`;
          
          // 根据提取的颜色亮度调整文字颜色，确保可读性
          const brightness = (color.r * 299 + color.g * 587 + color.b * 114) / 1000;
          const textColor = brightness > 128 ? '#000' : '#fff';
          dialogContent.style.color = textColor;
          
          // 调整确认按钮颜色：使用提取颜色的加深版本，增加对比度
          const confirmBtn = dialog.querySelector('#confirm-btn');
          if (confirmBtn) {
            const darkColor = darkenColor(color, 0.4); // 加深40%，提高对比度
            confirmBtn.style.background = darkColor.toString();
            confirmBtn.style.color = textColor;
            confirmBtn.style.border = `1px solid rgba(${darkColor.r}, ${darkColor.g}, ${darkColor.b}, 0.5)`;
            confirmBtn.style.boxShadow = `0 2px 10px rgba(${darkColor.r}, ${darkColor.g}, ${darkColor.b}, 0.3)`;
            confirmBtn.style.fontWeight = '600';
          }
        }
      } catch (e) {
        console.error('提取颜色失败:', e);
      } finally {
        // 无论颜色提取是否成功，都显示对话框
        document.body.appendChild(dialog);
        // 清理临时图片
        img.remove();
      }
    };
    
    img.onerror = () => {
      console.error('加载封面图片失败');
      // 图片加载失败也显示对话框
      document.body.appendChild(dialog);
      img.remove();
    };
    
    img.src = coverUrl;
  });
  
  // 确认按钮事件
  const confirmBtn = dialog.querySelector('#confirm-btn');
  const closeBtn = dialog.querySelector('#close-btn');
  
  const confirmHandler = () => {
    const playlistId = dialog.querySelector('input[name="playlist"]:checked');
    if (playlistId) {
      addToSelectedPlaylist(song, playlistId.value, btnEl, dialog);
    } else {
      dialog.remove();
    }
  };
  
  const closeHandler = () => {
    dialog.remove();
  };
  
  const overlayHandler = (e) => {
    if (e.target === dialog) {
      dialog.remove();
    }
  };
  
  confirmBtn.addEventListener('click', confirmHandler);
  closeBtn.addEventListener('click', closeHandler);
  dialog.addEventListener('click', overlayHandler);
}

// 将歌曲添加到选中的收藏夹
export async function addToSelectedPlaylist(song, playlistId, btnEl, dialog) {
  state.favorites.add(song.id);
  if (btnEl) { btnEl.classList.add('active'); btnEl.innerHTML = '<i class="fas fa-heart"></i>'; }
  try { 
    await api.favorites.add(song.id, playlistId); 
  } catch (e) { 
    console.error(e); 
    // 回滚 UI
    state.favorites.delete(song.id);
    if (btnEl) { btnEl.classList.remove('active'); btnEl.innerHTML = '<i class="far fa-heart"></i>'; }
  }
  saveFavorites();

  const currentPlaying = state.playQueue[state.currentTrackIndex];
  if (currentPlaying && currentPlaying.id === song.id) {
    updateDetailFavButton(state.favorites.has(song.id));
  }
  
  // 关闭对话框
  dialog.remove();
}

// 显示创建新收藏夹对话框
export function showCreatePlaylistDialog() {
  // 创建独立的创建收藏夹对话框
  const dialog = document.createElement('div');
  dialog.className = 'playlist-select-dialog';
  
  dialog.innerHTML = `
    <div class="dialog-content">
      <div class="dialog-header">
        <h3>创建新收藏夹</h3>
        <button id="close-btn" class="close-btn">&times;</button>
      </div>
      <div class="dialog-body">
        <input type="text" id="playlist-name" placeholder="输入收藏夹名称" class="text-input">
      </div>
      <div class="dialog-actions">
        <button id="create-btn" class="btn-primary">创建</button>
      </div>
    </div>
  `;
  
  // 获取输入框焦点
  const playlistNameInput = dialog.querySelector('#playlist-name');
  
  // 确认按钮事件
  const createBtn = dialog.querySelector('#create-btn');
  const closeBtn = dialog.querySelector('#close-btn');
  
  const createHandler = async () => {
    const name = playlistNameInput.value.trim();
    if (!name) {
      // 空名称验证
      playlistNameInput.style.borderColor = '#ff4444';
      playlistNameInput.placeholder = '请输入收藏夹名称';
      return;
    }
    
    try {
      // 先获取所有收藏夹，检查名称是否已存在
      const listRes = await api.favoritePlaylists.list();
      if (listRes.success) {
        // 检查是否存在同名收藏夹
        const existingPlaylist = listRes.data.find(playlist => playlist.name === name);
        if (existingPlaylist) {
          playlistNameInput.style.borderColor = '#ff4444';
          showToast(`已存在名为"${name}"的收藏夹，请使用其他名称`, 'error');
          return;
        }
      }
      
      // 创建收藏夹
      const res = await api.favoritePlaylists.create(name);
      if (res.success) {
        // 创建成功后刷新收藏夹页面
        console.log('收藏夹创建成功:', res.data);
        renderPlaylist();
        showToast('收藏夹创建成功', 'success');
        dialog.remove();
      } else {
        console.error('创建收藏夹失败:', res.message);
        // 显示错误提示
        showToast(`创建收藏夹失败: ${res.message || '未知错误'}`, 'error');
      }
    } catch (e) {
      console.error('创建收藏夹失败:', e);
      // 显示错误提示
      showToast(`创建收藏夹失败: ${e.message || '网络错误'}`, 'error');
    }
  };
  
  const closeHandler = () => {
    dialog.remove();
  };
  
  const overlayHandler = (e) => {
    if (e.target === dialog) {
      dialog.remove();
    }
  };
  
  // 回车创建
  playlistNameInput.addEventListener('keypress', (e) => {
    if (e.key === 'Enter') {
      createHandler();
    }
  });
  
  createBtn.addEventListener('click', createHandler);
  closeBtn.addEventListener('click', closeHandler);
  dialog.addEventListener('click', overlayHandler);
  

  const dialogContent = dialog.querySelector('.dialog-content');
  const createBtnElement = dialog.querySelector('#create-btn');
  
  dialogContent.style.background = 'var(--glass-bg)';
  dialogContent.style.border = '1px solid var(--glass-border)';
  dialogContent.style.color = 'var(--text-main)';
  
  createBtnElement.style.background = 'var(--glass-bg)';
  createBtnElement.style.border = '1px solid var(--glass-border)';
  createBtnElement.style.color = 'var(--text-main)';
  createBtnElement.style.fontWeight = '600';
  
  // 显示对话框并设置焦点
  document.body.appendChild(dialog);
  playlistNameInput.focus();
}