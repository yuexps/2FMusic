<template>
  <div class="flex flex-col mount-manager-view">
    <div class="view-header">
      <h1 class="view-title">音乐目录管理</h1>
    </div>

    <!-- 添加挂载点 -->
    <div class="glass-panel p-6 rounded-2xl mb-8 box-border max-md:p-4 max-md:mb-5">
      <div class="mb-5">
        <h3 class="card-title">
          <SvgIcon name="folder-plus" /> 添加服务器目录
        </h3>
        <p class="hint-text">
          请输入服务器上的绝对路径（例如: <code>/vol1/music/custom</code>）。系统将自动监控该目录下的音乐文件并完成导入。
        </p>
      </div>

      <div class="flex gap-3 items-center max-md:flex-col max-md:gap-2.5 max-md:items-stretch">
        <n-input type="text" v-model:value="newMountPath" placeholder="请输入服务器上的文件夹绝对路径..." round
          @keyup.enter="handleAddMount" />
        <n-button round type="primary" size="large" @click="handleAddMount" :disabled="!newMountPath.trim() || isAdding"
          :loading="isAdding" class="max-md:w-full">
          添加并扫描
        </n-button>
      </div>
    </div>

    <!-- 已挂载点列表 -->
    <div class="flex flex-col">
      <h2 class="section-title">已添加的监控目录</h2>

      <div v-if="systemStore.mountPoints.length === 0"
        class="flex flex-col items-center py-12 text-center text-body-muted">
        <SvgIcon name="network" class="text-[32px] mb-3 opacity-50" />
        <p>暂无外接监控目录，仅使用默认音乐库</p>
      </div>

      <div v-else class="flex flex-col gap-3">
        <div v-for="path in systemStore.mountPoints" :key="path"
          class="glass-card p-4 rounded-xl flex justify-between items-center flex-wrap gap-4 max-md:p-3 max-md:gap-3 max-md:flex-nowrap">
          <div class="flex items-center gap-3 flex-1 min-w-62.5 overflow-hidden max-md:min-w-0">
            <div class="text-warning text-[20px] shrink-0">
              <SvgIcon name="folder" />
            </div>
            <div class="font-sans text-sm text-ink truncate" :title="path">{{ path }}</div>
          </div>

          <div class="mount-actions flex items-center gap-2 shrink-0">
            <n-button round size="small" @click="handleTriggerScan(path)" title="手动触发增量扫描"
              class="max-md:w-8 max-md:h-8 max-md:p-0 max-md:rounded-full max-md:min-w-8 max-md:shrink-0 max-md:flex max-md:items-center max-md:justify-center [&_.n-button__icon]:max-md:mr-0!">
              <template #icon>
                <SvgIcon name="sync" class="max-md:m-0!" />
              </template>
              <span class="max-md:hidden">增量扫描</span>
            </n-button>

            <n-button round size="small" @click="handleTriggerRescrape(path)" title="重新刮削封面与歌词"
              class="max-md:w-8 max-md:h-8 max-md:p-0 max-md:rounded-full max-md:min-w-8 max-md:shrink-0 max-md:flex max-md:items-center max-md:justify-center [&_.n-button__icon]:max-md:mr-0!">
              <template #icon>
                <SvgIcon name="magic" class="max-md:m-0!" />
              </template>
              <span class="max-md:hidden">重新刮削</span>
            </n-button>

            <n-button round size="small" type="error" secondary @click="confirmRemoveMount(path)" title="移除该目录"
              class="max-md:w-8 max-md:h-8 max-md:p-0 max-md:rounded-full max-md:min-w-8 max-md:shrink-0 max-md:flex max-md:items-center max-md:justify-center [&_.n-button__icon]:max-md:mr-0!">
              <template #icon>
                <SvgIcon name="trash" class="max-md:m-0!" />
              </template>
              <span class="max-md:hidden">移除</span>
            </n-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 移除确认模态框 -->
    <n-modal v-model:show="showRemoveConfirm" preset="dialog" title="确认移除监控目录">
      <div>
        确定要移除监控目录 <code>{{ pathToRemove }}</code> 吗？移除目录<strong>不会</strong>物理删除磁盘上的任何歌曲，但会从当前播放器的数据库中抹除该路径下的所有歌曲及关联收藏。
      </div>
      <template #action>
        <n-space>
          <n-button round @click="showRemoveConfirm = false">取消</n-button>
          <n-button round type="error" @click="handleRemoveMount">确认移除</n-button>
        </n-space>
      </template>
    </n-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useSystemStore } from '../stores/system'
import { NModal, NButton, NInput, NSpace, useMessage } from 'naive-ui'

const systemStore = useSystemStore()
const message = useMessage()

const newMountPath = ref('')
const isAdding = ref(false)

const showRemoveConfirm = ref(false)
const pathToRemove = ref('')

const handleAddMount = async () => {
  const path = newMountPath.value.trim()
  if (!path) return

  isAdding.value = true
  const res = await systemStore.addMountPoint(path)
  isAdding.value = false

  if (res.success) {
    message.success(res.message || '目录已成功添加，正在后台开启监控与扫描')
    newMountPath.value = ''
  } else {
    message.error(res.error)
  }
}

const handleTriggerScan = async (path: string) => {
  const res = await systemStore.triggerScan(path)
  if (res.success) {
    message.success(res.message || '增量扫描任务已触发，请查看侧边栏进度')
  } else {
    message.error(res.error)
  }
}

const handleTriggerRescrape = async (path: string) => {
  const res = await systemStore.triggerRescrape(path)
  if (res.success) {
    message.success(res.message || '元数据刮削已触发，请查看侧边栏进度')
  } else {
    message.error(res.error)
  }
}

const confirmRemoveMount = (path: string) => {
  pathToRemove.value = path
  showRemoveConfirm.value = true
}

const handleRemoveMount = async () => {
  if (!pathToRemove.value) return
  const path = pathToRemove.value

  const res = await systemStore.removeMountPoint(path)
  showRemoveConfirm.value = false
  if (res.success) {
    message.success('已移去监控目录并清理数据库记录')
  } else {
    message.error(res.error)
  }
}

onMounted(() => {
  systemStore.fetchMountPoints()
})
</script>