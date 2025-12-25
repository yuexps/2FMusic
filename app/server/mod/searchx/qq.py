import json
import time
import asyncio
from mod import tools
from mod import textcompare
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
    
import aiohttp

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

async def async_search_with_keyword(keyword, searchType=0, resultNum=50, pageNum=1, origin=False):
    t_start = time.perf_counter()
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
    t_req_start = time.perf_counter()
    connector = aiohttp.TCPConnector(ssl=False)
    async with aiohttp.ClientSession(headers=headers, connector=connector) as session:
        async with session.post(COMMON_SEARCH_URL_QQ, data=json.dumps(data, ensure_ascii=False).encode('utf-8')) as resp:
            resp_data = await resp.json(content_type=None)
    t_req_end = time.perf_counter()
    t_parse_end = time.perf_counter()
    print(f"[qq] search_with_keyword: 网络请求耗时: {(t_req_end-t_req_start)*1000:.2f} ms, 解析耗时: {(t_parse_end-t_req_end)*1000:.2f} ms, 总耗时: {(t_parse_end-t_start)*1000:.2f} ms")
    if origin:
        return resp_data
    try:
        body = resp_data['req']['data']['body']
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

def search_with_keyword(*args, **kwargs):
    return asyncio.run(async_search_with_keyword(*args, **kwargs))

async def async_get_song_lyric(songmid, parse=False, origin=False):
    t_start = time.perf_counter()
    url = LYRIC_URL_QQ.format(songmid)
    t_req_start = time.perf_counter()
    connector = aiohttp.TCPConnector(ssl=False)
    async with aiohttp.ClientSession(headers=headers, connector=connector) as session:
        async with session.get(url) as resp:
            data = await resp.json(content_type=None)
    t_req_end = time.perf_counter()
    t_parse_end = time.perf_counter()
    print(f"[qq] get_song_lyric: 网络请求耗时: {(t_req_end-t_req_start)*1000:.2f} ms, 解析耗时: {(t_parse_end-t_req_end)*1000:.2f} ms, 总耗时: {(t_parse_end-t_start)*1000:.2f} ms")
    if origin:
        return data
    try:
        if not parse:
            return data.get('lyric', '') + "\n" + data.get('trans', '')
        else:
            return parse_lyric(data)
    except Exception:
        return None

def get_song_lyric(*args, **kwargs):
    return asyncio.run(async_get_song_lyric(*args, **kwargs))

