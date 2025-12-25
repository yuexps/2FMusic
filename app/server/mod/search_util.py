import threading
from mod.searchx import qq_new, netease, kugou
from mod import textcompare

API_BONUS = {'qq': 0.01, 'netease': 0.005, 'kugou': 0.0}

def search_song_best(title, artist, album):
    """
    并发搜索三大平台，返回最优匹配结果（dict），无则返回None。
    """

    def filter_music_json(item):
        # 只保留标准字段
        return {
            "title": item.get("title", ""),
            "album": item.get("album", ""),
            "artist": item.get("artist", ""),
            "lyrics": item.get("lyrics", ""),
            "cover": item.get("cover", ""),
            "id": item.get("id", "")
        }

    def search_api(api_func, title, artist, album, result_list, source):
        results = api_func(title=title, artist=artist, album=album)
        if results:
            for item in results:
                result_list.append(filter_music_json(item))

    threads = []
    all_results = []
    apis = [
        (qq_new.search, 'qq'),
        (netease.search, 'netease'),
        (kugou.search, 'kugou')
    ]
    for api, source in apis:
        t = threading.Thread(target=search_api, args=(api, title, artist, album, all_results, source))
        threads.append(t)
        t.start()
    for t in threads:
        t.join()
    scored = []
    for item in all_results:
        title_score = textcompare.association(title, item.get('title', ''))
        artist_score = textcompare.assoc_artists(artist, item.get('artist', '')) if artist else 1.0
        album_score = textcompare.association(album, item.get('album', '')) if album else 1.0
        score = 0.6 * title_score + 0.3 * artist_score + 0.1 * album_score
        score += API_BONUS.get(item.get('source'), 0.0)
        scored.append((score, item))
    scored.sort(reverse=True, key=lambda x: x[0])
    # 优先选有cover的高分结果
    best = None
    for score, item in scored:
        if item.get('cover'):
            best = item
            break
    if not best and scored:
        best = scored[0][1]
    if best:
        lyrics_preview = best.get('lyrics')
        if lyrics_preview:
            lyrics_preview = lyrics_preview[:20] + '...' if len(lyrics_preview) > 20 else lyrics_preview
        print(f"[search_util] 最优结果: source={best.get('source')} title={best.get('title')} artist={best.get('artist')} album={best.get('album')} cover={bool(best.get('cover'))} lyrics_preview={lyrics_preview}")
    return best
