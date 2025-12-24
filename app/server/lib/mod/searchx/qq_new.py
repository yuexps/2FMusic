import json
import logging
import urllib.parse
from functools import lru_cache

import requests

from mod import textcompare, tools
from mygo.devtools import no_error
from mod.ttscn import t2s

headers = {
    'user-agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.0',
    'origin': 'https://y.qq.com/',
    'referer': 'https://y.qq.com/',
    'accept': 'application/json, text/plain, */*',
    'content-type': 'application/json;charset=UTF-8',
}

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

# QQ音乐API
COMMON_SEARCH_URL_QQ = 'https://u.y.qq.com/cgi-bin/musicu.fcg'
SONG_URL_QQ = 'https://u.y.qq.com/cgi-bin/musicu.fcg'
SONGLIST_URL_QQ = 'https://i.y.qq.com/qzone-music/fcg-bin/fcg_ucc_getcdinfo_byids_cp.fcg?type=1&json=1&utf8=1&onlysong=0&nosign=1&disstid={}&g_tk=5381&loginUin=0&hostUin=0&format=json&inCharset=GB2312&outCharset=utf-8&notice=0&platform=yqq&needNewCode=0'
LYRIC_URL_QQ = 'https://i.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid={}&g_tk=5381&format=json&inCharset=utf8&outCharset=utf-8&nobase64=1'
ALBUM_URL_QQ = 'https://i.y.qq.com/v8/fcg-bin/fcg_v8_album_info_cp.fcg?platform=h5page&albummid={}&g_tk=938407465&uin=0&format=json&inCharset=utf-8&outCharset=utf-8&notice=0&platform=h5&needNewCode=1&_=1459961045571'
SINGER_URL_QQ = 'https://u.y.qq.com/cgi-bin/musicu.fcg?format=json&loginUin=0&hostUin=0inCharset=utf8&outCharset=utf-8&platform=yqq.json&needNewCode=0&data=%7B%22comm%22%3A%7B%22ct%22%3A24%2C%22cv%22%3A0%7D%2C%22singer%22%3A%7B%22method%22%3A%22get_singer_detail_info%22%2C%22param%22%3A%7B%22sort%22%3A5%2C%22singermid%22%3A%22{}%22%2C%22sin%22%3A0%2C%22num%22%3A50%7D%2C%22module%22%3A%22music.web_singer_info_svr%22%7D%7D'


def get_music_url(songmid, quality="320", origin=False):
    quality_map = {
        "m4a": ("C400", "m4a"),
        "128": ("M500", "mp3"),
        "320": ("M800", "mp3"),
    }
    prefix, suffix = quality_map.get(quality.lower(), ("M800", "mp3"))
    filename = f"{prefix}{songmid}.{suffix}"
    data = {
        "req_1": {
            "module": "vkey.GetVkeyServer",
            "method": "CgiGetVkey",
            "param": {
                "filename": [filename],
                "guid": "10000",
                "songmid": [songmid],
                "songtype": [0],
                "uin": "0",
                "loginflag": 1,
                "platform": "20"
            }
        },
        "loginUin": "0",
        "comm": {"uin": "0", "format": "json", "ct": 24, "cv": 0}
    }
    resp = requests.post(SONG_URL_QQ, headers=headers, data=json.dumps(data))
    data = resp.json()
    if origin:
        return data
    try:
        return data['req_1']['data']['sip'][0] + data['req_1']['data']['midurlinfo'][0]['purl']
    except Exception as e:
        logger.error(f"get_music_url error: {e}")
        return None


def get_songlist(categoryID, origin=False):
    url = SONGLIST_URL_QQ.format(categoryID)
    resp = requests.get(url, headers=headers)
    data = resp.json()
    if origin:
        return data
    try:
        return data['cdlist'][0]['songlist']
    except Exception as e:
        logger.error(f"get_songlist error: {e}")
        return None


def get_songlist_name(categoryID, origin=False):
    data = get_songlist(categoryID, True)
    if origin:
        return data
    try:
        return data['cdlist'][0]['dissname']
    except Exception as e:
        logger.error(f"get_songlist_name error: {e}")
        return None


