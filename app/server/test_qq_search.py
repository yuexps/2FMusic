from mod.searchx import qq

if __name__ == "__main__":
    # 测试：搜索歌曲“可能”，歌手“程响”
    result = qq.search(title="恋爱循环", artist="花泽香菜", album="")
    print("搜索结果：")
    for item in result:
        print(f"标题: {item['title']}")
        print(f"歌手: {item['artist']}")
        print(f"专辑: {item['album']}")
        print(f"封面: {item['cover']}")
        print(f"歌词: {item['lyrics']}" if item['lyrics'] else "歌词: 无")
        #if item['lyrics']:
        #    lrc = item['lyrics']
        #    print(f"歌词: {lrc[:20]}{'...' if len(lrc) > 20 else ''}")
        #else:
        #    print("歌词: 无")
        print(f"ID: {item['id']}")
        print('-'*40)