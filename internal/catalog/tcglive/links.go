package tcglive

import "fmt"

var ExportCodes = map[string]string{
	"SVP": "PR-SV",
}

var ExportSets = map[string]string{
	"30th": "me5-5",
}

type GameSet struct {
	Code   string
	Offset int
}

var SwshSets = map[string]GameSet{
	"swsh1": {Code: "swsh1"}, "swsh2": {Code: "swsh2"}, "swsh3": {Code: "swsh3"}, "swsh3.5": {Code: "swsh3-5"},
	"swsh4": {Code: "swsh4"}, "swsh4.5": {Code: "swsh4-5"}, "swsh4.5sv": {Code: "swsh4-5", Offset: 73},
	"swsh5": {Code: "swsh5"}, "swsh6": {Code: "swsh6"}, "swsh7": {Code: "swsh7"}, "swsh8": {Code: "swsh8"},
	"swsh9": {Code: "swsh9"}, "swsh9tg": {Code: "swsh9a"},
	"swsh10": {Code: "swsh10"}, "swsh10tg": {Code: "swsh10a"}, "swsh10.5": {Code: "swsh10-5"},
	"swsh11": {Code: "swsh11"}, "swsh11tg": {Code: "swsh11a"},
	"swsh12": {Code: "swsh12"}, "swsh12tg": {Code: "swsh12a"},
	"swsh12.5": {Code: "swsh12-5"}, "swsh12.5gg": {Code: "swsh12-5a"},
	"cel25": {Code: "swsh7-5"}, "cel25cc": {Code: "swsh7-5r"},
	"swshp": {Code: "swshbsp"},
}

// GameNumber returns the card's three-digit number in the game.
func GameNumber(localID string, offset int) (string, bool) {
	end := len(localID)
	start := end
	for start > 0 && localID[start-1] >= '0' && localID[start-1] <= '9' {
		start--
	}
	if start == end {
		return "", false
	}
	n := 0
	for _, d := range localID[start:end] {
		n = n*10 + int(d-'0')
	}
	return fmt.Sprintf("%03d", n+offset), true
}

var Languages = []string{"fr", "en"}
