import hashlib
import os
import sys
if getattr(sys, 'frozen', False):
    # 【打包模式】基准目录是二进制文件所在位置
    BASE_DIR = os.path.dirname(sys.executable)
else:
    # 【源码模式】基准目录是脚本所在位置
    BASE_DIR = os.path.dirname(os.path.abspath(__file__))
    # 仅在源码模式下加载 lib
    sys.path.insert(0, os.path.join(BASE_DIR, 'lib'))

import requests

QQ_SEARCH_URL = 'https://c.y.qq.com/soso/fcgi-bin/music_search_new_platform'
QQ_LYRIC_URL = 'https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg'
QQ_ALBUM_IMG_URL = 'http://imgcache.qq.com/music/photo/album_300/{}/300_albumpic_{}_0.jpg'

HEADERS = {
    'User-Agent': 'Mozilla/5.0',
    'Referer': 'https://y.qq.com/'
}

def parse_f_field(f_str):
    fields = f_str.split('|')
    return {
        'song_id': fields[0] if len(fields) > 0 else '',
        'song_name': fields[1] if len(fields) > 1 else '',
        'singer_name': fields[3] if len(fields) > 3 else '',
        'img_id': fields[4] if len(fields) > 4 else '',
        'album': fields[5] if len(fields) > 5 else '',
        'songmid': fields[20] if len(fields) > 20 else '',
    }

def get_cover_url(img_id):
    try:
        img_id = int(img_id)
        return QQ_ALBUM_IMG_URL.format(img_id % 100, img_id)
    except Exception:
        return None

def get_lyrics(songmid):
    params = {
        'songmid': songmid,
        'format': 'json',
        'nobase64': 1,
    }
    resp = requests.get(QQ_LYRIC_URL, params=params, headers=HEADERS, timeout=10)
    data = resp.json()
    if data.get('code') == 0 and 'lyric' in data:
        return data['lyric']
    return None

def search_track(keyword, page=1, num=10):
    params = {
        'searchid': '53806572956004615',
        't': 1,
        'aggr': 1,
        'cr': 1,
        'catZhida': 1,
        'lossless': 0,
        'flag_qc': 0,
        'p': page,
        'n': num,
        'w': keyword,
        'format': 'json'
    }
    resp = requests.get(QQ_SEARCH_URL, params=params, headers=HEADERS, timeout=10)
    data = resp.json()
    song_list = []
    try:
        for item in data['data']['song']['list']:
            f = item.get('f', '')
            info = parse_f_field(f)
            lyrics = get_lyrics(info['songmid']) if info['songmid'] else None
            cover = get_cover_url(info['img_id']) if info['img_id'] else None
            music_json_data = {
                'title': info['song_name'],
                'artist': info['singer_name'],
                'album': info['album'],
                'lyrics': lyrics,
                'cover': cover,
                'id': hashlib.md5(f"title:{info['song_name']};artists:{info['singer_name']};album:{info['album']}".encode('utf-8')).hexdigest()
            }
            song_list.append(music_json_data)
    except Exception as e:
        print('解析失败:', e)
    return song_list

def search(title='', artist='', album='', page=1, num=10):
    keyword = ' '.join([str(x) for x in [title, artist] if x])
    if not keyword:
        return []
    return search_track(keyword, page=page, num=num)

if __name__ == "__main__":
    result = search(title="可能", num=2)
    for song in result:
        print(song)
