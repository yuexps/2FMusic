// 后端 API 封装
const jsonOrThrow = async (resp) => {
  const data = await resp.json();
  return data;
};

export const api = {
  library: {
    async list() {
      const res = await fetch('/api/music');
      return jsonOrThrow(res);
    },
    async deleteFile(filename) {
      const encodedName = encodeURIComponent(filename);
      const res = await fetch(`/api/music/delete/${encodedName}`, { method: 'DELETE' });
      return jsonOrThrow(res);
    },
    async importPath(path) {
      const res = await fetch('/api/music/import_path', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path })
      });
      return jsonOrThrow(res);
    },
    async externalMeta(path) {
      const res = await fetch(`/api/music/external/meta?path=${encodeURIComponent(path)}`);
      return jsonOrThrow(res);
    },
    async lyrics(query) {
      const res = await fetch(`/api/music/lyrics${query}`);
      return jsonOrThrow(res);
    },
    async albumArt(query) {
      const res = await fetch(`/api/music/album-art${query}`);
      return jsonOrThrow(res);
    }
  },
  system: {
    async status() {
      const res = await fetch('/api/system/status');
      return jsonOrThrow(res);
    }
  },
  mount: {
    async list() {
      const res = await fetch('/api/mount_points');
      return jsonOrThrow(res);
    },
    async add(path) {
      const res = await fetch('/api/mount_points', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path })
      });
      return jsonOrThrow(res);
    },
    async remove(path) {
      const res = await fetch('/api/mount_points', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path })
      });
      return jsonOrThrow(res);
    }
  },
  netease: {
    async search(keywords) {
      const res = await fetch(`/api/netease/search?keywords=${encodeURIComponent(keywords)}`);
      return jsonOrThrow(res);
    },
    async resolve(input) {
      const res = await fetch(`/api/netease/resolve?input=${encodeURIComponent(input)}`);
      return jsonOrThrow(res);
    },
    async download(body) {
      const res = await fetch('/api/netease/download', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body)
      });
      return jsonOrThrow(res);
    },
    async configGet() {
      const res = await fetch('/api/netease/config');
      return jsonOrThrow(res);
    },
    async configSave(payload) {
      const res = await fetch('/api/netease/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      return jsonOrThrow(res);
    },
    async loginStatus() {
      const res = await fetch('/api/netease/login/status');
      return jsonOrThrow(res);
    },
    async loginQr() {
      const res = await fetch('/api/netease/login/qrcode');
      return jsonOrThrow(res);
    },
    async loginCheck(key) {
      const res = await fetch(`/api/netease/login/check?key=${encodeURIComponent(key)}`);
      return jsonOrThrow(res);
    },
    async logout() {
      const res = await fetch('/api/netease/logout', { method: 'POST' });
      return jsonOrThrow(res);
    },
    async playlist(id) {
      const res = await fetch(`/api/netease/playlist?id=${encodeURIComponent(id)}`);
      return jsonOrThrow(res);
    },
    async song(id) {
      const res = await fetch(`/api/netease/song?id=${encodeURIComponent(id)}`);
      return jsonOrThrow(res);
    },
    async task(taskId) {
      const res = await fetch(`/api/netease/task/${encodeURIComponent(taskId)}`);
      return jsonOrThrow(res);
    },
    async installService() {
      const res = await fetch('/api/netease/install_service', { method: 'POST' });
      return jsonOrThrow(res);
    },
    async getInstallStatus() {
      const res = await fetch('/api/netease/install/status');
      return jsonOrThrow(res);
    },
    async recommend() {
      const res = await fetch('/api/netease/recommend');
      return jsonOrThrow(res);
    }
  }
};
