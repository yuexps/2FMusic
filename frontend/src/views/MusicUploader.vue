<template>
  <div class="flex flex-col min-h-full">
    <div class="view-header">
      <h1 class="view-title">上传音乐</h1>
    </div>

    <!-- 上传目录选择 -->
    <div class="glass-panel p-5 rounded-2xl mb-6 box-border max-md:p-4">
      <div class="flex items-center justify-between max-md:flex-col max-md:items-stretch max-md:gap-2">
        <label for="upload-folder-select" class="text-sm font-semibold text-ink">目标保存目录</label>
        <div class="select-wrapper">
          <n-dropdown trigger="click" :options="folderOptions" @select="handleFolderSelect">
            <n-button round class="min-w-55 max-md:w-full flex justify-between items-center">
              <div class="flex items-center gap-2">
                <SvgIcon name="folder" />
                <span>{{ selectedFolderLabel }}</span>
              </div>
              <SvgIcon name="chevron-down" class="text-[10px] opacity-50 ml-3" />
            </n-button>
          </n-dropdown>
        </div>
      </div>
    </div>

    <!-- 拖拽上传区域 -->
    <div
      class="drag-drop-zone border-2 border-dashed border-border-input rounded-2xl h-60 flex items-center justify-center cursor-pointer bg-action-btn backdrop-blur-card text-center p-6 box-border mb-8 max-md:h-40 max-md:p-4"
      :class="{ dragging: isDragging }" @dragover.prevent="isDragging = true" @dragleave.prevent="isDragging = false"
      @drop.prevent="handleFileDrop" @click="triggerFileSelect">
      <input ref="fileInput" type="file" multiple accept=".mp3,.flac,.wav,.ogg,.m4a,.aac" class="hidden"
        @change="handleFileSelect" />

      <div class="flex flex-col items-center gap-3">
        <SvgIcon name="cloud-upload" class="text-[44px] text-primary opacity-80 max-md:text-[32px]" />
        <h3 class="m-0 text-base font-semibold text-ink max-md:text-sm">拖拽音乐文件到此区域</h3>
        <p class="m-0 text-xs text-body-muted max-md:text-[11px]">或点击此处从本地选取文件。支持格式：MP3, FLAC, WAV, M4A, OGG, AAC</p>
      </div>
    </div>

    <!-- 上传列表及进度展示 -->
    <div v-if="uploadList.length > 0" class="flex flex-col gap-3">
      <div class="flex justify-between items-center border-b border-border-main pb-2 mb-2">
        <h2 class="m-0 text-sm font-semibold text-ink">上传进度 ({{ finishedCount }} / {{ uploadList.length }})</h2>
        <n-button round text @click="clearFinished" :disabled="uploadingCount > 0" class="max-md:w-8 max-md:h-8 max-md:flex max-md:items-center max-md:justify-center [&_.n-button__icon]:max-md:mr-0!">
          <template #icon>
            <SvgIcon name="trash" class="md:hidden!" />
          </template>
          <span class="max-md:hidden!">清除已完成</span>
        </n-button>
      </div>

      <div class="flex flex-col gap-2">
        <div v-for="task in uploadList" :key="task.id"
          class="glass-card p-3 rounded-lg flex justify-between items-center gap-4 max-md:p-2.5 max-md:gap-2"
          :class="task.status">
          <div class="overflow-hidden flex flex-col gap-1 flex-1">
            <div class="text-xs font-semibold text-ink truncate" :title="task.name">{{ task.name }}</div>
            <div class="text-[11px] text-body-muted">{{ formatSize(task.size) }}</div>
          </div>

          <div class="w-37.5 flex justify-end max-md:w-25">
            <div v-if="task.status === 'uploading'" class="flex flex-col gap-1 w-full">
              <div class="text-[10px] text-primary font-medium text-right">{{ task.progress }}%</div>
              <div class="h-1 bg-border-input rounded-full overflow-hidden">
                <div class="h-full bg-primary rounded-full transition-all duration-200"
                  :style="{ width: task.progress + '%' }"></div>
              </div>
            </div>
            <div v-else-if="task.status === 'success'"
              class="text-xs font-semibold flex items-center gap-1 text-success">
              <SvgIcon name="check-circle" /> 成功
            </div>
            <div v-else-if="task.status === 'error'"
              class="text-xs font-semibold flex items-center gap-1 text-danger cursor-help" :title="task.error">
              <SvgIcon name="exclamation-circle" /> 失败
            </div>
            <div v-else class="text-xs font-semibold flex items-center gap-1 text-body-muted text-[11px]">
              等待中
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useSystemStore } from '../stores/system'
import { NDropdown, NButton, useMessage } from 'naive-ui'
import client from '../api/client'

