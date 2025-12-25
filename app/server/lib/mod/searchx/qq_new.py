import json
import requests
from mod import tools
from mod import textcompare

COMMON_SEARCH_URL_QQ = 'https://u.y.qq.com/cgi-bin/musicu.fcg'
LYRIC_URL_QQ = 'https://i.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid={}&g_tk=5381&format=json&inCharset=utf8&outCharset=utf-8&nobase64=1'
ALBUM_COVER_URL_QQ = 'https://y.qq.com/music/photo_new/T002R300x300M000{albummid}.jpg'

headers = {
    'user-agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/115.0',
    'origin': 'https://y.qq.com/',
    'referer': 'https://y.qq.com/',
    'accept': 'application/json, text/plain, */*',
    'content-type': 'application/json;charset=UTF-8',
}

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
    except Exception:
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
    except Exception:
        return None

def get_album_cover_image(albummid=None, vs=None, singer_mid=None, songmid=None):
    """
    获取QQ音乐歌曲所有可能的封面图片链接，按优先级返回第一个有效图片（默认），或全部可用图片。
    支持参数：
        albummid: 专辑mid
        vs: vs字段图片编号列表
        singer_mid: 歌手mid
        songmid: 歌曲mid
    """
    def check_url(url):
        try:
            resp = requests.head(url, timeout=2)
            return resp.status_code == 200
        except Exception:
            return False

    candidates = []
    # 专辑封面
    if albummid and isinstance(albummid, str) and len(albummid) >= 4:
        candidates.append(ALBUM_COVER_URL_QQ.format(albummid=albummid))
    # vs 字段图片
    if vs and isinstance(vs, list):
        for v in vs:
            if v and isinstance(v, str) and len(v) >= 4:
                candidates.append(f"https://y.qq.com/music/photo_new/T062R300x300M000{v}.jpg")

    for url in candidates:
        if check_url(url):
            return url
    return None

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

def search_track(title, artist, album, max_results=3, score_threshold=0.5):
    """
    智能筛选：多结果排序，分数高于阈值的都可返回，默认最多返回3个。
    """
    search_str = ' '.join([item for item in [title, artist, album] if item])
    songs = search_with_keyword(search_str, searchType=0, resultNum=10)
    if not songs or 'list' not in songs or not songs['list']:
        return []
    results = []
    for song_item in songs['list']:
        song_title = song_item.get('name', '')
        album_name = song_item.get('album', {}).get('title', '')
        artist_name = ' '.join([s.get('name') for s in song_item.get('singer', [])])
        title_score = textcompare.association(title, song_title)
        artist_score = textcompare.assoc_artists(artist, artist_name) if artist else 1.0
        album_score = textcompare.association(album, album_name) if album else 1.0
        # 综合分数
        score = 0.6 * title_score + 0.3 * artist_score + 0.1 * album_score
        # 阈值过滤
        if score < score_threshold:
            continue
        # 组装歌词
        songmid = song_item.get('mid')
        cover_url = get_album_cover_image(
            albummid=song_item.get('album', {}).get('mid', ''),
            vs=song_item.get('vs', []),
            singer_mid=(song_item.get('singer', [{}])[0].get('mid', '') if song_item.get('singer') else ''),
            songmid=song_item.get('mid', '')
        )
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
        music_json_data = dict(song_item)
        music_json_data.update({
            "title": song_title,
            "album": album_name,
            "artist": artist_name,
            "lyrics": lyrics,
            "cover": cover_url,
            "id": tools.calculate_md5(f"title:{song_title};artists:{artist_name};album:{album_name}", base='decstr'),
            "score": score
        })
        results.append(music_json_data)
    # 按分数排序，返回前max_results个
    results.sort(key=lambda x: x['score'], reverse=True)
    return results[:max_results]

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
    return None
