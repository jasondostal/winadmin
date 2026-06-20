package groups

import "testing"

func TestInGroupList(t *testing.T) {
	fetched := []string{`CORP\Domain Admins`, `CORP\Backup Operators`, "Administrators"}

	cases := []struct {
		group string
		want  bool
	}{
		{`CORP\Backup Operators`, true},  // exact
		{"backup operators", true},       // case-insensitive, no domain
		{`OTHER\Backup Operators`, true}, // leaf-name match ignores domain
		{"Administrators", true},         // bare well-known
		{"Domain Users", false},          // not present
		{`CORP\Domain Users`, false},     // not present, with domain
	}
	for _, c := range cases {
		if got := InGroupList(c.group, fetched); got != c.want {
			t.Errorf("InGroupList(%q) = %v, want %v", c.group, got, c.want)
		}
	}
}

func TestInGroupListEmpty(t *testing.T) {
	if InGroupList("Administrators", nil) {
		t.Error("nothing should match an empty list")
	}
}
