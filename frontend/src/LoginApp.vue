<script setup lang="ts">
import { ref } from 'vue'
import { getApiUrl } from './utils/path'
import {
  NConfigProvider,
  NInput,
  NButton,
  NCheckbox
} from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'

// 从全局 window 读取 Flask 注入的模板变量
const loginError = (window as any).loginError || ''
const actionUrl = (window as any).actionUrl || '/login'

const password = ref('')
const rememberChecked = ref(false)
const hashedPassword = ref('')
const loading = ref(false)
const showError = ref(!!loginError)

const formRef = ref<HTMLFormElement | null>(null)

// 自定义 Naive UI 主题
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#0066cc',
    primaryColorHover: '#0071e3',
    primaryColorPressed: '#005bbf',
    fontFamily: '"SF Pro Text", "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
    borderRadius: '11px',
  },
  Button: {
    borderRadiusMedium: '9999px',
    borderRadiusSmall: '8px',
    borderRadiusLarge: '9999px',
  },
  Input: {
    borderRadius: '11px'
  }
}

// 异步计算 SHA-256 密码哈希，确保网络传输安全
async function sha256(message: string): Promise<string> {
  const msgBuffer = new TextEncoder().encode(message)
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer)
  const hashArray = Array.from(new Uint8Array(hashBuffer))
  return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
}

const handleLogin = async (e: Event) => {
  e.preventDefault()
  if (!password.value) return
  loading.value = true
  showError.value = false

  try {
    hashedPassword.value = await sha256(password.value)
    // 延迟提交，让 Vue 能够完成对隐藏 Input 绑定的值同步
    setTimeout(() => {
      if (formRef.value) {
        formRef.value.submit()
      }
    }, 50)
  } catch (err) {
    console.error('Hashing failed, falling back to plaintext', err)
    hashedPassword.value = password.value
    setTimeout(() => {
      if (formRef.value) {
        formRef.value.submit()
      }
    }, 50)
  }
}
</script>

<template>
  <n-config-provider :theme-overrides="themeOverrides">
    <div class="login-container">
      <div class="login-card product-cover-shadow">
        <header class="login-header">
          <div class="logo-container">
            <img :src="getApiUrl('/ICON.PNG')" class="app-logo" width="60" height="60" alt="Logo" />
          </div>
          <h1 class="app-title">2FMusic</h1>
          <p class="app-subtitle">请输入您的访问密码以进入音乐库</p>
        </header>

        <!-- 错误状态展示 -->
        <transition name="fade">
          <div v-if="showError" class="error-banner">
            <span>{{ loginError }}</span>
          </div>
        </transition>

        <!-- Vue 渲染的交互表单 (拦截 submit 并执行哈希计算) -->
        <form class="interactive-form" @submit="handleLogin">
          <div class="form-group">
            <n-input v-model:value="password" type="password" show-password-on="click" placeholder="访问密码" size="large"
              autofocus />
          </div>

          <div class="form-options">
            <n-checkbox v-model:checked="rememberChecked">
              保持登录 (30天内免登)
            </n-checkbox>
          </div>

          <div class="form-action">
            <n-button type="primary" size="large" block attr-type="submit" :loading="loading">
              登录
            </n-button>
          </div>
        </form>

        <!-- 后端 Flask 接受的隐藏表单桥接 -->
        <form ref="formRef" method="POST" :action="actionUrl" style="display: none;">
          <input type="hidden" name="password" :value="hashedPassword">
          <input type="hidden" name="remember" :value="rememberChecked ? 'on' : ''">
        </form>
      </div>
    </div>
  </n-config-provider>
</template>

<style scoped>
.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  width: 100vw;
  /* 极简背光效果 */
  background: radial-gradient(circle at 50% 25%, rgba(0, 102, 204, 0.08), transparent 45%), var(--canvas-parchment);
  transition: background-color var(--transition-normal);
}

.login-card {
  width: min(390px, 90vw);
  padding: 42px 36px;
  border-radius: 20px;
  background: var(--surface-glass);
  backdrop-filter: saturate(180%) blur(20px);
  -webkit-backdrop-filter: saturate(180%) blur(20px);
  border: 1px solid var(--hairline);
  box-sizing: border-box;
  color: var(--ink);
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.login-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 8px;
}

.logo-container {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 8px;
}

.app-logo {
  border-radius: 14px;
  object-fit: cover;
  filter: drop-shadow(0 4px 12px rgba(0, 102, 204, 0.25));
}

.app-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: 26px;
  font-weight: 700;
  letter-spacing: -0.5px;
}

.app-subtitle {
  margin: 0;
  font-size: 13px;
  color: var(--body-muted);
}

.error-banner {
  padding: 10px 14px;
  background-color: var(--color-danger-soft);
  border: 1px solid var(--color-danger-border);
  border-radius: var(--radius-sm);
  color: var(--color-danger);
  font-size: 13px;
  text-align: center;
  font-weight: 500;
}

.interactive-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-options {
  display: flex;
  justify-content: flex-start;
}

.form-action {
  margin-top: 4px;
}

/* 动效过渡 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