def search_with_keyword(keyword, searchType=0, resultNum=50, pageNum=1, origin=False):
    data = {
        "comm": {"ct": "19", "cv": "1859", "uin": "0"},
        "req": {
            "method": "DoSearchForQQMusicDesktop",
            "module": "music.search.SearchCgiService",
            "param": {
                "grp": 1,
                "num_per_page": resultNum,
                "page_num": pageNum,
                "query": keyword,
                "search_type": searchType
            }
        }
    }
    resp = requests.post(COMMON_SEARCH_URL_QQ, headers=headers, data=json.dumps(data))
    data = resp.json()
    if origin:
        return data
    try:
        body = data['req']['data']['body']
        if searchType == 0 or searchType == 7:
            return body['song']
        elif searchType == 2:
            return body['album']
        elif searchType == 3:
            return body['songlist']
        elif searchType == 4:
            return body['mv']
        elif searchType == 8:
            return body['user']
        else:
            return body
    except Exception as e:
        logger.error(f"search_with_keyword error: {e}")
        return None


def get_song_lyric(songmid, parse=False, origin=False):
    url = LYRIC_URL_QQ.format(songmid)
    resp = requests.get(url, headers=headers)
    data = resp.json()
    if origin:
        return data
    try:
        if not parse:
            return data.get('lyric', '') + "\n" + data.get('trans', '')
        else:
            return parse_lyric(data)
    except Exception as e:
        logger.error(f"get_song_lyric error: {e}")
        return None


def get_album_songlist(albummid, origin=False):
    url = ALBUM_URL_QQ.format(albummid)
    resp = requests.get(url, headers=headers)
    data = resp.json()
    if origin:
        return data
    try:
        return data['data']['list']
    except Exception as e:
        logger.error(f"get_album_songlist error: {e}")
        return None


def get_album_name(albummid, origin=False):
    data = get_album_songlist(albummid, True)
    if origin:
        return data
    try:
        return data['data']['name']
    except Exception as e:
        logger.error(f"get_album_name error: {e}")
        return None


def get_mv_info(vid, origin=True):
    data = {
        "comm": {"ct": 6, "cv": 0, "g_tk": 1646675364, "uin": 0, "format": "json", "platform": "yqq"},
        "mvInfo": {
            "module": "music.video.VideoData",
            "method": "get_video_info_batch",
            "param": {
                "vidlist": [vid],
                "required": ["vid", "type", "sid", "cover_pic", "duration", "singers", "new_switch_str", "video_pay", "hint", "code", "msg", "name", "desc", "playcnt", "pubdate", "isfav", "fileid", "filesize_v2", "switch_pay_type", "pay", "pay_info", "uploader_headurl", "uploader_nick", "uploader_uin", "uploader_encuin", "play_forbid_reason"]
            }
        },
        "mvUrl": {
            "module": "music.stream.MvUrlProxy",
            "method": "GetMvUrls",
            "param": {"vids": [vid], "request_type": 10003, "addrtype": 3, "format": 264, "maxFiletype": 60}
        }
    }
    resp = requests.post(COMMON_SEARCH_URL_QQ, headers=headers, data=json.dumps(data))
    return resp.json()


def get_singer_info(singermid, origin=False):
    url = SINGER_URL_QQ.format(singermid)
    resp = requests.get(url, headers=headers)
    data = resp.json()
    if origin:
        return data
    try:
        return data['singer']['data']
    except Exception as e:
        logger.error(f"get_singer_info error: {e}")
        return None


def get_album_cover_image(albummid):
    if not albummid or not isinstance(albummid, str) or len(albummid) < 4:
        return None
    return f"https://y.qq.com/music/photo_new/T002R300x300M000{albummid}.jpg"


