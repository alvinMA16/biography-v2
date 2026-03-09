package realtime

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/peizhengma/biography-v2/internal/provider/llm"
)

const (
	realtimeRecentTurnLimit         = 6
	longUserTurnThresholdRunes      = 320
	longTurnSummarySentenceLimit    = 2
	longTurnFactsLimit              = 5
	longTurnFollowupCandidatesLimit = 3
	defaultTopicContextMaxRunes     = 120
	longTurnFocusMaxRunes           = 120
	longTurnSummaryMaxRunes         = 90
	longTurnFactMaxRunes            = 40
)

var (
	yearPattern  = regexp.MustCompile(`(?:19|20)\d{2}年?`)
	placePattern = regexp.MustCompile(`[\p{Han}]{2,10}(?:省|市|县|区|镇|村|厂|学校|大学|部队)`)
)

var (
	personKeywords = []string{"父亲", "母亲", "爸爸", "妈妈", "爷爷", "奶奶", "外公", "外婆", "哥哥", "姐姐", "弟弟", "妹妹", "爱人", "老伴", "丈夫", "妻子", "儿子", "女儿", "老师", "厂长", "班主任", "同学", "同事"}
	eventKeywords  = []string{"搬家", "结婚", "上学", "读书", "参军", "当兵", "进厂", "工作", "下乡", "高考", "毕业", "生病", "创业", "退休", "调动", "回城"}
)

type ChatContextPacket struct {
	CurrentUserTurn TurnContext
	RecentTurns     []TurnContext
	Topic           TopicContext
	CoreProfile     CoreProfile
	RecentSummary   string
	Constraints     AssistantConstraints
}

type TurnContext struct {
	Role         string
	RawText      string
	WorkingText  string
	IsCompressed bool
}

type TopicContext struct {
	Title   string
	Context string
}

type CoreProfile struct {
	UserName  string
	BirthYear *int
	Hometown  string
	MainCity  string
}

type AssistantConstraints struct {
	MaxSentences int
	VoiceFirst   bool
}

func buildChatContextPacket(config *SessionConfig, messages []llm.Message) ChatContextPacket {
	conversation := make([]llm.Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		conversation = append(conversation, msg)
	}

	packet := ChatContextPacket{
		Topic: TopicContext{
			Title:   strings.TrimSpace(config.TopicTitle),
			Context: truncateRunes(strings.TrimSpace(config.TopicContext), defaultTopicContextMaxRunes),
		},
		CoreProfile: CoreProfile{
			UserName:  strings.TrimSpace(config.UserName),
			BirthYear: config.BirthYear,
			Hometown:  strings.TrimSpace(config.Hometown),
			MainCity:  strings.TrimSpace(config.MainCity),
		},
		Constraints: AssistantConstraints{
			MaxSentences: 3,
			VoiceFirst:   true,
		},
	}

	if len(conversation) == 0 {
		return packet
	}

	currentIdx := len(conversation) - 1
	packet.CurrentUserTurn = buildTurnContext(conversation[currentIdx])

	history := conversation[:currentIdx]
	if len(history) > realtimeRecentTurnLimit {
		history = history[len(history)-realtimeRecentTurnLimit:]
	}
	packet.RecentTurns = make([]TurnContext, 0, len(history))
	for _, msg := range history {
		packet.RecentTurns = append(packet.RecentTurns, buildTurnContext(msg))
	}

	return packet
}

func buildTurnContext(msg llm.Message) TurnContext {
	raw := strings.TrimSpace(msg.Content)

	return TurnContext{
		Role:         msg.Role,
		RawText:      raw,
		WorkingText:  raw,
		IsCompressed: false,
	}
}

func buildInferenceMessages(packet ChatContextPacket, systemPrompt string) []llm.Message {
	inferenceMessages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}

	for _, turn := range packet.RecentTurns {
		if strings.TrimSpace(turn.WorkingText) == "" {
			continue
		}
		inferenceMessages = append(inferenceMessages, llm.Message{
			Role:    turn.Role,
			Content: turn.WorkingText,
		})
	}

	if strings.TrimSpace(packet.CurrentUserTurn.WorkingText) != "" {
		inferenceMessages = append(inferenceMessages, llm.Message{
			Role:    packet.CurrentUserTurn.Role,
			Content: packet.CurrentUserTurn.WorkingText,
		})
	}

	return inferenceMessages
}

func buildLongTurnWorkingText(raw string) string {
	sentences := splitSentences(raw)
	lastFocus := selectLastFocus(sentences)
	summary := selectStorySummary(sentences, raw)
	facts := extractFacts(raw, sentences)
	followups := extractFollowupCandidates(raw, sentences)

	var sb strings.Builder
	sb.WriteString("这是用户本轮长回复的摘要卡，请基于这些信息继续追问，不要复述整段内容。\n")
	if lastFocus != "" {
		sb.WriteString("最后落点：")
		sb.WriteString(lastFocus)
		sb.WriteString("\n")
	}
	if summary != "" {
		sb.WriteString("主线：")
		sb.WriteString(summary)
		sb.WriteString("\n")
	}
	if len(facts) > 0 {
		sb.WriteString("关键信息：")
		sb.WriteString(strings.Join(facts, "；"))
		sb.WriteString("\n")
	}
	if len(followups) > 0 {
		sb.WriteString("可追问点：")
		sb.WriteString(strings.Join(followups, "；"))
	}
	return strings.TrimSpace(sb.String())
}

