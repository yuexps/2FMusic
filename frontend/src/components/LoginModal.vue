<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { getApiUrl } from '../utils/path'
import client from '../api/client'
import {
  NConfigProvider,
  NInput,
  NButton,
  NCheckbox,
  darkTheme,
  useOsTheme
} from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import { usePreferencesStore } from '../stores/preferences'

const props = defineProps<{
  show: boolean
}>()

const emit = defineEmits<{
  (e: 'success'): void
}>()

const preferencesStore = usePreferencesStore()
const osThemeRef = useOsTheme()

const isDark = computed(() => {
  if (preferencesStore.themeMode === 'system') {
    return osThemeRef.value === 'dark'
  }
  return preferencesStore.themeMode === 'dark'
})

const theme = computed(() => isDark.value ? darkTheme : null)

const password = ref('')
const rememberChecked = ref(false)
const loading = ref(false)
const showError = ref(false)
const errorMessage = ref('')

// Naive UI 主题
const themeOverrides = computed<GlobalThemeOverrides>(() => ({
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
}))

// 校验当前浏览器是否支持 Web Crypto SHA-256
function isCryptoSupported(): boolean {
  return typeof window !== 'undefined' && !!(window.crypto && window.crypto.subtle && typeof window.crypto.subtle.digest === 'function')
}

// 异步计算 SHA-256 密码哈希
async function sha256(message: string): Promise<string> {
  if (!isCryptoSupported()) {
    throw new Error('当前浏览器不支持 SHA-256 ，请升级或更换现代浏览器！')
  }
  const msgBuffer = new TextEncoder().encode(message)
  const hashBuffer = await crypto.subtle.digest('SHA-256', msgBuffer)
  const hashArray = Array.from(new Uint8Array(hashBuffer))
  return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
}

const handleLogin = async (e: Event) => {
  e.preventDefault()
  if (!isCryptoSupported()) {
    showError.value = true
    errorMessage.value = '当前浏览器不支持 SHA-256 ，请升级或更换现代浏览器！'
    return
  }
  if (!password.value) return
  loading.value = true
  showError.value = false
  errorMessage.value = ''

  try {
    const hashedPassword = await sha256(password.value)
    const res = await client.post('/api/login', { password: hashedPassword })

    if (res.data && res.data.success) {
      localStorage.setItem('2fmusic_password', hashedPassword)
      emit('success')
    } else {
      showError.value = true
      errorMessage.value = res.data?.error || '密码错误'
    }
  } catch (err: any) {
    showError.value = true
    errorMessage.value = err?.message || err?.response?.data?.error || err?.response?.data?.message || '密码不正确'
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (newVal) => {
  if (newVal) {
    password.value = ''
    if (!isCryptoSupported()) {
      showError.value = true
      errorMessage.value = '当前浏览器不支持 SHA-256 ，请升级或更换现代浏览器！'
    } else {
      showError.value = false
      errorMessage.value = ''
    }
  }
})
</script>

<template>
  <Transition name="fade">
    <div v-if="show"
      class="fixed inset-0 z-99999 flex items-center justify-center p-4 bg-black/45 select-none box-border">
      <n-config-provider :theme="theme" :theme-overrides="themeOverrides">
        <div
          class="w-[min(390px,90vw)] p-[42px_36px] rounded-xl bg-surface-glass backdrop-saturate-180 backdrop-blur-sidebar border border-border-main box-border text-ink flex flex-col gap-6 product-cover-shadow">
          <header class="flex flex-col items-center text-center gap-2">
            <div class="flex items-center justify-center mb-2">
              <img :src="getApiUrl('/ICON.PNG')"
                class="rounded-[14px] object-cover drop-shadow-[0_4px_12px_rgba(0,102,204,0.25)]" width="60" height="60"
                alt="Logo" />
            </div>
            <h1 class="m-0 font-display text-[26px] font-bold tracking-[-0.5px]">2FMusic</h1>
            <p class="m-0 text-[13px] text-body-muted">请输入访问密码以解锁音乐库</p>
          </header>

          <form class="flex flex-col gap-5" @submit="handleLogin">
            <div class="form-group">
              <n-input v-model:value="password" type="password" show-password-on="click" placeholder="请输入密码"
                size="large" autofocus />
            </div>

            <div class="flex justify-start">
              <n-checkbox v-model:checked="rememberChecked">
                保持登录
              </n-checkbox>
            </div>

            <div class="mt-1">
              <n-button type="primary" size="large" block attr-type="submit" :loading="loading">
                登录
              </n-button>
            </div>

            <transition enter-active-class="transition-opacity duration-200 ease" enter-from-class="opacity-0"
              leave-active-class="transition-opacity duration-200 ease" leave-to-class="opacity-0">
              <div v-if="showError" class="text-danger text-[13px] text-center font-medium">
                <span>{{ errorMessage }}</span>
              </div>
            </transition>
          </form>
        </div>
      </n-config-provider>
    </div>
  </Transition>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
