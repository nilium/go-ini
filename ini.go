// Package ini is an INI parsing library.
package ini // import "go.spiff.io/go-ini"

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	// Sections

	rSectionOpen  = '['
	rSectionClose = ']'

	// Quoted values

	rQuote    = '"'
	rRawQuote = '`'

	// Whitespace

	rSpace   = ' '
	rTab     = '\t'
	rNewline = '\n'
	rCR      = '\r' // Ignored outside of strings.

	// Comment

	rSemicolon = ';'
	rHash      = '#'

	// Values

	rEquals = '='
	rEscape = '\\'
)

// Values is any set of INI values. This may be used as a Recorder for a Reader.
type Values map[string][]string

// Set replaces key with a slice containing only value.
func (v Values) Set(key, value string) {
	v[key] = []string{value}
}

// Add adds a value to key's value slice (allocating one if none is present).
func (v Values) Add(key, value string) {
	v[key] = append(v[key], value)
}

// Get returns the first value for key. If key does not exist or has an empty value slice, Get
// returns an empty string.
func (v Values) Get(key string) string {
	if d := v[key]; len(d) > 0 {
		return d[0]
	}
	return ""
}

// Del removes the key from the receiver.
func (v Values) Del(key string) {
	delete(v, key)
}

// Contains returns true if the key is defined in the Values map. This is an existence check, not
// a content check, so the key may be for a nil or empty slice.
func (v Values) Contains(key string) bool {
	_, ok := v[key]
	return ok
}

// Copy copies the values from the receiver to dst. If a key from the receiver exists in dst, the
// values from the receiver are appended to the values of dst[key].
func (v Values) Copy(dst Values) Values {
	if dst == nil {
		dst = make(Values, len(v))
	}
	for k, vs := range v {
		dst[k] = append(dst[k], vs...)
	}
	return dst
}

// Matching iterates through the receiver's values, calling fn for each key and its values. If fn
// returns true, that key's values are returned in the resulting Values. fn must not modify the
// values slice it receives.
func (v Values) Matching(dst Values, fn func(key string, values []string) bool) Values {
	if dst == nil {
		dst = make(Values)
	}
	for k, vs := range v {
		if fn(k, vs) {
			dst[k] = append(dst[k], vs...)
		}
	}
	return dst
}

// WithPrefix returns a copy of Values containing only the keys with the given prefix string.
func (v Values) WithPrefix(dst Values, prefix string) Values {
	return v.Matching(dst, func(k string, _ []string) bool {
		return strings.HasPrefix(k, prefix)
	})
}

// nextfunc is a parsing function that modifies the decoder's state and returns another parsing
// function. If nextfunc returns io.EOF, parsing is complete. Any other error halts parsing.
type nextfunc func() (nextfunc, error)

// decoder is a wrapper around an io.Reader for the purpose of doing by-rune parsing of INI file
// input. It also holds enough state to track line, column, key prefixes (from sections), and
// errors.
type decoder struct {
	true string

	rd       io.Reader
	readrune func() (rune, int, error)

	err    error
	sep    []byte
	sep2   [4]byte
	dst    Recorder
	casefn func(rune) rune

	current   rune
	line, col int

	// Storage
	buffer  bytes.Buffer
	key     string
	prefix  []byte   // prefix is prepended to all buffered keys
	prefix2 [32]byte // prefix2 is a buffer to hold most key prefixes

	// peek / next state
	havenext bool
	next     rune
	nexterr  error
}

// True is the default value provided to value-less keys in INI files. This is done to treat
// value-less keys as boolean-on flags in INI files.
const True string = "1"

// ReadINI reads an INI file from b, writing the results to the out Values. If out is nil, a new
// Values is allocated to store the results.
//
// ReadINI is a convenience function for calling DefaultDecoder.Read(bytes.NewReader(b), out).
func ReadINI(b []byte, out Values) (Values, error) {
	if out == nil {
		out = make(Values)
	}
	err := DefaultDecoder.Read(bytes.NewReader(b), out)
	if err != nil {
		return nil, err
	}
	return out, err
}

func (d *decoder) add(key, value string) {
	if d.dst != nil {
		d.dst.Add(key, value)
	}
}

func (d *decoder) syntaxerr(err error, msg ...interface{}) *SyntaxError {
	if se, ok := err.(*SyntaxError); ok {
		return se
	}
	se := &SyntaxError{Line: d.line, Col: d.col, Err: err, Desc: fmt.Sprint(msg...)}
	return se
}

