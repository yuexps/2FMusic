package scanner

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

type lrcLine struct {
	ts      string
	content string
}

var (
	lrcTimeRegexp      = regexp.MustCompile(`\[(\d{2}:\d{2}(?:\.\d{2,3})?)\]`)
	lrcTimeCleanRegexp = regexp.MustCompile(`(\[\d{2}:\d{2}(?:\.\d{2,3})?\])+`)
)

func parseLrcLines(lrcText string) []lrcLine {
	var result []lrcLine
	lines := strings.Split(lrcText, "\n")
	for _, line := range lines {
		matches := lrcTimeRegexp.FindAllStringSubmatch(line, -1)
		content := strings.TrimSpace(lrcTimeCleanRegexp.ReplaceAllString(line, ""))
		if len(matches) > 0 && content != "" {
			for _, m := range matches {
				if len(m) >= 2 {
					result = append(result, lrcLine{ts: m[1], content: content})
				}
			}
		}
	}
	return result
}

func mergeLyricsWithTranslation(originLyric, transLyric string) (string, bool) {
	originLyric = strings.TrimSpace(originLyric)
	transLyric = strings.TrimSpace(transLyric)
	if transLyric == "" {
		return originLyric, false
	}
	if originLyric == "" {
		return "", false
	}

	originList := parseLrcLines(originLyric)
	transList := parseLrcLines(transLyric)

	if len(transList) == 0 {
		return originLyric, false
	}

	var merged []string
	i, j := 0, 0
	for i < len(originList) || j < len(transList) {
		if i < len(originList) && (j >= len(transList) || originList[i].ts <= transList[j].ts) {
			merged = append(merged, fmt.Sprintf("[%s]%s", originList[i].ts, originList[i].content))
			i++
		} else if j < len(transList) {
			merged = append(merged, fmt.Sprintf("[%s]%s", transList[j].ts, transList[j].content))
			j++
		}
	}

	return strings.Join(merged, "\n"), true
}

// LongestCommonSubstring 最长公共子串算法
func LongestCommonSubstring(str1, str2 string) int {
	r1, r2 := []rune(str1), []rune(str2)
	m, n := len(r1), len(r2)
	if m == 0 || n == 0 {
		return 0
	}
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	maxLen := 0
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if r1[i-1] == r2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
				if dp[i][j] > maxLen {
					maxLen = dp[i][j]
				}
			}
		}
	}
	return maxLen
}

// CharDuplicateRate 字符交并比算法
func CharDuplicateRate(str1, str2 string) float64 {
	set1 := make(map[rune]bool)
	set2 := make(map[rune]bool)
	for _, r := range []rune(str1) {
		set1[r] = true
	}
	for _, r := range []rune(str2) {
		set2[r] = true
	}
	if len(set1) == 0 || len(set2) == 0 {
		return 0.0
	}
	intersection := 0
	unionMap := make(map[rune]bool)
	for r := range set1 {
		unionMap[r] = true
		if set2[r] {
			intersection++
		}
	}
	for r := range set2 {
		unionMap[r] = true
	}
	return float64(intersection) / float64(len(unionMap))
}

// StringSimilarity 计算复合文本相似度
func StringSimilarity(s1, s2 string) float64 {
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	if s1 == s2 {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}

	r1Len := float64(len([]rune(s1)))
	lcs := float64(LongestCommonSubstring(s1, s2))
	commonRatio := lcs / r1Len
	if commonRatio > 1.0 {
		commonRatio = 1.0
	}

	dupRate := CharDuplicateRate(s1, s2)
	similarRatio := commonRatio * math.Pow(math.Sqrt(dupRate), 1.0/1.5)
	return similarRatio
}

var versionMarkerReg = regexp.MustCompile(`(?i)(instrumental|inst|off\s*vocal|karaoke|remix|mix|version|ver\.?|cover|live|edit|arrange|伴奏|カラオケ|インスト|リミックス|remaster|remastered)`)
var featTagReg = regexp.MustCompile(`(?i)[\(\[（【]\s*(feat|featuring|ft|with)\.?\s+[^\)\]）】]+[\)\]）】]|\b(feat|featuring|ft|with)\.?\s+.+$`)

// normalizeLyricMatchText 清理歌名中的版本与修饰标签
func normalizeLyricMatchText(val string) string {
	s := featTagReg.ReplaceAllString(val, "")
	s = versionMarkerReg.ReplaceAllString(s, "")
	s = strings.ToLower(s)
	reg := regexp.MustCompile(`[\p{P}\p{S}\s]+`)
	return strings.TrimSpace(reg.ReplaceAllString(s, ""))
}

// calculateArtistMatchSimilarity 计算歌手匹配度（包含多歌手切分与主歌手保护）
func calculateArtistMatchSimilarity(target, search string) float64 {
	splitReg := regexp.MustCompile(`(?i)[,&、\/]|feat\.?|ft\.?|featuring|与`)
	tArtists := splitReg.Split(target, -1)
	sArtists := splitReg.Split(search, -1)

	var cleanT []string
	for _, a := range tArtists {
		if ca := normalizeLyricMatchText(a); ca != "" {
			cleanT = append(cleanT, ca)
		}
	}
	var cleanS []string
	for _, a := range sArtists {
		if ca := normalizeLyricMatchText(a); ca != "" {
			cleanS = append(cleanS, ca)
		}
	}

	if len(cleanT) == 0 || len(cleanS) == 0 {
		return StringSimilarity(target, search)
	}

	matchCount := 0
	for _, a1 := range cleanT {
		for _, a2 := range cleanS {
			if a1 == a2 || (len(a1) >= 3 && strings.Contains(a2, a1)) || (len(a2) >= 3 && strings.Contains(a1, a2)) {
				matchCount++
				break
			}
		}
	}

	tokenSim := float64(matchCount) / math.Max(float64(len(cleanT)), float64(len(cleanS)))
	isMainMatched := cleanT[0] == cleanS[0] || (len(cleanT[0]) >= 3 && strings.Contains(cleanS[0], cleanT[0])) || (len(cleanS[0]) >= 3 && strings.Contains(cleanT[0], cleanS[0]))

	mainBonus := tokenSim
	if isMainMatched {
		mainBonus = math.Max(tokenSim, 0.7)
	}
	return math.Max(mainBonus, StringSimilarity(target, search))
}

// calculateDurationMultiplier 计算时长偏差惩罚乘子
func calculateDurationMultiplier(targetMs, searchMs int) float64 {
	if targetMs <= 0 || searchMs <= 0 {
		return 0.9
	}
	diff := math.Abs(float64(targetMs - searchMs))
	if diff <= 1000 {
		return 1.0
	}
	if diff <= 3000 {
		return 0.95
	}
	if diff <= 5000 {
		return 0.75
	}
	if diff <= 10000 {
		return 0.35
	}
	return 0.1
}

// isExactMatch 判断是否完全匹配
func isExactMatch(title, artist, searchTitle, searchArtist string) bool {
	cTitle := normalizeLyricMatchText(title)
	cSearchTitle := normalizeLyricMatchText(searchTitle)
	if cTitle == "" || cTitle != cSearchTitle {
		return false
	}

	if artist == "" {
		return true
	}

	cArtist := normalizeLyricMatchText(artist)
	cSearchArtist := normalizeLyricMatchText(searchArtist)
	if cSearchArtist == "" {
		return true
	}

	return strings.Contains(cSearchArtist, cArtist) || strings.Contains(cArtist, cSearchArtist)
}
