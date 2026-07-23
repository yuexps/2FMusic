import { sha256 as jsSha256 } from 'js-sha256'

/**
 * 密码学工具函数库
 * 提供跨 HTTP/HTTPS 场景兼容的 SHA-256 散列计算
 */

/**
 * 校验当前浏览器是否支持 Web Crypto 原生 SHA-256 (仅 Secure Context / HTTPS / localhost 可用)
 */
export function isNativeCryptoSupported(): boolean {
  return typeof window !== 'undefined' && !!(window.crypto && window.crypto.subtle && typeof window.crypto.subtle.digest === 'function')
}

/**
 * 纯 JavaScript 实现的 SHA-256 降级算法 (使用权威开源 js-sha256 库)
 * 用于非安全上下文 (HTTP 环境下 window.crypto.subtle 为 undefined 的情况)
 */
export function sha256PureJS(message: string): string {
  return jsSha256(message)
}

/**
 * 异步计算 SHA-256 哈希值 (优先原生 Web Crypto API，HTTP 非安全上下文自动降级至 js-sha256)
 */
export function sha256(message: string): Promise<string> {
  if (isNativeCryptoSupported()) {
    const msgBuffer = new TextEncoder().encode(message)
    return crypto.subtle.digest('SHA-256', msgBuffer).then(hashBuffer => {
      const hashArray = Array.from(new Uint8Array(hashBuffer))
      return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
    }).catch(() => sha256PureJS(message))
  }
  console.info('[Crypto] 原生 Web Crypto 不可用 (HTTP非安全上下文)，已自动降级至 js-sha256')
  return Promise.resolve(sha256PureJS(message))
}
