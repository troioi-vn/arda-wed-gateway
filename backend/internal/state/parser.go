package state

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	ansiRegex   = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)
	promptRegex = regexp.MustCompile(`\(\s*(\d+)\s*/\s*(\d+)\s+(\d+)\s*/\s*(\d+)\s+(\d+)\s*/\s*(\d+)\s+(\d+)\s+\|\s*\)`)
	tagRegex    = regexp.MustCompile(`\(([^()]{1,64})\)`)
)

var knownStatusTags = map[string]string{
	"белая аура":   "Белая Аура",
	"серая аура":   "СераЯ Аура",
	"красная аура": "Красная Аура",
	"в полете":     "В полете",
	"плавает":      "Плавает",
	"светится":     "Светится",
	"волшебное":    "Волшебное",
}

var equipmentAliases = map[string][]string{
	"head":      {"head", "голова", "на голове"},
	"neck":      {"neck", "шея", "на шее"},
	"body":      {"body", "тело", "туловище", "на теле"},
	"fingers":   {"fingers", "finger", "палец", "пальцы", "на пальце"},
	"arms":      {"arms", "arm", "рука", "руки", "на руках"},
	"shoulders": {"shoulders", "плечо", "плечи", "на плечах"},
	"legs":      {"legs", "нога", "ноги", "на ногах"},
	"wrist":     {"wrist", "запястье", "запястьях", "на запястье"},
	"shield":    {"shield", "щит", "щите"},
	"wield":     {"wield", "wielded", "оружие", "в правой руке", "в левой руке"},
	"held":      {"held", "держит", "в руках"},
}

type PromptTuple struct {
	HPCurrent int `json:"hp_current"`
	HPMax     int `json:"hp_max"`
	MACurrent int `json:"ma_current"`
	MAMax     int `json:"ma_max"`
	MVCurrent int `json:"mv_current"`
	MVMax     int `json:"mv_max"`
	EXP       int `json:"exp"`
}

type Snapshot struct {
	SessionID  string            `json:"-"`
	Location   string            `json:"location,omitempty"`
	Prompt     *PromptTuple      `json:"prompt,omitempty"`
	StatusTags []string          `json:"status_tags,omitempty"`
	Equipment  map[string]string `json:"equipment,omitempty"`
	UpdatedAt  string            `json:"updated_at,omitempty"`
}

type Delta struct {
	Location   string
	Prompt     *PromptTuple
	StatusTags []string
	Equipment  map[string]string
}

func (d Delta) Empty() bool {
	return d.Location == "" && d.Prompt == nil && len(d.StatusTags) == 0 && len(d.Equipment) == 0
}

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(text string) Delta {
	cleaned := normalizeText(text)
	if strings.TrimSpace(cleaned) == "" {
		return Delta{}
	}

	delta := Delta{}
	delta.Prompt = parsePrompt(cleaned)
	delta.StatusTags = parseStatusTags(cleaned)
	delta.Equipment = parseEquipment(cleaned)
	delta.Location = parseLocation(cleaned)
	return delta
}

func normalizeText(text string) string {
	text = ansiRegex.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return text
}

func parsePrompt(text string) *PromptTuple {
	matches := promptRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}
	last := matches[len(matches)-1]
	if len(last) != 8 {
		return nil
	}

	values := make([]int, 0, 7)
	for _, raw := range last[1:] {
		v, err := strconv.Atoi(raw)
		if err != nil {
			return nil
		}
		values = append(values, v)
	}

	return &PromptTuple{
		HPCurrent: values[0],
		HPMax:     values[1],
		MACurrent: values[2],
		MAMax:     values[3],
		MVCurrent: values[4],
		MVMax:     values[5],
		EXP:       values[6],
	}
}

func parseStatusTags(text string) []string {
	matches := tagRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil
	}

	seen := map[string]struct{}{}
	tags := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		normalized := strings.ToLower(strings.TrimSpace(match[1]))
		canonical, ok := knownStatusTags[normalized]
		if !ok {
			continue
		}
		if _, ok := seen[canonical]; ok {
			continue
		}
		seen[canonical] = struct{}{}
		tags = append(tags, canonical)
	}

	sort.Strings(tags)
	if len(tags) == 0 {
		return nil
	}
	return tags
}

func parseEquipment(text string) map[string]string {
	lines := strings.Split(text, "\n")
	equipment := make(map[string]string)

	for _, line := range lines {
		label, item, ok := parseEquipmentLine(line)
		if !ok {
			continue
		}

		slot := normalizeEquipmentSlot(label)
		if slot == "" {
			continue
		}
		if item == "" || strings.EqualFold(item, "none") || strings.EqualFold(item, "nothing") || strings.Contains(strings.ToLower(item), "ничего") {
			continue
		}
		equipment[slot] = item
	}

	if len(equipment) == 0 {
		return nil
	}
	return equipment
}

func parseEquipmentLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", false
	}
	if strings.HasPrefix(line, "#") {
		return "", "", false
	}

	if strings.HasPrefix(line, "[") {
		if idx := strings.Index(line, "]"); idx > 1 {
			label := strings.TrimSpace(line[1:idx])
			item := strings.TrimSpace(line[idx+1:])
			return label, item, true
		}
	}

	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	label := strings.TrimSpace(parts[0])
	item := strings.TrimSpace(parts[1])
	if label == "" || item == "" {
		return "", "", false
	}
	return label, item, true
}

func normalizeEquipmentSlot(label string) string {
	value := strings.ToLower(strings.TrimSpace(label))
	for slot, aliases := range equipmentAliases {
		for _, alias := range aliases {
			if strings.Contains(value, alias) {
				return slot
			}
		}
	}
	return ""
}

func parseLocation(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if promptRegex.MatchString(line) {
			continue
		}
		if strings.HasPrefix(line, "(") && strings.Contains(line, ")") {
			continue
		}
		if strings.Contains(line, ":") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "выход") || strings.Contains(lower, "exits") {
				continue
			}
			continue
		}
		if isLikelyLocation(line) {
			return line
		}
	}
	return ""
}

func isLikelyLocation(line string) bool {
	if len(line) < 3 || len(line) > 120 {
		return false
	}
	if strings.HasSuffix(line, ".") {
		return false
	}
	lower := strings.ToLower(line)
	if strings.Contains(lower, "урон") || strings.Contains(lower, "аукцион") {
		return false
	}
	for _, r := range line {
		if r == '/' || r == '|' {
			return false
		}
	}
	return true
}
