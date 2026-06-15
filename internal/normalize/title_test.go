package normalize

import "testing"

func TestTitle(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Hades™", "hades"},
		{"Hades", "hades"},
		{"HADES", "hades"},
		{"Disco Elysium®", "disco elysium"},
		{"Pathfinder: Wrath of the Righteous", "pathfinder wrath of the righteous"},
		{"A Short Hike", "a short hike"},
		{"Vampire Survivors", "vampire survivors"},
		{"  Spaces  Everywhere  ", "spaces everywhere"},
		{"Half-Life 2", "halflife 2"},
		{"GRIS©", "gris"},
		{"Baldur's Gate 3", "baldurs gate 3"},
		{"", ""},
		{"™®©", ""},
		{"It Takes Two", "it takes two"},
	}

	for _, c := range cases {
		got := Title(c.in)
		if got != c.want {
			t.Errorf("Title(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
