import sys
import os
sys.path.append(os.path.join(os.path.dirname(os.path.abspath(__file__)), 'lib'))
import threading
from mod.searchx import qq_new, netease, kugou
from mod import textcompare

def search_api(api_func, title, artist, album, result_list, source):
    results = api_func(title=title, artist=artist, album=album)
    if results:
        for item in results:
            item = dict(item)
            item['source'] = source
            result_list.append(item)

# 定义API优先级加分
API_BONUS = {'qq': 0.01, 'netease': 0.005, 'kugou': 0.0}

def main(title, artist, album):
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
        score += API_BONUS.get(item.get('source'), 0.0)  # 微小加分
        scored.append((score, item))
    scored.sort(reverse=True, key=lambda x: x[0])
    best = scored[0][1] if scored else None
    print('最有效结果:', best)
    print('\n全部结果按相似度排序:')
    for s, item in scored:
        print(f"[source={item.get('source')}] score={s:.3f} title={item.get('title')} artist={item.get('artist')} album={item.get('album')} cover={item.get('cover')}")
    return best

if __name__ == "__main__":
    # 示例：
    main(title="可能", artist="程响", album="")