func (d *decoder) nextRune() (r rune, size int, err error) {
	if d.err != nil {
		return d.current, utf8.RuneLen(d.current), d.err
	}

	if d.havenext {
		r, size, err = d.peekRune()
		d.havenext = false
	} else if d.readrune != nil {
		r, size, err = d.readrune()
	} else {
		r, size, err = readrune(d.rd)
	}

	d.current = r

	if err != nil {
		d.err = err
		d.rd = nil
	}

	if d.current == '\n' {
		d.line++
		d.col = 1
	}

	return r, size, err
}

func (d *decoder) skip() error {
	_, _, err := d.nextRune()
	return err
}

func (d *decoder) peekRune() (r rune, size int, err error) {
	if d.havenext {
		r = d.next
		size = utf8.RuneLen(r)
		return r, size, d.nexterr
	}

	// Even if there's an error.
	d.havenext = true
	if d.readrune != nil {
		r, size, err = d.readrune()
	} else {
		r, size, err = readrune(d.rd)
	}
	d.next, d.nexterr = r, err
	return r, size, err
}

func (d *decoder) readUntil(oneof runeset, buffer bool, runemap func(rune) rune) (err error) {
	for out := &d.buffer; ; {
		var r rune
		r, _, err = d.nextRune()
		if err != nil {
			return err
		} else if oneof.Contains(r) {
			return nil
		} else if buffer {
			if runemap != nil {
				r = runemap(r)
			}
			if r >= 0 {
				out.WriteRune(r)
			}
		}
	}
}

func escaped(r rune) rune {
	switch r {
	case '0':
		return 0
	case 'a':
		return '\a'
	case 'b':
		return '\b'
	case 'f':
		return '\f'
	case 'n':
		return '\n'
	case 'r':
		return '\r'
	case 't':
		return '\t'
	case 'v':
		return '\v'
	default:
		return r
	}
}

func (d *decoder) readComment() (next nextfunc, err error) {
	defer stopOnEOF(&next, &err)
	next, err = d.readElem, d.readUntil(oneRune(rNewline), true, nil)
	return
}

func isHorizSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\r' }

func (d *decoder) skipSpace(newlines bool) error {
	fn := unicode.IsSpace
	if !newlines {
		fn = isHorizSpace
	}

	if fn(d.current) {
		return d.readUntil(notRune(runeFunc(fn)), false, nil)
	}
	return nil
}

func isKeyEnd(r rune) bool {
	return r == rEquals || r == rHash || r == rSemicolon || unicode.IsSpace(r)
}

func casenop(r rune) rune { return r }

func (d *decoder) readKey() (nextfunc, error) {
	casefn := d.casefn
	d.buffer.Write(d.prefix)
	switch d.current {
	case rEquals:
		return nil, d.syntaxerr(ErrEmptyKey, "keys may not be blank")
	case rQuote, rRawQuote:
		return nil, d.syntaxerr(BadCharError(d.current), "keys may not be quoted strings")
	default:
		r := d.current
		if casefn != nil {
			r = casefn(r)
		}
		d.buffer.WriteRune(r)
	}

	err := d.readUntil(runeFunc(isKeyEnd), true, casefn)
	if err != nil && err != io.EOF {
		return nil, err
	}

	if err == io.EOF {
		d.add(d.buffer.String(), d.true)
		return nil, nil
	}

	d.key = d.buffer.String()
	d.buffer.Reset()

	return d.readValueSep, nil
}

func (d *decoder) readValueSep() (next nextfunc, err error) {
	if err = must(d.skipSpace(false), io.EOF, nil); err == io.EOF {
		d.add(d.key, d.true)
		return nil, nil
	}

	defer stopOnEOF(&next, &err)
	// Aside from whitespace, the only thing that can follow a key is a newline or =.
	switch d.current {
	case rNewline:
		d.add(d.key, d.true)
		return d.readElem, d.skip()
	case rEquals:
		if err = d.skip(); err == io.EOF {
			d.add(d.key, "")
			return nil, nil
		}
		return d.readValue, nil
	case rHash, rSemicolon:
		d.add(d.key, d.true)
		return d.readComment, nil
	default:
		return nil, d.syntaxerr(BadCharError(d.current), "expected either =, newline, or a comment")
	}
}

