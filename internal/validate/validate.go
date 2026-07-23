package validate

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	// US ZIP: 5 digits or ZIP+4
	reZIP   = regexp.MustCompile(`^[0-9]{5}$`)
	reEmail = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$`)
	reQ     = regexp.MustCompile(`^[A-Za-z0-9 _'\\-]{1,50}$`)
	reID    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
	reCond  = regexp.MustCompile(`^(FIRST_HAND|SECOND_HAND)$`)
)

func Region(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || len(s) > 5 {
		return "", false
	}
	return s, reZIP.MatchString(s)
}

func Email(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 0 || len(s) > 50 {
		return "", false
	}
	return s, reEmail.MatchString(s)
}

// Q validates a search query: trims, enforces allowed characters and max length
func Q(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if len(s) > 50 {
		s = s[:50]
	}
	return s, reQ.MatchString(s)
}
func Qty(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n < 1 {
		return 1
	}
	if n > 50 {
		return 50
	} // clamp to avoid abuse
	return n
}

// ID validates a simple resource identifier (product/category ids).
func ID(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && reID.MatchString(s)
}

// Condition validates allowed condition enums.
func Condition(s string) (string, bool) {
	s = strings.TrimSpace(s)
	return s, s != "" && reCond.MatchString(s)
}

// Name validates a displayable name with a reasonable max length.
func Name(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 20 {
		return "", false
	}
	return s, true
}

// Password enforces a simple length window for login checks.
func Password(s string) bool {
	l := len(s)
	if l < 8 || l > 20 {
		return false
	}
	var hasLower, hasUpper, hasDigit, hasSymbol bool
	for _, r := range s {
		switch {
		case 'a' <= r && r <= 'z':
			hasLower = true
		case 'A' <= r && r <= 'Z':
			hasUpper = true
		case '0' <= r && r <= '9':
			hasDigit = true
		default:
			hasSymbol = true
		}
	}
	return hasLower && hasUpper && hasDigit && hasSymbol
}