func splitSentences(text string) []string {
	var result []string
	var current strings.Builder
	flush := func() {
		s := normalizeWhitespace(current.String())
		if s != "" {
			result = append(result, s)
		}
		current.Reset()
	}

	for _, r := range text {
		current.WriteRune(r)
		switch r {
		case '。', '！', '？', '；', '\n':
			flush()
		}
	}
	flush()
	return result
}

func selectLastFocus(sentences []string) string {
	if len(sentences) == 0 {
		return ""
	}
	start := max(len(sentences)-longTurnSummarySentenceLimit, 0)
	lastSentences := make([]string, 0, len(sentences[start:]))
	for _, sentence := range sentences[start:] {
		lastSentences = append(lastSentences, truncateRunes(sentence, longTurnFocusMaxRunes))
	}
	return strings.Join(lastSentences, " ")
}

func selectStorySummary(sentences []string, raw string) string {
	bestSentence := ""
	bestScore := -1
	for _, sentence := range sentences {
		score := scoreSentence(sentence)
		if score > bestScore {
			bestScore = score
			bestSentence = sentence
		}
	}
	if bestSentence == "" {
		bestSentence = truncateRunes(normalizeWhitespace(raw), longTurnSummaryMaxRunes)
	}
	return truncateRunes(bestSentence, longTurnSummaryMaxRunes)
}

func scoreSentence(sentence string) int {
	score := utf8.RuneCountInString(sentence)
	if yearPattern.MatchString(sentence) {
		score += 20
	}
	if placePattern.MatchString(sentence) {
		score += 15
	}
	for _, keyword := range personKeywords {
		if strings.Contains(sentence, keyword) {
			score += 10
		}
	}
	for _, keyword := range eventKeywords {
		if strings.Contains(sentence, keyword) {
			score += 8
		}
	}
	return score
}

func extractFacts(raw string, sentences []string) []string {
	facts := make([]string, 0, longTurnFactsLimit)
	seen := map[string]struct{}{}
	appendFact := func(prefix, value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		fact := prefix + value
		if _, ok := seen[fact]; ok {
			return
		}
		seen[fact] = struct{}{}
		facts = append(facts, truncateRunes(fact, longTurnFactMaxRunes))
	}

	for _, match := range yearPattern.FindAllString(raw, -1) {
		appendFact("时间：", match)
		if len(facts) >= longTurnFactsLimit {
			return facts
		}
	}
	for _, match := range placePattern.FindAllString(raw, -1) {
		appendFact("地点：", match)
		if len(facts) >= longTurnFactsLimit {
			return facts
		}
	}
	for _, keyword := range personKeywords {
		if strings.Contains(raw, keyword) {
			appendFact("人物：", keyword)
			if len(facts) >= longTurnFactsLimit {
				return facts
			}
		}
	}
	for _, keyword := range eventKeywords {
		if strings.Contains(raw, keyword) {
			appendFact("事件：", keyword)
			if len(facts) >= longTurnFactsLimit {
				return facts
			}
		}
	}

	for _, sentence := range sentences {
		if len(facts) >= longTurnFactsLimit {
			break
		}
		if scoreSentence(sentence) < 45 {
			continue
		}
		appendFact("线索：", sentence)
	}

	return facts
}

func extractFollowupCandidates(raw string, sentences []string) []string {
	candidates := make([]string, 0, longTurnFollowupCandidatesLimit)
	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(candidates, value) {
			return
		}
		candidates = append(candidates, value)
	}

	hasPerson := containsAny(raw, personKeywords)
	hasEvent := containsAny(raw, eventKeywords)
	hasTime := yearPattern.MatchString(raw) || strings.Contains(raw, "那时候") || strings.Contains(raw, "后来")
	hasPlace := placePattern.MatchString(raw) || strings.Contains(raw, "老家") || strings.Contains(raw, "家乡")

	if hasEvent {
		appendCandidate("这件事是怎么发生、怎么推进的")
	}
	if hasPerson {
		appendCandidate("关键人物当时是怎么决定、怎么影响您的")
	}
	appendCandidate("您当时心里最强烈的感受是什么")
	if hasTime {
		appendCandidate("那大概是在什么阶段、前后发生了什么")
	}
	if hasPlace {
		appendCandidate("那个地方当时的环境和生活是什么样")
	}

	if len(candidates) == 0 && len(sentences) > 0 {
		appendCandidate("您最想展开讲的细节是什么")
	}

	if len(candidates) > longTurnFollowupCandidatesLimit {
		candidates = candidates[:longTurnFollowupCandidatesLimit]
	}
	return candidates
}

func containsAny(text string, keywords []string) bool {
	for _, keyword := range keywords {
		if strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func truncateRunes(text string, limit int) string {
	text = normalizeWhitespace(text)
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func normalizeWhitespace(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatTurnContextForLog(turn TurnContext) string {
	if !turn.IsCompressed {
		return truncateRunes(turn.WorkingText, 80)
	}
	return fmt.Sprintf("compressed=%q", truncateRunes(turn.WorkingText, 80))
}
