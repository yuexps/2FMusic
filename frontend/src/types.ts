// 强类型定义

export interface Song {
  id: string
  filename: string
  title: string
  artist: string
  album: string
  album_art: string | null
  mtime: number
  size: number
  playCount?: number
  [key: string]: any
}

export interface FavoritePlaylist {
  id: string
  name: string
  is_default: number
  created_at: number
  song_count?: number
}

export interface MountPoint {
  path: string
  created_at?: number
}

export interface NeteaseSong {
  id: number | string
  title: string
  artist: string
  album: string
  cover: string
  duration: number
  is_vip: boolean
  level: string
  max_level: string
  size: number | null
}

export interface DownloadTask {
  task_id: string
  status: 'pending' | 'preparing' | 'downloading' | 'success' | 'error'
  progress: number
  title: string
  artist: string
  message?: string
  filename?: string
}

export interface SystemStatus {
  scanning: boolean
  total: number
  processed: number
  current_file: string
  current_path?: string
  library_version: number
  music_count: number
  playlist_count: number
  is_scraping?: boolean
  failed?: number
}
