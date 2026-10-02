package main

import "testing"

// The filename is what the albums sort by, so it has to sort numbers as numbers.
func TestDiplomaFileName(t *testing.T) {
	for _, c := range []struct{ number, contentType, want string }{
		{"7", "image/jpeg", "007.jpg"},
		{"42", "", "042.jpg"},
		{"186", "image/jpeg", "186.jpg"},
		{"1042", "image/jpeg", "1042.jpg"},
		{" 9 ", "image/webp", "009.webp"},
		{"12A", "image/jpeg", "12A.jpg"},
		{"", "image/jpeg", ""},
	} {
		if got := diplomaFileName(c.number, c.contentType); got != c.want {
			t.Errorf("diplomaFileName(%q, %q) = %q, want %q", c.number, c.contentType, got, c.want)
		}
	}
}