def parse_lyric(data):
    parsed = {
        "ti": "",
        "ar": "",
        "al": "",
        "by": "",
        "offset": "",
        "count": 0,
        "haveTrans": False,
        "lyric": [],
    }
    lyric = data.get('lyric', '').split("\n")
    trans = data.get('trans', '').split("\n")
    parsed['haveTrans'] = bool(trans and trans[0])

    def substr(str_):
        return str_[str_.find(":") + 1:str_.find("]")]
    if lyric and not lyric[0].startswith("[0"):
        parsed['ti'] = substr(lyric[0])
        parsed['ar'] = substr(lyric[1])
        parsed['al'] = substr(lyric[2])
        parsed['by'] = substr(lyric[3])
        parsed['offset'] = substr(lyric[4])
        lyric = lyric[5:]
        if parsed['haveTrans']:
            trans = trans[5:]
    parsed['count'] = len(lyric)
    for i in range(parsed['count']):
        ele = {"time": "", "lyric": "", "trans": ""}
        if lyric[i]:
            ele['time'] = lyric[i][1:lyric[i].find("]")]
            ele['lyric'] = lyric[i][lyric[i].find("]") + 1:]
            if parsed['haveTrans'] and i < len(trans):
                ele['trans'] = trans[i][trans[i].find("]") + 1:]
        parsed['lyric'].append(ele)
    return parsed

def search_track(title, artist, album):
    # 只返回第一首歌曲，且保留API所有字段
    search_str = ' '.join([item for item in [title, artist, album] if item])
    songs = search_with_keyword(search_str, searchType=0, resultNum=1)
    if not songs or 'list' not in songs or not songs['list']:
        return []
    song_item = songs['list'][0]
    songmid = song_item.get('mid')
    song_title = song_item.get('name')
    album_name = song_item.get('album', {}).get('title', '')
    artist_name = ' '.join([s.get('name') for s in song_item.get('singer', [])])
    cover_url = get_album_cover_image(song_item.get('album', {}).get('mid', ''))
    # 获取完整歌词和翻译，拼接为lrc规范的双语歌词格式
    lyric_data = get_song_lyric(songmid, parse=True) if songmid else ''
    if isinstance(lyric_data, dict):
        lines = []
        for item in lyric_data.get('lyric', []):
            time = item.get('time', '')
            lyric_line = item.get('lyric', '')
            trans_line = item.get('trans', '')
            if time and lyric_line:
                lines.append(f"[{time}]{lyric_line}")
                if trans_line:
                    lines.append(f"[{time}]{trans_line}")
        lyrics = '\n'.join(lines)
    else:
        lyrics = lyric_data or ''
    # 保留API所有字段
    music_json_data = dict(song_item)
    music_json_data.update({
        "title": song_title,
        "album": album_name,
        "artist": artist_name,
        "lyrics": lyrics,
        "cover": cover_url,
        "id": tools.calculate_md5(f"title:{song_title};artists:{artist_name};album:{album_name}", base='decstr')
    })
    return [music_json_data]

def search_artist(artist):
    # 用关键词搜索歌手，取第一个歌手的mid，再查头像
    result = search_with_keyword(artist, searchType=8, resultNum=1)
    if result and 'list' in result and result['list']:
        singer = result['list'][0]
        singermid = singer.get('mid')
        info = get_singer_info(singermid) if singermid else None
        cover = info.get('basic', {}).get('avatar') if info else None
        return [{
            'cover': cover
        }]
    return None

def search_album(artist, album):
    # 用关键词搜索专辑，取第一个专辑的mid，再查封面
    keyword = f"{artist} {album}".strip()
    result = search_with_keyword(keyword, searchType=2, resultNum=1)
    if result and 'list' in result and result['list']:
        album_item = result['list'][0]
        albummid = album_item.get('mid')
        cover = get_album_cover_image(albummid) if albummid else None
        return [{
            'cover': cover
        }]
    return None

def search(title='', artist='', album=''):
    title = str(title) if title else ''
    artist = str(artist) if artist else ''
    album = str(album) if album else ''
    if not any((title, artist, album)):
        return None
    title = title.strip()
    artist = artist.strip()
    album = album.strip()
    if title:
        return search_track(title=title, artist=artist, album=album)
    elif artist and album:
        return search_album(artist, album)
    elif artist:
        return search_artist(artist)
    return None
