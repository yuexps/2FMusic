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

// 从 window 读取 Flask 注入的模板变量
const loginError = (window as any).loginError || ''
const actionUrl = (window as any).actionUrl || '/login'

const password = ref('')
const rememberChecked = ref(false)
const hashedPassword = ref('')
const loading = ref(false)
const showError = ref(!!loginError)

const formRef = ref<HTMLFormElement | null>(null)

// Naive UI 主题
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
    <div
      class="flex items-center justify-center min-h-screen w-screen bg-[radial-gradient(circle_at_50%_25%,rgba(0,102,204,0.08),transparent_45%)] bg-bg-app-layout transition-colors duration-normal">
      <div
        class="w-[min(390px,90vw)] p-[42px_36px] rounded-xl bg-surface-glass backdrop-saturate-180 backdrop-blur-sidebar border border-border-main box-border text-ink flex flex-col gap-6 product-cover-shadow">
        <header class="flex flex-col items-center text-center gap-2">
          <div class="flex items-center justify-center mb-2">
            <img :src="getApiUrl('/ICON.PNG')"
              class="rounded-[14px] object-cover drop-shadow-[0_4px_12px_rgba(0,102,204,0.25)]" width="60" height="60"
              alt="Logo" />
          </div>
          <h1 class="m-0 font-display text-[26px] font-bold tracking-[-0.5px]">2FMusic</h1>
          <p class="m-0 text-[13px] text-body-muted">请输入您的访问密码以进入音乐库</p>
        </header>

        <!-- 错误提示 -->
        <transition enter-active-class="transition-opacity duration-200 ease" enter-from-class="opacity-0"
          leave-active-class="transition-opacity duration-200 ease" leave-to-class="opacity-0">
          <div v-if="showError"
            class="p-[10px_14px] bg-danger-soft border border-danger-border rounded-sm text-danger text-[13px] text-center font-medium">
            <span>{{ loginError }}</span>
          </div>
        </transition>

        <!-- Vue 表单（拦截 submit 执行哈希） -->
        <form class="flex flex-col gap-5" @submit="handleLogin">
          <div class="form-group">
            <n-input v-model:value="password" type="password" show-password-on="click" placeholder="访问密码" size="large"
              autofocus />
          </div>

          <div class="flex justify-start">
            <n-checkbox v-model:checked="rememberChecked">
              保持登录 (30天内免登)
            </n-checkbox>
          </div>

          <div class="mt-1">
            <n-button type="primary" size="large" block attr-type="submit" :loading="loading">
              登录
            </n-button>
          </div>
        </form>

        <!-- 后端接收的隐藏表单 -->
        <form ref="formRef" method="POST" :action="actionUrl" style="display: none;">
          <input type="hidden" name="password" :value="hashedPassword">
          <input type="hidden" name="remember" :value="rememberChecked ? 'on' : ''">
        </form>
      </div>
    </div>
  </n-config-provider>
</template>
