import os
import requests
from mutagen import File
from mutagen.easyid3 import EasyID3
from mutagen.id3 import ID3, APIC, USLT
from mutagen.flac import FLAC, Picture
from mutagen.mp4 import MP4, MP4Cover
from core.config import app_config
from core.utils.logger import logger
from core.utils.common import COMMON_HEADERS

def get_metadata(file_path: str) -> dict:
    """提取音频文件元数据 (标题, 艺术家, 专辑)"""
    metadata = {'title': None, 'artist': None, 'album': None}
    try:
        audio = None
        try:
            audio = EasyID3(file_path)
        except Exception:
            try:
                audio = File(file_path, easy=True)
            except Exception as e2:
                audio = File(file_path)
                logger.warning(f"文件 {file_path} 元数据解析异常: {e2}")
        if audio:
            def get_tag(key):
                val = None
                # 方法 1: 直接 get (.get)
                if hasattr(audio, 'get'):
                    val = audio.get(key)
                # 方法 2: 通过 tags
                elif hasattr(audio, 'tags') and audio.tags:
                    val = audio.tags.get(key)
                    if not val:
                        val = audio.tags.get(key.upper())
                
                if val:
                    if isinstance(val, list):
                        val = val[0]
                    if val is not None and not isinstance(val, str):
                        val = str(val)
                    return val
                return None
                
            metadata['title'] = get_tag('title')
            metadata['artist'] = get_tag('artist')
            metadata['album'] = get_tag('album')
    except Exception as e:
        logger.error(f"提取元数据失败: {file_path}, 错误: {e}")
        
    filename = os.path.splitext(os.path.basename(file_path))[0]
    if not metadata['title']:
        if ' - ' in filename:
            parts = filename.split(' - ', 1)
            if not metadata['artist']: 
                metadata['artist'] = parts[0].strip()
            metadata['title'] = parts[1].strip()
        else:
            metadata['title'] = filename
            
    if not metadata['artist']: 
        metadata['artist'] = "未知艺术家"
        
    logger.debug(f"文件 {file_path} 元数据: {metadata}")
    return metadata

def extract_embedded_cover(file_path: str, song_id: str = None) -> bool:
    """从音频文件中提取内嵌的封面并保存为 covers/<song_id>.webp"""
    try:
        if not os.path.exists(file_path):
            return False
        if not song_id:
            from core.utils.hasher import generate_song_id
            song_id = generate_song_id(file_path)
        cover_dir = app_config.COVERS_DIR
        os.makedirs(cover_dir, exist_ok=True)
        target_path = os.path.join(cover_dir, f"{song_id}.webp")
        if os.path.exists(target_path):
            return True

        audio = File(file_path)
        if not audio:
            return False

        data = None

        # MP3 / ID3
        if hasattr(audio, 'tags') and audio.tags:
            if hasattr(audio.tags, 'getall'):
                for tag in audio.tags.getall('APIC'):
                    if getattr(tag, 'data', None):
                        data = tag.data
                        break
            if not data:
                covr = audio.tags.get('covr')
                if covr:
                    val = covr[0] if isinstance(covr, (list, tuple)) else covr
                    try:
                        data = bytes(val)
                    except Exception:
                        pass

        # FLAC / OGG
        if not data and hasattr(audio, 'pictures'):
            pics = getattr(audio, 'pictures') or []
            if pics:
                data = pics[0].data

        if not data:
            logger.info(f"未找到内嵌封面: {file_path}")
            return False

        if save_cover_file(data, song_id):
            logger.info(f"内嵌封面提取并保存为 WebP: {target_path}")
            return True
        return False
    except Exception as e:
        logger.warning(f"提取内嵌封面失败: {file_path}, 错误: {repr(e)}")
        return False

def extract_embedded_lyrics(file_path: str) -> str:
    """提取音频文件内嵌歌词，返回歌词内容或 None"""
    try:
        if not os.path.exists(file_path):
            return None
        
        audio = File(file_path)
        if not audio:
            return None

        # 1. MP3 / ID3 (USLT)
        if hasattr(audio, 'tags') and isinstance(audio.tags, ID3):
            for key in audio.tags.keys():
                if key.startswith('USLT'):
                    return audio.tags[key].text
        
        # 2. FLAC / Vorbis Comments
        if hasattr(audio, 'tags'):
            lyrics = audio.tags.get('lyrics') or audio.tags.get('LYRICS') or audio.tags.get('unsyncedlyrics') or audio.tags.get('UNSYNCEDLYRICS')
            if lyrics:
                return lyrics[0]
                
        # 3. M4A / MP4
        if hasattr(audio, 'tags') and '©lyr' in audio.tags:
             return audio.tags['©lyr'][0]

    except Exception as e:
        logger.warning(f"提取内嵌歌词失败: {file_path}, 错误: {repr(e)}")
    return None

