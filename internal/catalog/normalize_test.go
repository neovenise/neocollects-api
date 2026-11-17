package catalog

import "testing"

func TestSearchName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Dracaufeu-ex", "dracaufeu ex"},
		{"DRACAUFEU ex", "dracaufeu ex"},
		{"Ékaïser", "ekaiser"},
		{"Goupix d'Alola V", "goupix d alola v"},
		{"  Mew  ", "mew"},
		{"Florizarre et Vipélierre GX", "florizarre et vipelierre gx"},
		{"", ""},
		{"---", ""},
	}
	for _, tt := range tests {
		if got := SearchName(tt.in); got != tt.want {
			t.Errorf("SearchName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLocalNumber(t *testing.T) {
	tests := []struct {
		in     string
		want   int
		wantOK bool
	}{
		{"125", 125, true},
		{"024", 24, true},
		{"0", 0, true},
		{"TG01", 0, false},
		{"SWSH123", 0, false},
		{"12a", 0, false},
		{"", 0, false},
		{"-5", 0, false},
	}
	for _, tt := range tests {
		got, ok := LocalNumber(tt.in)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("LocalNumber(%q) = %d, %v; want %d, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}
}
