// 后端 API 封装
const jsonOrThrow = async (resp) => {
    if (resp.status === 401) throw new Error("401");
    if (!resp.ok) throw new Error(`${resp.status} ${resp.statusText}`);
    const data = await resp.json();
    return data;
};
// 前端 fetch('index.cgi/api/...') 会被 index.cgi 拦截并转发到 localhost:23237
const API_BASE = "index.cgi";

export const api = {
    API_BASE,
    library: {
        async list() {
            const res = await fetch(`${API_BASE}/api/music`);
            return jsonOrThrow(res);
        },
        async externalMeta(path) {
            const res = await fetch(`${API_BASE}/api/music/external/meta?path=${encodeURIComponent(path)}`);
            return jsonOrThrow(res);
        },
        async clearMetadata(id) {
            const res = await fetch(`${API_BASE}/api/music/clear_metadata/${encodeURIComponent(id)}`, { method: 'POST' });
            return jsonOrThrow(res);
        },
        async clearMetadataExternal(path) {
            const res = await fetch(`${API_BASE}/api/music/clear_metadata`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path })
            });
            return jsonOrThrow(res);
        },
        async lyrics(query) {
            const res = await fetch(`${API_BASE}/api/music/lyrics${query}`);
            return jsonOrThrow(res);
        },
        async albumArt(query) {
            const res = await fetch(`${API_BASE}/api/music/album-art${query}`);
            return jsonOrThrow(res);
        },
        async importPath(path) {
            const res = await fetch(`${API_BASE}/import_mode`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path: path })
            });
            return jsonOrThrow(res);
        }
    },
    async login(password) {
        const formData = new URLSearchParams();
        formData.append('password', password);
        const res = await fetch(`${API_BASE}/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: formData
        });
        if (res.url.includes('/login') || res.status === 401) {
            throw new Error("密码错误");
        }
        return true;
    },
    favorites: {
        async list() {
            const res = await fetch(`${API_BASE}/api/favorites`);
            return jsonOrThrow(res);
        },
        async add(id) {
            const res = await fetch(`${API_BASE}/api/favorites/${encodeURIComponent(id)}`, { method: 'POST' });
            return jsonOrThrow(res);
        },
        async remove(id) {
            const res = await fetch(`${API_BASE}/api/favorites/${encodeURIComponent(id)}`, { method: 'DELETE' });
            return jsonOrThrow(res);
        }
    }
};
