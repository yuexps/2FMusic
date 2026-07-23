package scanner

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"2fmusic/backend/core"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

type sfCall struct {
	wg  sync.WaitGroup
	val map[string]interface{}
}

// singleflightGroup 并发请求去重
type singleflightGroup struct {
	mu sync.Mutex
	m  map[string]*sfCall
}

func (g *singleflightGroup) Do(key string, fn func() map[string]interface{}) map[string]interface{} {
	g.mu.Lock()
	if g.m == nil {
		g.m = make(map[string]*sfCall)
	}
	if c, ok := g.m[key]; ok {
		g.mu.Unlock()
		c.wg.Wait()
		return c.val
	}
	c := new(sfCall)
	c.wg.Add(1)
	g.m[key] = c
	g.mu.Unlock()

	c.val = fn()
	c.wg.Done()

	g.mu.Lock()
	delete(g.m, key)
	g.mu.Unlock()

	return c.val
}

var searchSingleflight singleflightGroup

type ttlCacheEntry struct {
	val       map[string]interface{}
	createdAt time.Time
}

type ttlCache struct {
	mu sync.Mutex
	m  map[string]ttlCacheEntry
}

func (c *ttlCache) Get(key string) (map[string]interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		return nil, false
	}
	entry, ok := c.m[key]
	if !ok {
		return nil, false
	}
	if time.Since(entry.createdAt) > 10*time.Second {
		delete(c.m, key)
		return nil, false
	}
	return entry.val, true
}

func (c *ttlCache) Set(key string, val map[string]interface{}) {
	if val == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]ttlCacheEntry)
	}
	c.m[key] = ttlCacheEntry{
		val:       val,
		createdAt: time.Now(),
	}
}

var searchCache ttlCache

type searchResult struct {
	title          string
	artist         string
	album          string
	lyrics         string
	cover          string
	source         string
	platformRank   int
	durationMs     int
	hasTranslation bool
}

// executeWithFallback 刮削退避执行器
func executeWithFallback(tag, title string, searchFn func(searchTitle string) map[string]interface{}) map[string]interface{} {
	res := searchFn(title)
	if res != nil {
		return res
	}

	cleanedTitle := cleanTitleForSearch(title)
	if cleanedTitle != "" && cleanedTitle != title {
		core.Info("[%s] 原始标题 '%s' 未命中，剔除修饰后缀重试: search_title='%s'", tag, title, cleanedTitle)
		return searchFn(cleanedTitle)
	}

	return nil
}

// SearchSongFastSequential 批量自适应刮削（网易云短路优先 + 降级并发）
func SearchSongFastSequential(title, artist, album string, durationMs int) map[string]interface{} {
	key := fmt.Sprintf("fast|%s|%s|%s|%d", title, artist, album, durationMs)
	if val, ok := searchCache.Get(key); ok {
		return val
	}
	res := searchSingleflight.Do(key, func() map[string]interface{} {
		return executeWithFallback("SearchSongFastSequential", title, func(searchTitle string) map[string]interface{} {
			if searchTitle == "" {
				return nil
			}

			dur := durationMs

			// 1. 网易云短路匹配
			neteaseList := searchNetease(searchTitle, artist)
			if len(neteaseList) > 0 {
				var currentBest *searchResult
				currentBestScore := 0.0
				for _, item := range neteaseList {
					score := calculateItemMatchScore(searchTitle, artist, album, dur, item)
					if score > currentBestScore {
						currentBestScore = score
						itemCopy := item
						currentBest = &itemCopy
					}
				}

				// 得分 >= 0.75 且包含封面时触发短路
				if currentBest != nil && currentBestScore >= 0.75 && currentBest.cover != "" {
					core.Info("[SearchSongFast] 网易云高置信度命中: title='%s', artist='%s', score=%.2f", currentBest.title, currentBest.artist, currentBestScore)
					return map[string]interface{}{
						"title":           currentBest.title,
						"artist":          currentBest.artist,
						"album":           currentBest.album,
						"lyrics":          currentBest.lyrics,
						"cover":           currentBest.cover,
						"source":          currentBest.source,
						"has_translation": currentBest.hasTranslation,
					}
				}
			}

			// 2. 降级并发查询 QQ 与酷狗
			var results []searchResult
			var mu sync.Mutex
			var wg sync.WaitGroup

			sources := []string{"qq", "kugou"}
			for _, src := range sources {
				wg.Add(1)
				go func(source string) {
					defer wg.Done()
					var l []searchResult
					switch source {
					case "qq":
						l = searchQQ(searchTitle, artist)
					case "kugou":
						l = searchKugou(searchTitle, artist)
					}
					if len(l) > 0 {
						mu.Lock()
						results = append(results, l...)
						mu.Unlock()
					}
				}(src)
			}
			wg.Wait()

			if len(results) == 0 {
				core.Info("[SearchSongFast] 未检索到候选曲目: title='%s', artist='%s'", searchTitle, artist)
				return nil
			}

			var bestItem *searchResult
			bestScore := 0.0
			for _, item := range results {
				score := calculateItemMatchScore(searchTitle, artist, album, dur, item)
				if score > bestScore {
					bestScore = score
					itemCopy := item
					bestItem = &itemCopy
				}
			}

			if bestItem == nil || bestScore < 0.60 {
				core.Info("[SearchSongFast] 候选曲目得分未达阈值 (max_score=%.2f): title='%s'", bestScore, searchTitle)
				return nil
			}

			core.Info("[SearchSongFast] 备选源检索成功: source=%s, title='%s', artist='%s', score=%.2f",
				bestItem.source, bestItem.title, bestItem.artist, bestScore)

			return map[string]interface{}{
				"title":           bestItem.title,
				"artist":          bestItem.artist,
				"album":           bestItem.album,
				"lyrics":          bestItem.lyrics,
				"cover":           bestItem.cover,
				"source":          bestItem.source,
				"has_translation": bestItem.hasTranslation,
			}
		})
	})
	if res != nil {
		searchCache.Set(key, res)
	}
	return res
}