def fetch_cover_bytes(url: str) -> bytes:
    """下载网络封面图片字节数据"""
    if not url:
        return None
    try:
        resp = requests.get(url, timeout=8, headers=COMMON_HEADERS)
        if resp.status_code == 200 and resp.content:
            return resp.content
    except Exception as e:
        logger.warning(f"封面下载失败: {url}, 错误: {e}")
    return None

def embed_cover_to_file(audio_path: str, cover_bytes: bytes):
    """将封面字节嵌入音频文件 (支持 mp3/flac/m4a)"""
    if not cover_bytes or not os.path.exists(audio_path):
        return
    ext = os.path.splitext(audio_path)[1].lower()
    try:
        if ext == '.mp3':
            audio = None
            try:
                audio = ID3(audio_path)
            except Exception:
                audio = File(audio_path)
                audio.add_tags()
                audio.save()
                audio = ID3(audio_path)
            if audio:
                audio.delall('APIC')
                audio.add(APIC(mime='image/jpeg', type=3, desc='Cover', data=cover_bytes))
                audio.save()
        elif ext == '.flac':
            audio = FLAC(audio_path)
            pic = Picture()
            pic.data = cover_bytes
            pic.type = 3
            pic.mime = 'image/jpeg'
            audio.clear_pictures()
            audio.add_picture(pic)
            audio.save()
        elif ext in ('.m4a', '.m4b', '.m4p', '.mp4'):
            audio = MP4(audio_path)
            fmt = MP4Cover.FORMAT_JPEG
            if cover_bytes.startswith(b'\x89PNG'):
                fmt = MP4Cover.FORMAT_PNG
            audio['covr'] = [MP4Cover(cover_bytes, fmt)]
            audio.save()
    except Exception as e:
        logger.warning(f"内嵌封面失败: {audio_path}, 错误: {e}")

def save_cover_file(cover_bytes: bytes, song_id: str) -> str:
    """将封面字节写入 covers/ 缓存目录，以 song_id.webp 命名"""
    if not cover_bytes or not song_id:
        return None
    try:
        cover_dir = app_config.COVERS_DIR
        os.makedirs(cover_dir, exist_ok=True)
        cover_path = os.path.join(cover_dir, f"{song_id}.webp")
        
        from core.utils.image import compress_and_convert_to_webp
        processed_bytes = compress_and_convert_to_webp(cover_bytes)
        
        with open(cover_path, 'wb') as f:
            f.write(processed_bytes)
        return cover_path
    except Exception as e:
        logger.warning(f"封面保存失败: {song_id}, 错误: {e}")
        return None

def embed_lyrics_to_file(audio_path: str, lrc_text: str):
    """将歌词文本嵌入音频文件 (USLT 或 LYRICS)"""
    if not lrc_text or not os.path.exists(audio_path):
        return
    ext = os.path.splitext(audio_path)[1].lower()
    try:
        if ext == '.mp3':
            try:
                tags = ID3(audio_path)
            except Exception:
                tags = File(audio_path)
                tags.add_tags()
                tags.save()
                tags = ID3(audio_path)
            tags.delall('USLT')
            tags.add(USLT(encoding=3, lang='chi', desc='Lyric', text=lrc_text))
            tags.save()
        elif ext == '.flac':
            audio = FLAC(audio_path)
            audio['LYRICS'] = lrc_text
            audio.save()
        elif ext in ('.m4a', '.m4b', '.m4p', '.mp4'):
            audio = MP4(audio_path)
            audio['\xa9lyr'] = lrc_text
            audio.save()
        elif ext in ('.ogg', '.oga'):
            audio = File(audio_path)
            audio['LYRICS'] = lrc_text
            audio.save()
    except Exception as e:
        logger.warning(f"内嵌歌词失败: {audio_path}, 错误: {e}")
