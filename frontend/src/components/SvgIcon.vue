<template>
  <n-icon :size="iconSize" :depth="depth" :class="['inline-block align-[-0.15em] shrink-0', { 'animate-spin': spin }]">
    <component :is="iconComponent" />
  </n-icon>
</template>

<script setup lang="ts">
import { computed, type PropType } from 'vue'
import { NIcon } from 'naive-ui'
import { Docker, SortAmountDown, SortAmountUp } from '@vicons/fa'
import { Playlist, Repeat, RepeatOne, Shuffle } from '@vicons/carbon'
import { Previous32Filled, Next32Filled, Play48Filled, Pause48Filled, DrawerPlay24Regular } from '@vicons/fluent'
import {
  VolumeHighOutline, VolumeMuteOutline,
  Heart, HeartOutline, Add, Close,
  CheckmarkCircleOutline, AlertCircleOutline, EllipsisHorizontal,
  SearchOutline, SettingsOutline, TrashOutline,
  TimeOutline, MusicalNotesOutline, FolderOutline, FolderOpenOutline,
  DownloadOutline, CloudDownloadOutline, CloudUploadOutline,
  CloudOfflineOutline,
  SyncOutline, ChevronDown, ChevronUp, ChevronForward, ChevronBack,
  ArrowBack, SwapVerticalOutline,
  LogoGithub, ServerOutline, LogInOutline,
  ShieldCheckmarkOutline, SparklesOutline, WifiOutline, ColorPaletteOutline, MenuOutline
} from '@vicons/ionicons5'

const props = defineProps({
  name: {
    type: String,
    required: true
  },
  size: {
    type: [Number, String],
    default: '1em'
  },
  spin: {
    type: Boolean,
    default: false
  },
  depth: {
    type: Number as PropType<1 | 2 | 3 | 4 | 5>,
    default: undefined
  }
})

// 处理 size 的显示形式
const iconSize = computed(() => {
  return typeof props.size === 'number' ? `${props.size}px` : props.size
})

// 图标组件映射表
const COMPONENT_MAP: Record<string, any> = {
  'play': Play48Filled,
  'pause': Pause48Filled,
  'backward': Previous32Filled,
  'forward': Next32Filled,
  'volume-up': VolumeHighOutline,
  'volume-mute': VolumeMuteOutline,
  'redo': Repeat,
  'redo-alt': RepeatOne,
  'random': Shuffle,
  'heart': Heart,
  'heart-o': HeartOutline,
  'plus': Add,
  'close': Close,
  'times': Close,
  'check-circle': CheckmarkCircleOutline,
  'exclamation-circle': AlertCircleOutline,
  'ellipsis-h': EllipsisHorizontal,
  'search': SearchOutline,
  'spinner': SyncOutline,
  'cog': SettingsOutline,
  'trash': TrashOutline,
  'trash-alt': TrashOutline,
  'history': TimeOutline,
  'music': MusicalNotesOutline,
  'folder': FolderOutline,
  'folder-plus': FolderOpenOutline,
  'download': DownloadOutline,
  'cloud-download': CloudDownloadOutline,
  'cloud-download-alt': CloudDownloadOutline,
  'cloud-upload': CloudUploadOutline,
  'cloud-upload-alt': CloudUploadOutline,
  'cloud-off': CloudOfflineOutline,
  'settings': SettingsOutline,
  'sync': SyncOutline,
  'sync-alt': SyncOutline,
  'chevron-down': ChevronDown,
  'chevron-up': ChevronUp,
  'chevron-right': ChevronForward,
  'chevron-left': ChevronBack,
  'arrow-left': ArrowBack,
  'sort': SwapVerticalOutline,
  'sort-amount-down': SortAmountDown,
  'sort-amount-down-alt': SortAmountDown,
  'sort-amount-up': SortAmountUp,
  'docker': Docker,
  'github': LogoGithub,
  'network': ServerOutline,
  'network-wired': ServerOutline,
  'login': LogInOutline,
  'sign-in-alt': LogInOutline,
  'tasks': Playlist,
  'user-shield': ShieldCheckmarkOutline,
  'magic': SparklesOutline,
  'palette': ColorPaletteOutline,
  'wifi': WifiOutline,
  'menu': MenuOutline,
  'folia': DrawerPlay24Regular
}

const iconComponent = computed(() => {
  return COMPONENT_MAP[props.name] || null
})
</script>

<script lang="ts">
export default {
  name: 'SvgIcon'
}
</script>