def get_album_cover_image(albummid=None, vs=None, singer_mid=None, songmid=None):
    """
    获取QQ音乐歌曲所有可能的封面图片链接，优先返回albummid封面，vs图作为备选。
    支持参数：
        albummid: 专辑mid（优先使用）
        vs: vs字段图片编号列表（备选）
    """
    async def async_check_album_url(url, timeout=0.7):
        try:
            connector = aiohttp.TCPConnector(ssl=False)
            async with aiohttp.ClientSession(headers=headers, connector=connector) as session:
                async with session.get(url, timeout=aiohttp.ClientTimeout(total=timeout)) as resp:
                    print(f"[debug] check album {url} status: {resp.status}")
                    return resp.status == 200
        except Exception as e:
            print(f"[debug] check album {url} exception: {repr(e)}")
            return False

    t_start = time.perf_counter()

    # 优先检测 albummid 封面（大概率可用）
    if albummid and isinstance(albummid, str) and len(albummid) >= 4:
        album_url = ALBUM_COVER_URL_QQ.format(albummid=albummid)
        if asyncio.run(async_check_album_url(album_url, timeout=0.7)):
            t_end = time.perf_counter()
            print(f"[qq] get_album_cover_image: 命中album封面，总耗时: {(t_end-t_start)*1000:.2f} ms")
            return album_url
        else:
            print(f"[qq] get_album_cover_image: album封面无效，尝试vs图")
    else:
        print(f"[qq] get_album_cover_image: 无albummid，尝试vs图")
    
    # albummid封面不可用或不存在时，再检测vs字段图片（备选方案）
    if vs and isinstance(vs, list):
        vs_urls = []
        for v in vs:
            if v and isinstance(v, str) and len(v) >= 4:
                vs_urls.append(f"https://y.qq.com/music/photo_new/T062R300x300M000{v}.jpg")

        async def async_quick_check_url(session, url, timeout=0.5):
            try:
                async with session.get(url, timeout=aiohttp.ClientTimeout(total=timeout)) as resp:
                    print(f"[debug] check {url} status: {resp.status}")
                    return resp.status == 200
            except Exception as e:
                print(f"[debug] check {url} exception: {repr(e)}")
                return False

        async def check_vs_images(urls):
            connector = aiohttp.TCPConnector(ssl=False)
            async with aiohttp.ClientSession(headers=headers, connector=connector) as session:
                tasks = [async_quick_check_url(session, url, timeout=0.5) for url in urls]
                results = await asyncio.gather(*tasks, return_exceptions=True)
                for result, url in zip(results, urls):
                    if result is True:
                        return url
            return None

        if vs_urls:
            url = asyncio.run(check_vs_images(vs_urls))
            t_end = time.perf_counter()
            if url:
                print(f"[qq] get_album_cover_image: 命中vs封面，总耗时: {(t_end-t_start)*1000:.2f} ms")
                return url
    
    t_end = time.perf_counter()
    print(f"[qq] get_album_cover_image: 未命中封面，总耗时: {(t_end-t_start)*1000:.2f} ms")
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
    t_start = time.perf_counter()
    search_str = ' '.join([item for item in [title, artist, album] if item])
    songs = search_with_keyword(search_str, searchType=0, resultNum=10)
    t_search_end = time.perf_counter()
    if not songs or 'list' not in songs or not songs['list']:
        print(f"[qq] search_track: 搜索耗时: {(t_search_end-t_start)*1000:.2f} ms (无结果)")
        return []

    scored_items = []
    for song_item in songs['list']:
        song_title = song_item.get('name', '')
        album_name = song_item.get('album', {}).get('title', '')
        artist_name = ' '.join([s.get('name') for s in song_item.get('singer', [])])
        title_score = textcompare.association(title, song_title)
        artist_score = textcompare.assoc_artists(artist, artist_name) if artist else 1.0
        album_score = textcompare.association(album, album_name) if album else 1.0
        score = 0.6 * title_score + 0.3 * artist_score + 0.1 * album_score
        if score < score_threshold:
            continue
        scored_items.append((score, song_item, song_title, album_name, artist_name))

    import concurrent.futures

    def fetch_detail(args):
        score, song_item, song_title, album_name, artist_name = args
        songmid = song_item.get('mid')
        t_cover_start = time.perf_counter()
        cover_url = get_album_cover_image(
            albummid=song_item.get('album', {}).get('mid', ''),
            vs=song_item.get('vs', []),
            singer_mid=(song_item.get('singer', [{}])[0].get('mid', '') if song_item.get('singer') else ''),
            songmid=song_item.get('mid', '')
        )
        t_cover_end = time.perf_counter()
        t_lyric_start = time.perf_counter()
        lyric_data = get_song_lyric(songmid, parse=True) if songmid else ''
        t_lyric_end = time.perf_counter()
        print(f"[qq] 单曲cover耗时: {(t_cover_end-t_cover_start)*1000:.2f} ms, lyric耗时: {(t_lyric_end-t_lyric_start)*1000:.2f} ms")
        # 判断是否非中文（本语），且有翻译
        def is_non_chinese(text):
            # 简单判断：只要不是全中文就认为是非本语
            import re
            if not text:
                return False
            # 包含非中文字符比例大于50%则认为是非本语
            chinese_chars = re.findall(r'[\u4e00-\u9fff]', text)
            return len(chinese_chars) / max(len(text), 1) < 0.5

        has_translation = False
        if isinstance(lyric_data, dict):
            lines = []
            for item in lyric_data.get('lyric', []):
                time_tag = item.get('time', '')
                lyric_line = item.get('lyric', '')
                trans_line = item.get('trans', '')
                if time_tag and lyric_line:
                    lines.append(f"[{time_tag}]{lyric_line}")
                    if trans_line:
                        lines.append(f"[{time_tag}]{trans_line}")
                        has_translation = True
            lyrics = '\n'.join(lines)
        else:
            lyrics = lyric_data or ''

        # 如果歌曲名、歌手、专辑三者都非中文，且有翻译，则提升分数
        if (is_non_chinese(song_title) or is_non_chinese(artist_name) or is_non_chinese(album_name)) and has_translation:
            score += 0.2

        music_json_data = dict(song_item)
        music_json_data.update({
            "title": song_title,
            "album": album_name,
            "artist": artist_name,
            "lyrics": lyrics,
            "cover": cover_url,
            "id": tools.calculate_md5(f"title:{song_title};artists:{artist_name};album:{album_name}", base='decstr'),
            "score": score,
            "has_translation": has_translation
        })
        return music_json_data

    # 按分数排序，取前max_results个进行并发详情获取
    scored_items.sort(key=lambda x: x[0], reverse=True)
    top_items = scored_items[:max_results]
    results = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=3) as executor:  # 限制并发数为3，避免过多连接
        future_to_item = {executor.submit(fetch_detail, item): item for item in top_items}
        for future in concurrent.futures.as_completed(future_to_item):
            results.append(future.result())
    t_end = time.perf_counter()
    print(f"[qq] search_track: 总耗时: {(t_end-t_start)*1000:.2f} ms")
    return results

def search(title='', artist='', album=''):
    t_start = time.perf_counter()
    title = str(title) if title else ''
    artist = str(artist) if artist else ''
    album = str(album) if album else ''
    if not any((title, artist, album)):
        return None
    title = title.strip()
    artist = artist.strip()
    album = album.strip()
    if title:
        res = search_track(title=title, artist=artist, album=album)
        t_end = time.perf_counter()
        print(f"[qq] search: 总耗时: {(t_end-t_start)*1000:.2f} ms")
        return res
    return None