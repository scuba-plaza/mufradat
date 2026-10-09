package arabic

func IsLetter(r rune) bool {
	switch {
	case r >= 0x0621 && r <= 0x063A:
		return true
	case r >= 0x0641 && r <= 0x064A:
		return true
	case r >= 0x0671 && r <= 0x06D3:
		return true
	}
	return false
}

func HasArabic(s string) bool {
	for _, r := range s {
		if IsLetter(r) {
			return true
		}
	}
	return false
}
