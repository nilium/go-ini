package ini

import (
	"errors"
	"fmt"
)

// SyntaxError is an error returned when the INI parser encounters any syntax it does not
// understand. It contains the line, column, any other error encountered, and a description of the
// syntax error.
type SyntaxError struct {
	Line, Col int
	Err       error
	Desc      string
}

func (s *SyntaxError) Error() string {
	if s.Desc == "" {
		return fmt.Sprintf("ini: syntax error at %d:%d: %v", s.Line, s.Col, s.Err)
	}
	return fmt.Sprintf("ini: syntax error at %d:%d: %v -- %s", s.Line, s.Col, s.Err, s.Desc)
}

// UnclosedError is an error describing an unclosed bracket from {, (, [, and <. It is typically set
// as the Err field of a SyntaxError.
//
// Its value is expected to be one of the above opening braces.
type UnclosedError rune

// Expecting returns the rune that was expected but not found for the UnclosedError's rune value.
func (u UnclosedError) Expecting() rune {
	switch u := rune(u); u {
	case '{':
		return '}'
	case '(':
		return ')'
	case '[':
		return ']'
	case '<':
		return '>'
	default:
		return u
	}
}

func (u UnclosedError) Error() string {
	return fmt.Sprintf("ini: unclosed %c, expecting %c", rune(u), u.Expecting())
}

// BadCharError is an error describing an invalid character encountered during parsing. It is
// typically set as the Err field of a SyntaxError.
type BadCharError rune

func (r BadCharError) Error() string {
	return fmt.Sprintf("ini: encountered invalid character %q", rune(r))
}

var (
	// ErrSectionRawStr is a syntax error seen when a raw string is present in a location that
	// is not valid.
	ErrSectionRawStr = errors.New("ini: raw string not accepted in section")
	// ErrUnclosedSection is a syntax error seen when a section name has not been closed.
	ErrUnclosedSection = errors.New("ini: section missing closing ]")
	// ErrEmptyKey is a syntax error seen if a key is empty.
	ErrEmptyKey = errors.New("ini: key is empty")

	// ErrBadNewline is a BadCharError for unexpected newlines.
	ErrBadNewline = BadCharError('\n')
)