// SearchSongBest 全网并发精细刮削（QQ/网易云/酷狗）
func SearchSongBest(title, artist, album string, durationMs int) map[string]interface{} {
	key := fmt.Sprintf("best|%s|%s|%s|%d", title, artist, album, durationMs)
	if val, ok := searchCache.Get(key); ok {
		return val
	}
	res := searchSingleflight.Do(key, func() map[string]interface{} {
		return executeWithFallback("SearchSongBest", title, func(searchTitle string) map[string]interface{} {
			if searchTitle == "" {
				return nil
			}

			core.Info("[SearchSongBest] 开始全源并发检索: title='%s', artist='%s', album='%s'", searchTitle, artist, album)

			var results []searchResult
			var mu sync.Mutex
			var wg sync.WaitGroup

			sources := []string{"netease", "qq", "kugou"}

			for _, src := range sources {
				wg.Add(1)
				go func(source string) {
					defer wg.Done()
					var res []searchResult

					switch source {
					case "netease":
						res = searchNetease(searchTitle, artist)
					case "qq":
						res = searchQQ(searchTitle, artist)
					case "kugou":
						res = searchKugou(searchTitle, artist)
					}

					if len(res) > 0 {
						mu.Lock()
						results = append(results, res...)
						mu.Unlock()
					}
				}(src)
			}

			c := make(chan struct{})
			go func() {
				wg.Wait()
				close(c)
			}()

			select {
			case <-c:
			case <-time.After(6 * time.Second):
				core.Info("[SearchSongBest] 检索超时 (6s)，基于已返回结果计算")
			}

			mu.Lock()
			resultsCopy := make([]searchResult, len(results))
			copy(resultsCopy, results)
			mu.Unlock()

			if len(resultsCopy) == 0 {
				core.Info("[SearchSongBest] 未检索到有效候选曲目: title='%s'", searchTitle)
				return nil
			}

			type scoredItem struct {
				score float64
				item  searchResult
			}

			var scored []scoredItem
			for _, item := range resultsCopy {
				finalScore := calculateItemMatchScore(searchTitle, artist, album, durationMs, item)
				scored = append(scored, scoredItem{score: finalScore, item: item})
			}

			sort.Slice(scored, func(i, j int) bool {
				return scored[i].score > scored[j].score
			})

			var best *searchResult
			for _, sc := range scored {
				if sc.score >= 0.75 && sc.item.cover != "" && len(sc.item.lyrics) > 50 {
					best = &sc.item
					break
				}
			}

			if best == nil {
				for _, sc := range scored {
					if sc.score >= 0.70 && sc.item.cover != "" {
						best = &sc.item
						break
					}
				}
			}

			if best == nil && len(scored) > 0 && scored[0].score >= 0.65 {
				best = &scored[0].item
			}

			if best == nil {
				core.Info("[SearchSongBest] 候选曲目得分未达阈值 (max_score=%.2f, threshold=0.65): title='%s'", scored[0].score, searchTitle)
				return nil
			}

			core.Info("[SearchSongBest] 匹配成功: source=%s, title='%s', artist='%s', score=%.2f",
				best.source, best.title, best.artist, calculateItemMatchScore(searchTitle, artist, album, durationMs, *best))

			// 搜集同次检索中置信度合格且带封面的备选候选 URL (最多截取前 3 个优选源)
			var candidateCovers []map[string]string
			for _, sc := range scored {
				if sc.score >= 0.55 && sc.item.cover != "" {
					candidateCovers = append(candidateCovers, map[string]string{
						"source": sc.item.source,
						"cover":  sc.item.cover,
					})
					if len(candidateCovers) >= 3 {
						break
					}
				}
			}

			return map[string]interface{}{
				"title":            best.title,
				"artist":           best.artist,
				"album":            best.album,
				"lyrics":           best.lyrics,
				"cover":            best.cover,
				"source":           best.source,
				"has_translation":  best.hasTranslation,
				"candidate_covers": candidateCovers,
			}
		})
	})
	if res != nil {
		searchCache.Set(key, res)
	}
	return res
}
