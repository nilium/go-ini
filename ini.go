package ini

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	chPrefixBegin = byte('[')
	chPrefixEnd   = byte(']')
	chSpace       = byte(' ')
	chQuote       = byte('"')
	chTab         = byte('\t')
	chLine        = byte('\n')
	chFeed        = byte('\r') // ignored outside of strings.
	chComment     = byte(';')
	chEquals      = byte('=')
	chEscape      = byte('\\')
)

const (
	valueMarkers         = "\n" + string(chComment) + string(chEquals)
	horizontalWhitespace = " \t"
	whitespaceSansLine   = " \t\r"
	anyWhitespace        = " \t\r\n"
	commentNewline       = "\n" + string(chComment)
)

// True is the value set for value-less keys in an INI.
const True string = "1"

// defaultINICapacity is the default capacity used when allocating a new INI
// map[string]string. Overriding this is as simple as allocating your own map
// and passing it to ReadINI.
const defaultINICapacity int = 32

// PrefixSeparator is the default separator character used for key-value pairs
// beneath section headings (e.g., "[buzzsaw]").
var PrefixSeparator string = "."

type iniParser struct {
	rb     []byte            // slice of rbfix for reading
	quoted [][]byte          // slice of quoted string parts
	result map[string]string // Resulting string map
	prefix string
}

func (p *iniParser) put(k, v string) {
	if p.result == nil {
		return
	}

	if len(p.prefix) > 0 {
		k = p.prefix + k
	}
	p.result[k] = v
}

func advance(b []byte, from int, skip string) []byte {
	skipIdx := 0
	skipLen := len(skip)
	bLen := len(b)
trySkipAgain:
	for skipIdx = 0; skipIdx < skipLen && from < bLen; skipIdx++ {
		if b[from] == skip[skipIdx] {
			from++
			goto trySkipAgain
		}
	}

	if from == 0 {
		return b
	}

	return b[from:]
}

func (p *iniParser) readPrefix() error {
	p.rb = advance(p.rb, 0, anyWhitespace)

	if len(p.rb) == 0 {
		return nil
	}

	if p.rb[0] != chPrefixBegin {
		return p.readComment()
	}

	p.rb = advance(p.rb, 1, horizontalWhitespace)
	end := bytes.IndexByte(p.rb, chPrefixEnd)
	if end == -1 {
		return fmt.Errorf("No closing ']' found for prefix")
	}

	prefix := bytes.Trim(p.rb[:end], whitespaceSansLine)
	p.rb = p.rb[end+1:]
	prefixStr := string(prefix)
	if strings.ContainsAny(prefixStr, "\n") {
		return fmt.Errorf("Prefixes may not contain newlines (%q)", prefixStr)
	}
	if len(prefixStr) > 0 {
		p.prefix = prefixStr + PrefixSeparator
	} else {
		p.prefix = ""
	}

	return nil
}

func (p *iniParser) readComment() error {
	if p.rb[0] != chComment {
		return p.readKey()
	}

	if eol := bytes.IndexByte(p.rb, chLine); eol == -1 {
		p.rb = nil
	} else {
		p.rb = p.rb[eol+1:]
	}

	return nil
}

func (p *iniParser) readKey() error {
	var keyBytes []byte
	var ch byte

	eqIdx := bytes.IndexAny(p.rb, valueMarkers)
	if eqIdx == -1 {
		keyBytes = p.rb
		p.rb = nil
	} else {
		keyBytes = p.rb[:eqIdx]
		ch = p.rb[eqIdx]

		if ch == chEquals {
			// swallow the '='
			p.rb = p.rb[eqIdx+1:]
		} else {
			p.rb = p.rb[eqIdx:]
		}
	}

	keyBytes = bytes.TrimRight(keyBytes, whitespaceSansLine)
	key := string(keyBytes)

	for _, r := range key {
		if r != '-' && r != '_' && !unicode.IsSymbol(r) && !unicode.IsLetter(r) && !unicode.IsNumber(r) && !unicode.IsMark(r) {
			return fmt.Errorf("Keys may only contain letters, numbers, marks, symbols, hyphens, and underscores; %q is not a valid character.", r)
		}
	}

	if eqIdx == -1 {
		p.put(key, True)
		return nil
	}

	var err error
	var value string
	switch ch {
	case chEquals:
		value, err = p.readValue()
	case chComment:
		fallthrough
	case chLine:
		value = True
	}

	if err != nil {
		return err
	}

	p.put(key, string(value))
	return nil
}

// isValueBegin is a function intended to be used as a callback for
// bytes.IndexFunc when searching for the start of a value in a byte sequence.
// Effectively, this just means searching for the first non-whitespace rune.
// If a newline or comment is encountered, these are treated as empty values.
func isValueBegin(r rune) bool {
	return !(r == '\t' || r == ' ' || r == '\r')
}