func (d *decoder) readHexCode(size int) (result rune, err error) {
	for i := 0; i < size; i++ {
		r, sz, err := d.nextRune()
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return -1, d.syntaxerr(err, "expected hex code")
		} else if sz != 1 {
			// Quick size check
			return -1, d.syntaxerr(BadCharError(r), "expected hex code")
		}

		if r >= 'A' && r <= 'F' {
			r = 10 + (r - 'A')
		} else if r >= 'a' && r <= 'f' {
			r = 10 + (r - 'a')
		} else if r >= '0' && r <= '9' {
			r -= '0'
		} else {
			return -1, d.syntaxerr(BadCharError(r), "expected hex code")
		}
		result = result<<4 | r
	}
	return result, nil
}

func (d *decoder) readStringValue() (next nextfunc, err error) {
	err = d.readUntil(runestr(`"\`), true, nil)
	if err == io.EOF {
		return nil, d.syntaxerr(UnclosedError('"'), "encountered EOF inside string")
	} else if err != nil {
		return nil, err
	}

	switch d.current {
	case '"':
		if r, _, perr := d.peekRune(); perr == nil && r == rQuote {
			d.buffer.WriteRune(r)
			return d.readStringValue, d.skip()
		}
	case '\\':
		r, _, err := d.nextRune()
		must(err)
		switch r {
		case 'x': // 1 octet
			r, err = d.readHexCode(2)
			d.buffer.WriteByte(byte(r & 0xFF))
		case 'u': // 2 octets
			r, err = d.readHexCode(4)
			d.buffer.WriteRune(r)
		case 'U': // 4 octets
			r, err = d.readHexCode(8)
			d.buffer.WriteRune(r)
		default:
			r = escaped(r)
			d.buffer.WriteRune(escaped(r))
		}
		return d.readStringValue, err
	}

	defer stopOnEOF(&next, &err)
	d.add(d.key, d.buffer.String())
	return d.readElem, d.skip()
}

func (d *decoder) readRawValue() (next nextfunc, err error) {
	err = d.readUntil(oneRune(rRawQuote), true, nil)
	if err == io.EOF {
		return nil, d.syntaxerr(UnclosedError('`'), "encountered EOF inside raw string")
	} else if err != nil {
		return nil, err
	}

	if r, _, perr := d.peekRune(); perr == nil && r == rRawQuote {
		d.buffer.WriteRune(r)
		return d.readRawValue, d.skip()
	}

	defer stopOnEOF(&next, &err)
	d.add(d.key, d.buffer.String())
	return d.readElem, d.skip()
}

func (d *decoder) readValue() (next nextfunc, err error) {
	if err = must(d.skipSpace(false), io.EOF); err == io.EOF {
		d.add(d.key, "")
		return nil, nil
	}

	switch d.current {
	case rNewline:
		// Terminated by newline
		defer stopOnEOF(&next, &err)
		d.add(d.key, "")
		return d.readElem, d.skip()
	case rQuote:
		return d.readStringValue, nil
	case rRawQuote:
		return d.readRawValue, nil
	case rHash, rSemicolon:
		// Terminated by comment
		d.add(d.key, "")
		return d.readComment, nil
	}

	defer stopOnEOF(&next, &err)
	d.buffer.WriteRune(d.current)
	must(d.readUntil(runestr("\n;#"), true, nil), io.EOF)

	value := string(bytes.TrimRightFunc(d.buffer.Bytes(), unicode.IsSpace))
	d.add(d.key, value)
	return d.readElem, err
}

func (d *decoder) readQuotedSubsection() (next nextfunc, err error) {
	if must(d.readUntil(runestr(`"\`), true, nil), io.EOF) == io.EOF {
		return nil, d.syntaxerr(UnclosedError('"'), "encountered EOF inside quoted section name")
	}

	switch d.current {
	case rQuote:
		var r rune
		if r, _, err = d.peekRune(); err == nil && r == rQuote {
			d.buffer.WriteRune(r)
			return d.readQuotedSubsection, d.skip()
		}

		return d.readSubsection, d.skip()
	case rEscape:
		r, _, err := d.nextRune()
		must(err)
		switch r {
		case 'x': // 1 octet
			r, err = d.readHexCode(2)
			d.buffer.WriteByte(byte(r & 0xFF))
		case 'u': // 2 octets
			r, err = d.readHexCode(4)
			d.buffer.WriteRune(r)
		case 'U': // 4 octets
			r, err = d.readHexCode(8)
			d.buffer.WriteRune(r)
		default:
			r = escaped(r)
			d.buffer.WriteRune(escaped(r))
		}
		return d.readQuotedSubsection, nil
	}
	return nil, d.syntaxerr(BadCharError(d.current), "expected a closing quote or escape character")
}

func (d *decoder) readHeaderOpen() (nextfunc, error) {
	if d.current != rSectionOpen {
		// This should be more or less impossible, based on how it's called.
		return nil, d.syntaxerr(BadCharError(d.current), "expected an opening bracket ('[')")
	}
	return d.readSubsection, d.skip()
}

func (d *decoder) addPrefixSep() {
	sep := d.sep
	if d.buffer.Len() < len(sep) || bytes.HasSuffix(d.buffer.Bytes(), sep) {
		return
	}
	d.buffer.Write(sep)
}

func (d *decoder) readSubsection() (next nextfunc, err error) {
	d.addPrefixSep()

	switch d.current {
	case rSectionClose:
		if d.buffer.Len() == 0 {
			d.prefix = d.prefix[:0]
		} else {
			d.prefix = append(d.prefix[:0], d.buffer.Bytes()...)
		}
		defer stopOnEOF(&next, &err)
		return d.readElem, d.skip()
	case rRawQuote:
		return nil, d.syntaxerr(ErrSectionRawStr, "raw strings are not allowed in section names")
	case rQuote:
		return d.readQuotedSubsection, nil
	case rSpace, rTab:
		return d.readSubsection, d.skipSpace(false)
	case rNewline:
		return nil, d.syntaxerr(ErrBadNewline, "section headings may not contain unquoted newlines")
	default:
		if unicode.IsSpace(d.current) {
			return nil, d.syntaxerr(BadCharError(d.current), "expected section name")
		}
	}

	// Buffer initial
	r := d.current
	casefn := d.casefn
	if casefn != nil {
		r = casefn(r)
	}
	d.buffer.WriteRune(r)

	return d.readSubsection, d.readUntil(runestr(" \t\n\"]"), true, casefn)
}

func (d *decoder) start() (next nextfunc, err error) {
	_, _, err = d.nextRune()
	if err == io.EOF {
		return nil, nil
	}
	return d.readElem, err
}

func (d *decoder) readElem() (next nextfunc, err error) {
	d.buffer.Reset()

	if d.err == io.EOF {
		return nil, nil
	} else if d.err != nil {
		return nil, err
	}

	switch d.current {
	case rSectionOpen:
		return d.readHeaderOpen()
	case rHash, rSemicolon:
		return d.readComment()
	case ' ', '\t', '\n', '\f', '\r', 0x85, 0xA0:
		if err = d.skipSpace(true); err == io.EOF {
			return nil, nil
		}
		return d.readElem, err
	default:
		return d.readKey()
	}
}

var defaultSeparator = []byte{'.'}

// None is a value to force a Reader.True to indicate that empty keys should have no value or for
// Reader.Separator to indicate there should be no separator string between section keys and value
// keys.
//
// None is a specific sequence of garbage control characters, just due to it being unlikely that you
// would want it as a true or separator value. This is not guaranteed to be the same value between
// versions of the package.
const None = "\x00\x00\x13\x15\xff\x00\x12\x00\x13"

func (d *decoder) reset(cfg *Reader, dst Recorder, rd io.Reader) {
	const defaultBufferCap = 64

	if cfg == nil {
		cfg = &DefaultDecoder
	}

	if rx, ok := rd.(runeReader); ok {
		d.readrune = rx.ReadRune
	} else {
		d.readrune = nil
	}

	switch cfg.Casing {
	case UpperCase:
		d.casefn = unicode.ToUpper
	case LowerCase:
		d.casefn = unicode.ToLower
	default:
		d.casefn = nil
	}

	d.rd = rd
	d.err = nil
	d.dst = dst

	d.current = 0
	d.line = 1
	d.col = 0

	if cfg.True == None {
		d.true = ""
	} else if cfg.True != "" {
		d.true = cfg.True
	} else {
		d.true = True
	}

	if cfg.Separator == None {
		d.sep = nil
	} else if cfg.Separator != "" {
		d.sep = append(d.sep2[:0], cfg.Separator...)
	} else {
		d.sep = d.sep2[:1]
		d.sep[0] = '.'
	}

	d.buffer.Reset()
	d.buffer.Grow(defaultBufferCap)

	d.key = ""
	if d.prefix == nil {
		d.prefix = d.prefix2[:0]
	}

	d.havenext = false
	d.nexterr = nil
}

func (d *decoder) read() (err error) {
	defer panictoerr(&err)
	var next nextfunc = d.start
	for next != nil && err == nil {
		next, err = next()
	}
	return err
}

// KeyCase is an option value to change how unquoted keys are handled. For example, to lowercase all
// unquoted portions of a key, you would use LowerCase (the default of new Readers and zero value).
// This allows for basic case normalization across files.
type KeyCase int

const (
	// LowerCase indicates that you want all unquoted subsections lower-cased. This is the
	// default key casing.
	LowerCase KeyCase = iota
	// UpperCase indicates that you want all unquoted subsections upper-cased.
	UpperCase
	// CaseSensitive indicates that you want all unquoted subsections left as-is.
	CaseSensitive
)

// DefaultDecoder is the default Reader. Its separator is a "." (period), its True value is the
// string "1", and keys are case-sensitive.
var DefaultDecoder = Reader{
	Separator: ".",
	Casing:    CaseSensitive,
	True:      True,
}

// Recorder is any type that can accept INI values. Multiple calls to Add may occur with the same
// key. It is up to the Recorder to decide if it discards or appends to prior versions of a key. If
// Add panics, the value that it panics with is returned as an error.
type Recorder interface {
	Add(key, value string)
}

// Reader is an INI reader configuration. It does not hold state and may be copied as needed.
// It is not safe to modify a Reader while Reading, however, as the internal decoder keeps a pointer
// to the Reader.
type Reader struct {
	// Separator is the string that is inserted between key segments (i.e., given a Separator of
	// ":", the string "[a b c]\nd = 5" evaluates out to a:b:c:d = 5). If Separator is None, not
	// the empty string, there is no separator. If Separator is the empty string, it defaults to
	// "." (period).
	Separator string
	// Casing controls how unquoted key segments are cased. If LowerCase (the default / zero
	// value), unquoted key segments are converted to lowercase. If UpperCase, they're made
	// uppercase. If CaseSensitive, key case is the same as the input.
	Casing KeyCase
	// True is the value string used for keys with no value. For example, if True is "T"
	// (assuming default Separator), given the input "[a b c]\nd", it evaluates to a.b.c.d = T.
	True string
}

// Read decodes INI file input from r and conveys it to dst. If an error occurs, it is returned. If
// the error is an EOF before parsing is finished, io.ErrUnexpectedEOF is returned.
func (d *Reader) Read(r io.Reader, dst Recorder) error {
	var dec decoder
	dec.reset(d, dst, r)
	return dec.read()
}

// Utility functions

func panictoerr(err *error) {
	rc := recover()
	if perr, ok := rc.(error); ok {
		*err = perr
	} else if rc != nil {
		*err = fmt.Errorf("ini: panic: %v", rc)
	}

	if *err == io.EOF {
		*err = io.ErrUnexpectedEOF
	}
}

func must(err error, allowed ...error) error {
	if err == nil {
		return err
	}

	for _, e := range allowed {
		if e == err {
			return err
		}
	}

	panic(err)
}

func stopOnEOF(next *nextfunc, err *error) {
	if *err == io.EOF {
		*next = nil
		*err = nil
	}
}

// Rune handling

type runeReader interface {
	ReadRune() (rune, int, error)
}

func readrune(rd io.Reader) (r rune, size int, err error) {
	if rd, ok := rd.(runeReader); ok {
		return rd.ReadRune()
	}
	var b [4]byte
	for i, t := 0, 1; i < len(b); i, t = i+1, t+1 {
		_, err = rd.Read(b[i:t])
		if err != nil {
			return r, size, err
		} else if c := b[:t]; utf8.FullRune(c) {
			r, size = utf8.DecodeRune(c)
			return r, size, err
		}
	}

	return unicode.ReplacementChar, 1, nil
}

type (
	runeset interface {
		Contains(rune) bool
	}

	oneRune  rune
	runeFunc func(rune) bool
	runestr  string
)

func notRune(runes runeset) runeset {
	return runeFunc(func(r rune) bool { return !runes.Contains(r) })
}

func (s runestr) Contains(r rune) bool { return strings.ContainsRune(string(s), r) }

func (fn runeFunc) Contains(r rune) bool { return fn(r) }

func (lhs oneRune) Contains(rhs rune) bool { return rune(lhs) == rhs }
