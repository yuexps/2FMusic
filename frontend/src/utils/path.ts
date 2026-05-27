/**
 * 动态计算前端当前的 Base URL 前缀 (例如 '/2fmusic' 或 '')
 * 适用于在任意子路径下免配置重新打包部署
 */
export function getBaseUrl(): string {
  let path = window.location.pathname;
  // 过滤 index.html、login.html 等入口文件名
  path = path.replace(/\/(index|login)\.html$/, '');
  // 过滤 /login 单级路由
  path = path.replace(/\/(login)$/, '');
  
  if (!path.endsWith('/')) {
    path += '/';
  }
  
  if (path === '/') {
    return '';
  }
  
  // 去掉尾斜杠，返回如 '/2fmusic'
  return path.slice(0, -1);
}

/**
 * 将传入的相对或绝对路径转换为适配 Base URL 的完整路径
 */
export function getApiUrl(path: string): string {
  if (!path) return path;
  
  // 外部链接、blob URL、data URL 原样返回
  if (/^(https?:|blob:|data:)/i.test(path)) {
    return path;
  }
  
  const base = getBaseUrl();
  if (!base) return path;
  
  const normalizedPath = path.startsWith('/') ? path : '/' + path;
  
  // 幂等处理：避免重复拼接
  if (normalizedPath === base || normalizedPath.startsWith(base + '/')) {
    return normalizedPath;
  }
  
  return base + normalizedPath;
}