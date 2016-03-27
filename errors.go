package ini

import (
	"errors"
	"fmt"
)

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

type UnclosedError rune

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

type BadCharError rune

func (r BadCharError) Error() string {
	return fmt.Sprintf("ini: encountered invalid character %q", rune(r))
}

var (
	ErrSectionRawStr   = errors.New("ini: raw string not accepted in section")
	ErrUnclosedSection = errors.New("ini: section missing closing ]")
	ErrEmptyKey        = errors.New("ini: key is empty")

	ErrBadNewline = BadCharError('\n')
)