// readValue reads the value side of a key-value pair. For quotes, this
// descends into readQuote.
func (p *iniParser) readValue() (string, error) {
	if len(p.rb) == 0 {
		return "", nil
	}

	idx := bytes.IndexFunc(p.rb, isValueBegin)

	if idx == -1 {
		return string(bytes.Trim(p.rb, anyWhitespace)), nil
	}

	switch p.rb[idx] {
	case chQuote:
		p.rb = p.rb[idx+1:]
		return p.readQuote()
	case chComment:
		fallthrough
	case chLine:
		return ``, nil
		// value := bytes.Trim(p.rb[:idx], horizontalWhitespace)
		// p.rb = p.rb[idx:]
		// return string(value), nil
	default:
		end := bytes.IndexAny(p.rb, commentNewline)
		if end == -1 {
			value := string(bytes.Trim(p.rb[idx:], anyWhitespace))
			p.rb = nil
			return value, nil
		} else {
			value := bytes.Trim(p.rb[idx:end], horizontalWhitespace)
			p.rb = p.rb[end:]
			return string(value), nil
		}
	}
}

// Escape codes.
// Any not listed here escape to the literal character escaped.
var (
	escNUL       = []byte{0}          // NUL
	escBell      = []byte{byte('\a')} // Bell
	escBackspace = []byte{byte('\b')} // Backspace
	escFeed      = []byte{byte('\f')} // Form feed
	escNewline   = []byte{byte('\n')} // Newline
	escCR        = []byte{byte('\r')} // Carriage return
	escHTab      = []byte{byte('\t')} // Horizontal tab
	escVTab      = []byte{byte('\v')} // Vertical tab
	escSlash     = []byte{byte('\\')} // Backslash
	escDQuote    = []byte{byte('"')}  // Double quote
)

func (p *iniParser) readQuote() (string, error) {

	var (
		parts = p.quoted
		idx   int
		ch    byte
	)
	for ch != byte('"') {
		idx = bytes.IndexAny(p.rb, `\"`)
		if idx == -1 {
			return ``, io.ErrUnexpectedEOF
		}
		ch = p.rb[idx]

		if ch == byte('\\') && len(p.rb) > idx+1 {
			var escape []byte
			switch p.rb[idx+1] {
			case byte('0'):
				escape = escNUL
			case byte('a'):
				escape = escBell
			case byte('b'):
				escape = escBackspace
			case byte('f'):
				escape = escFeed
			case byte('n'):
				escape = escNewline
			case byte('r'):
				escape = escCR
			case byte('t'):
				escape = escHTab
			case byte('v'):
				escape = escVTab
			case byte('\\'):
				escape = escSlash
			case byte('"'):
				escape = escDQuote
			default:
				escape = p.rb[idx+1 : idx+2]
			}
			parts = append(parts, p.rb[:idx], escape)
			idx += 1
		} else if ch == byte('\\') {
			return ``, io.ErrUnexpectedEOF
		} else if ch == byte('"') && idx != 0 {
			parts = append(parts, p.rb[:idx])
		}
		p.rb = p.rb[idx+1:]
	}
	p.quoted = parts[:0]

	switch len(parts) {
	case 0:
		return ``, nil
	case 1:
		return string(parts[0]), nil
	default:
		return string(bytes.Join(parts, nil)), nil
	}
}

// ReadINI accepts a slice of bytes containing a string of bytes ostensibly
// parse-able as an INI and returns a map of strings to strings.
//
// Section names, such as "[buzzsaw]", are treated as prefixes to keys that
// follow them. So, for example, a "key = value" pair following a buzzsaw
// section would be recorded in the resulting map as
// map[string]string{"buzzsaw.key":"value"}.
//
// Value-less keys are treated as boolean flags and will be set to "1".
//
// Values enclosed in double quotes can contain newlines and escape characters
// supported by Go (\a, \b, \f, \n, \r, \t, \v, as well as escaped quotes and
// backslashes and \0 for the NUL character).
func ReadINI(b []byte, out map[string]string) (map[string]string, error) {
	var l int = len(b)
	if l == 0 {
		return out, nil
	}

	if out == nil {
		out = make(map[string]string, defaultINICapacity)
	}

	var p iniParser = iniParser{
		rb:     b,
		quoted: make([][]byte, 0, 4),
		result: out,
	}
	var err error
	for len(p.rb) > 0 {
		err = p.readPrefix()

		if err != nil {
			return nil, err
		}

		if len(p.rb) == l {
			return nil, errors.New("Read could not advance")
		}
		l = len(p.rb)
	}

	return out, nil
}
