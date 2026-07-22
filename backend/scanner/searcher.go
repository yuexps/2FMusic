package scanner

import (
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"2fmusic/backend/core"
)

var httpClient = &http.Client{
	Timeout: 5 * time.Second,
}

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

// SearchSongFastSequential 批量顺序刮削 (网易云 -> QQ -> 酷狗)
func SearchSongFastSequential(title, artist, album string) map[string]interface{} {
	if title == "" {
		return nil
	}

	sources := []string{"netease", "qq", "kugou"}
	var bestResult *searchResult
	bestScore := 0.0

	for _, source := range sources {
		var list []searchResult
		switch source {
		case "netease":
			list = searchNetease(title, artist)
		case "qq":
			list = searchQQ(title, artist)
		case "kugou":
			list = searchKugou(title, artist)
		}

		if len(list) == 0 {
			continue
		}

		var currentBest *searchResult
		currentBestScore := 0.0

		for _, item := range list {
			if isExactMatch(title, artist, item.title, item.artist) && (item.cover != "" || len(item.lyrics) > 20) {
				return map[string]interface{}{
					"title":  item.title,
					"artist": item.artist,
					"album":  item.album,
					"lyrics": item.lyrics,
					"cover":  item.cover,
					"source": item.source,
				}
			}

			titleScore := StringSimilarity(normalizeLyricMatchText(title), normalizeLyricMatchText(item.title))
			artistScore := 1.0
			if artist != "" {
				artistScore = calculateArtistMatchSimilarity(artist, item.artist)
			}
			albumScore := 1.0
			if album != "" {
				albumScore = StringSimilarity(normalizeLyricMatchText(album), normalizeLyricMatchText(item.album))
			}

			score := 0.5*titleScore + 0.35*artistScore + 0.15*albumScore
			if album != "" && strings.EqualFold(strings.TrimSpace(item.album), strings.TrimSpace(album)) {
				score += 0.2
			}
			if item.hasTranslation {
				score += 0.02
			}
			switch item.platformRank {
			case 0:
				score += 0.05
			case 1:
				score += 0.03
			case 2:
				score += 0.01
			}

			if score > currentBestScore {
				currentBestScore = score
				itemCopy := item
				currentBest = &itemCopy
			}
		}

		if currentBestScore > bestScore && currentBest != nil {
			bestScore = currentBestScore
			bestResult = currentBest
		}
	}

	if bestResult == nil {
		return nil
	}

	return map[string]interface{}{
		"title":           bestResult.title,
		"artist":          bestResult.artist,
		"album":           bestResult.album,
		"lyrics":          bestResult.lyrics,
		"cover":           bestResult.cover,
		"source":          bestResult.source,
		"has_translation": bestResult.hasTranslation,
	}
}

// SearchSongBest 并发检索 QQ/网易云/酷狗 返回最佳匹配
func SearchSongBest(title, artist, album string) map[string]interface{} {
	return SearchSongBestWithDuration(title, artist, album, 0)
}

func SearchSongBestWithDuration(title, artist, album string, durationMs int) map[string]interface{} {
	if title == "" {
		return nil
	}

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
				res = searchNetease(title, artist)
			case "qq":
				res = searchQQ(title, artist)
			case "kugou":
				res = searchKugou(title, artist)
			}

			mu.Lock()
			results = append(results, res...)
			mu.Unlock()
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
		core.Info("刮削搜索超时 (6s)，基于已返回的结果打分排序")
	}

	// 拷贝已返回的结果，避免并发读写
	mu.Lock()
	resultsCopy := make([]searchResult, len(results))
	copy(resultsCopy, results)
	mu.Unlock()

	if len(resultsCopy) == 0 {
		return nil
	}

	type scoredItem struct {
		score float64
		item  searchResult
	}

	var scored []scoredItem
	apiBonus := map[string]float64{"qq": 0.01, "netease": 0.005, "kugou": 0.0}

	for _, item := range resultsCopy {
		titleSim := StringSimilarity(normalizeLyricMatchText(title), normalizeLyricMatchText(item.title))
		artistSim := 1.0
		if artist != "" {
			artistSim = calculateArtistMatchSimilarity(artist, item.artist)
		}
		albumSim := 1.0
		if album != "" && item.album != "" {
			albumSim = StringSimilarity(normalizeLyricMatchText(album), normalizeLyricMatchText(item.album))
		}

		// SCORE_WEIGHTS: title 45, artist 25, album 30
		identityScore := 0.45*titleSim + 0.25*artistSim + 0.30*albumSim
		if titleSim < 0.65 || artistSim < 0.5 {
			identityScore = math.Min(identityScore, 0.74)
		}

		durMult := calculateDurationMultiplier(durationMs, item.durationMs)
		finalScore := identityScore * durMult

		finalScore += apiBonus[item.source]

		if item.hasTranslation {
			scoreBonus := 0.02
			finalScore += scoreBonus
		}

		switch item.platformRank {
		case 0:
			finalScore += 0.05
		case 1:
			finalScore += 0.03
		case 2:
			finalScore += 0.01
		}

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
		return nil
	}

	return map[string]interface{}{
		"title":           best.title,
		"artist":          best.artist,
		"album":           best.album,
		"lyrics":          best.lyrics,
		"cover":           best.cover,
		"source":          best.source,
		"has_translation": best.hasTranslation,
	}
}