const systemStore = useSystemStore()
const message = useMessage()

// 目录选择
const selectedFolderPath = ref('') // 空代表默认库根目录

const folderOptions = computed(() => {
  const options = [
    { label: '默认音乐库 (根目录)', key: '' }
  ]
  systemStore.mountPoints.forEach(p => {
    options.push({ label: p, key: p })
  })
  return options
})

const selectedFolderLabel = computed(() => {
  if (!selectedFolderPath.value) return '默认音乐库 (根目录)'
  return selectedFolderPath.value
})

const handleFolderSelect = (key: string) => {
  selectedFolderPath.value = key
}

// 拖拽文件与选择
const isDragging = ref(false)
const fileInput = ref<HTMLInputElement | null>(null)

interface UploadTask {
  id: string
  name: string
  size: number
  file: File
  progress: number
  status: 'pending' | 'uploading' | 'success' | 'error'
  error?: string
}

const uploadList = ref<UploadTask[]>([])

const triggerFileSelect = () => {
  fileInput.value?.click()
}

const handleFileDrop = (e: DragEvent) => {
  isDragging.value = false
  if (e.dataTransfer?.files) {
    addFilesToList(e.dataTransfer.files)
  }
}

const handleFileSelect = (e: Event) => {
  const target = e.target as HTMLInputElement
  if (target.files) {
    addFilesToList(target.files)
  }
}

const addFilesToList = (files: FileList) => {
  const allowedExts = ['.mp3', '.flac', '.wav', '.ogg', '.m4a', '.aac']

  for (let i = 0; i < files.length; i++) {
    const file = files[i]
    const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase()

    if (!allowedExts.includes(ext)) {
      message.error(`不支持文件格式: ${file.name}`)
      continue
    }

    const taskId = `${Date.now()}-${Math.random().toString(36).substring(2, 9)}`

    uploadList.value.push({
      id: taskId,
      name: file.name,
      size: file.size,
      file: file,
      progress: 0,
      status: 'pending'
    })
  }

  // 触发队列上传
  processQueue()
}

  // 上传并发数限制
  const MAX_CONCURRENT = 2

const uploadingCount = computed(() => {
  return uploadList.value.filter(t => t.status === 'uploading').length
})

const finishedCount = computed(() => {
  return uploadList.value.filter(t => t.status === 'success' || t.status === 'error').length
})

const processQueue = () => {
  if (uploadingCount.value >= MAX_CONCURRENT) return

  const nextTask = uploadList.value.find(t => t.status === 'pending')
  if (!nextTask) return

  uploadFile(nextTask)
  // 递归并发处理
  processQueue()
}

const uploadFile = async (task: UploadTask) => {
  task.status = 'uploading'

  const formData = new FormData()
  formData.append('file', task.file)
  if (selectedFolderPath.value) {
    formData.append('target_dir', selectedFolderPath.value)
  }

  try {
    const res = await client.post('/api/music/upload', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        if (progressEvent.total) {
          task.progress = Math.round((progressEvent.loaded / progressEvent.total) * 100)
        }
      }
    })

    if (res.data && res.data.success) {
      task.status = 'success'
      task.progress = 100
      // 成功上传后自动刷新本地歌曲列表
      systemStore.fetchSongs()
    } else {
      task.status = 'error'
      task.error = res.data?.error || '上传失败'
    }
  } catch (e: any) {
    task.status = 'error'
    task.error = e.response?.data?.error || '上传网络异常'
  } finally {
    // 递归触发下一个
    processQueue()
  }
}

const clearFinished = () => {
  uploadList.value = uploadList.value.filter(t => t.status === 'pending' || t.status === 'uploading')
}

const formatSize = (bytes: number) => {
  const mb = bytes / (1024 * 1024)
  return `${mb.toFixed(1)} MB`
}

onMounted(() => {
  systemStore.fetchMountPoints()
})
</script>
