package ini

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"
)

var succReaders = map[string]func(string) io.Reader{
	"bytes.Buffer":         func(s string) io.Reader { return bytes.NewBufferString(s) },
	"strings.Reader":       func(s string) io.Reader { return strings.NewReader(s) },
	"iotest.OneByteReader": func(s string) io.Reader { return iotest.OneByteReader(strings.NewReader(s)) },
}

var failReaders = map[string]func(string) io.Reader{
	"bytes.Buffer":         func(s string) io.Reader { return bytes.NewBufferString(s) },
	"strings.Reader":       func(s string) io.Reader { return strings.NewReader(s) },
	"iotest.OneByteReader": func(s string) io.Reader { return iotest.OneByteReader(strings.NewReader(s)) },
	"iotest.DataErrReader": func(s string) io.Reader { return iotest.DataErrReader(strings.NewReader(s)) },
	"iotest.HalfReader":    func(s string) io.Reader { return iotest.HalfReader(strings.NewReader(s)) },
	"iotest.TimeoutReader": func(s string) io.Reader { return iotest.TimeoutReader(strings.NewReader(s)) },
}

func TestPanicToErr_nonerr(t *testing.T) {
	var err error
	func() {
		defer panictoerr(&err)
		panic("foobar!")
	}()

	if want := "ini: panic: foobar!"; err == nil || err.Error() != want {
		t.Errorf("err(%v) = %v; want %q", err, want)
	}
}

func TestReadINI_altsep(t *testing.T) {
	dec := Reader{
		Separator: "_-_",
		Casing:    UpperCase,
	}
	testReadINIMatching(t, &dec, "[section name] abc = 1234", Values{"SECTION_-_NAME_-_ABC": []string{"1234"}})
}

func TestReadINI_keyless(t *testing.T) {
	testReadINIError(t, "[section name] \n= ; \n")
	testReadINIMatching(t, nil, "[section name] abc", Values{"section.name.abc": []string{True}})
}

func TestReadINI_section_badspace(t *testing.T) {
	testReadINIError(t, "[\f\r\n] \n= ; \n") // expected section name
}

func TestReadINIEmpty(t *testing.T) {
	testReadINIMatching(t, nil, "\n\t\n;empty\n\t\n\t", Values{})
}

func TestReadINI_rawval(t *testing.T) {
	// at start
	testReadINIMatching(t, nil, "k = ```raw`` value`", Values{"k": []string{"`raw` value"}})
	// single quote
	testReadINIMatching(t, nil, "k = ````", Values{"k": []string{"`"}})
	// middle
	testReadINIMatching(t, nil, "k = `raw ``value`` surrounded`", Values{"k": []string{"raw `value` surrounded"}})
	// at end
	testReadINIMatching(t, nil, "k = `raw ``value```", Values{"k": []string{"raw `value`"}})

	// Unclosed raw
	testReadINIError(t, "k = `with raw quote")
	// Unclosed regular
	testReadINIError(t, `k = "with regular quote`)
}

func TestReadINISectionSpaces(t *testing.T) {

	// Empty section
	testReadINIMatching(t, nil, "\n[ ]\nk = v\n", Values{`k`: []string{`v`}})

	// Good
	expected := Values{`section.OK.k`: []string{True}}
	testReadINIMatching(t, &Reader{Casing: LowerCase}, `
		[section "OK"]
		K
		`,
		expected)

	expected = Values{`section.ok.k`: []string{`v`}}
	testReadINIMatching(t, &Reader{Casing: LowerCase}, `
		[Section OK]
		K = v
		`,
		expected)

	// Errors
	testReadINIError(t, "\n[newline section\n]\nk = v\n")
	testReadINIError(t, "\n[\nnewline section]\nk = v\n")
	testReadINIError(t, "\n[\nnewline\nsection]\nk = v\n")
}

func TestReadQuotedMulti(t *testing.T) {
	src := `
	[foo HTTP://GIT.SPIFF.IO.FOO ]
		insteadOf = ` + "`left`" + ` ; comment
		insteadOf = center;
		insteadOf = "right"; comment
	[foo "HTTP:\\GIT.SPIFF.IO\x00\u00AB\U00007fff"]
		insteadOf = ` + "`left`" + ` ; comment
		insteadOf = center;
		insteadOf = "right"; comment
	[foo """HTTP://GIT.SPIFF.IO"""]
		insteadOf = ` + "`left`" + ` # comment
		insteadOf = center#
		insteadOf = "right"# comment
	`
	expected := Values{
		`foo.HTTP://GIT.SPIFF.IO.FOO.insteadOf`:                 []string{"left", "center", "right"},
		"foo.HTTP:\\GIT.SPIFF.IO\x00\u00AB\U00007FFF.insteadOf": []string{"left", "center", "right"},
		`foo."HTTP://GIT.SPIFF.IO".insteadOf`:                   []string{"left", "center", "right"},
	}

	testReadINIMatching(t, nil, src, expected)
	testReadINIError(t, "[section `with raw quote`]")
	testReadINIError(t, `[section "with unclosed quote`)
	testReadINIError(t, `[section "with escape at eof \`)
}

func TestReadINISectionValueComment(t *testing.T) {
	testReadINIMatching(t, nil,
		`[section "Quoted Subsection"] key = ; Comment`,
		Values{
			`section.Quoted Subsection.key`: []string{``},
		},
	)
}

func TestReadINIValueNewline(t *testing.T) {
	expected := Values{`key`: []string{``}}
	testReadINIMatching(t, nil, " key = \n ", expected)
	testReadINIMatching(t, nil, " key =\n ", expected)
	testReadINIMatching(t, nil, " key=\n ", expected)
	testReadINIMatching(t, nil, "\nkey=\n ", expected)
	testReadINIMatching(t, nil, "key=\n ", expected)
	testReadINIMatching(t, nil, "key\t=\n ", expected)
	testReadINIMatching(t, nil, "key\t=\t\n ", expected)
	testReadINIMatching(t, nil, "key=\t\n ", expected)
}

func TestReadINIValueSimple(t *testing.T) {
	expected := Values{`key`: []string{`value`}}
	// In the interest of being possibly unusually thorough.
	testReadINIMatching(t, nil, " key = value ", expected)
	testReadINIMatching(t, nil, " key=value ", expected)
	testReadINIMatching(t, nil, " key= value ", expected)
	testReadINIMatching(t, nil, " key =value ", expected)
	testReadINIMatching(t, nil, " key\t=\tvalue ", expected)
	testReadINIMatching(t, nil, "\tkey\t=\tvalue\t", expected)
	testReadINIMatching(t, nil, "\tkey\t=value\t", expected)
	testReadINIMatching(t, nil, "\tkey=\tvalue\t", expected)
	testReadINIMatching(t, nil, "\tkey=value\t", expected)
}

func TestReadINIFlagSimple(t *testing.T) {
	var (
		expected = Values{`key`: []string{True}}
		empty    = Values{`key`: []string{""}}
	)

	// empty
	testReadINIMatching(t, nil, "key=", empty)
	testReadINIMatching(t, nil, "key=", empty)
	testReadINIMatching(t, nil, "key=``", empty)
	testReadINIMatching(t, nil, `key=""`, empty)
	testReadINIMatching(t, nil, "key= ", empty)
	testReadINIMatching(t, nil, "key= ``", empty)
	testReadINIMatching(t, nil, `key= ""`, empty)
	testReadINIMatching(t, nil, "key = ", empty)
	testReadINIMatching(t, nil, "key = ``", empty)
	testReadINIMatching(t, nil, `key = ""`, empty)
	testReadINIMatching(t, nil, "[] key = ", empty)
	testReadINIMatching(t, nil, "[] key = ``", empty)
	testReadINIMatching(t, nil, `[] key = ""`, empty)

	// true
	testReadINIMatching(t, nil, "key", expected)
	testReadINIMatching(t, nil, " key ", expected)
	testReadINIMatching(t, nil, " key", expected)
	testReadINIMatching(t, nil, " key;comment", expected)
	testReadINIMatching(t, nil, " key ; comment", expected)
	testReadINIMatching(t, nil, " \nkey ", expected)
	testReadINIMatching(t, nil, " \nkey", expected)
	testReadINIMatching(t, nil, " \nkey\n", expected)
	testReadINIMatching(t, nil, " key \n", expected)
	testReadINIMatching(t, nil, " key\n ", expected)
	testReadINIMatching(t, nil, "\tkey\t\n", expected)
	testReadINIMatching(t, nil, "\tkey\t", expected)
	testReadINIMatching(t, nil, "\tkey", expected)
	testReadINIMatching(t, nil, "key\t", expected)

	testReadINIError(t, "key spaced")
}

func TestReadINI_hexstring(t *testing.T) {
	expected := Values{
		"hex": []string{"\x00\u00ab\u00AB\U0000ABAB"},
		"raw": []string{`\x00\u00ab\u00AB\U0000ABAB`},
	}

	testReadINIMatching(t, nil, (`
hex = "\x00\u00ab\u00AB\U0000ABAB"
raw = ` + "`" + `\x00\u00ab\u00AB\U0000ABAB` + "`" + ``)[1:],
		expected)

	testReadINIError(t, `hex = "\xg0\u00ab\u00AB\U0000ABAB"`) // Fail on bad character
	testReadINIError(t, `hex = "\x😀0\u00ab\u00AB\U0000ABAB"`) // Fail on size > 1
	testReadINIError(t, `hex = "\x`)                          // Fail on EOF
	testReadINIError(t, `hex = "\x1`)                         // Fail on EOF
	testReadINIError(t, `hex = "\x12`)                        // Fail on EOF, but not in readHexCode
}

func TestReadINIUnicode(t *testing.T) {
	expected := Values{
		"-_kŭjəl_-": []string{"käkə-pō"},
	}
	testReadINIMatching(t, nil, "-_kŭjəl_- = käkə-pō", expected)
	testReadINIMatching(t, nil, "-_kŭjəl_-=käkə-pō", expected)
	testReadINIMatching(t, nil, "\t-_kŭjəl_-\t=\tkäkə-pō\t", expected)
	testReadINIMatching(t, nil, "-_kŭj′əl_-", Values{"-_kŭj′əl_-": []string{True}})
	testReadINIMatching(t, nil, "[WUBWUB]-_kŭj′əl_-", Values{"WUBWUB.-_kŭj′əl_-": []string{True}})
	testReadINIMatching(t, &Reader{Casing: LowerCase}, "[WUBWUB]-_kŭj′əl_-", Values{"wubwub.-_kŭj′əl_-": []string{True}})
	testReadINIMatching(t, nil, "-_kŭj′əl_- ", Values{"-_kŭj′əl_-": []string{True}})
	testReadINIMatching(t, nil, " -_kŭj′əl_- ", Values{"-_kŭj′əl_-": []string{True}})
	testReadINIMatching(t, nil, "-_kŭj′əl_-\t", Values{"-_kŭj′əl_-": []string{True}})
	testReadINIMatching(t, nil, "\t-_kŭj′əl_-", Values{"-_kŭj′əl_-": []string{True}})
	testReadINIMatching(t, nil, "\t-_kŭj′əl_-\t", Values{"-_kŭj′əl_-": []string{True}})
}

func TestReadMultiline(t *testing.T) {
	expected := Values{
		`foo`: []string{True},
		`bar`: []string{``},
		`baz`: []string{`value`},
	}
	testReadINIMatching(t, nil, "foo\nbar=;\nbaz=value", expected)
	testReadINIMatching(t, nil, "foo;\nbar=\nbaz=value", expected)
	testReadINIMatching(t, nil, "foo\nbar=\nbaz = value", expected)
	testReadINIMatching(t, nil, "foo\t\n\tbar =\nbaz = value", expected)
	testReadINIMatching(t, nil, "foo\t\n\tbar =\nbaz = \"value\"", expected)
}

func TestReadQuoted(t *testing.T) {
	expected := Values{
		`normal`:  []string{`  a thing  `},
		`escaped`: []string{string([]byte{0}) + "\a\b\f\n\r\t\v\\\"jkl;"},
		`raw`:     []string{"a\n`b`\nc\n"},
		`quote`:   []string{"`", `"""`, `""`},
	}

	// In the interest of being possibly unusually thorough.
	testReadINIMatching(t, nil, `
		; Test a fairly normal string
		normal	= "  a thing  "
		escaped	= "\0\a\b\f\n\r\t\v\\\"\j\k\l\;"
		raw	= `+"`a\n``b``\nc\n`"+`
		quote   = `+"````"+`
		quote   = `+"`\"\"\"`"+`
		quote   = """"""`,
		expected)
	testReadINIMatching(t, nil, `
		; Test one with inline characters that could be escaped.
		normal	= "  a thing  "
		escaped	= "\0\a\b\f
\r	\v\\\"\j\k\l\;" ; Tests escaping non-escape characters as themselves
		raw	= `+"`a\n``b``\nc\n`"+`
		quote   = `+"````"+`
		quote   = `+"`\"\"\"`"+`
		quote   = """"""`,
		expected)

	testReadINIError(t, `unterminated = "`)
	testReadINIError(t, `unexpected = """`)

	testReadINIError(t, `"quoted key"`)
	testReadINIError(t, `'quoted key'`)
}

func TestReadININormal(t *testing.T) {
	s := `
a = "5\n
" ; COMMENT1

[ prefix.foo   ] ; COMMENT2
; Comment ; COMMENT3
a=value of "a" ; COMMENT4
b=unhandled ; COMMENT5
c; COMMENT6
; COMMENT7

[prefix.bar]
d =
efg=
hij
k
lmn

[]
no_prefix = this has no prefix
zero = "\x00\xaA\x0F\u00AB\U0000ABAB"
`

	keys := Values{
		`a`:              []string{"5\n\n"},
		`prefix.foo.a`:   []string{`value of "a"`},
		`prefix.foo.b`:   []string{`unhandled`},
		`prefix.foo.c`:   []string{True},
		`prefix.bar.d`:   []string{``},
		`prefix.bar.efg`: []string{``},
		`prefix.bar.hij`: []string{True},
		`prefix.bar.k`:   []string{True},
		`prefix.bar.lmn`: []string{True},
		`no_prefix`:      []string{`this has no prefix`},
		`zero`:           []string{"\x00\xaA\x0F\u00AB\U0000ABAB"},
	}

	testReadINIMatching(t, nil, s, keys)
}

func testReadINIMatching(t *testing.T, dec *Reader, b string, expected map[string][]string) {
	defer pushlog(t)()
	check := func(actual Values, err error) {
		dlog(1, actual)
		defer func() {
			if t.Failed() {
				dlogf(2, "Failed to parse:\n%s", b)
				t.FailNow()
			} else {
				dlog(2, "Succeeded")
			}
		}()

		if err != nil {
			elog(2, "Error reading INI:", err)
			return
		}

		if actual == nil {
			elog(2, "Returned map is nil")
			return
		} else if len(actual) != len(expected) {
			elogf(2, "Returned map has %d values, expected %d", len(actual), len(expected))
		}

		for k, v := range expected {
			mv, ok := actual[k]
			if !ok {
				elogf(2, "Result map does not contain key %q", k)
			}

			if !reflect.DeepEqual(v, mv) {
				elogf(2, "Value of %q in result map %q != (expected) %q", k, mv, v)
			}
		}

		for k := range actual {
			_, ok := expected[k]
			if ok {
				continue
			}
			elogf(2, "Key %q in result is not in expected results", k)
		}
	}

	if dec == nil {
		check(ReadINI([]byte(b), nil))
	}
	for desc, fn := range succReaders {
		r := fn(b)
		dlog(1, "Testing with reader: ", desc)
		dst := Values{}
		check(dst, dec.Read(r, dst))
	}
}

func testReadINIError(t *testing.T, b string) error {
	defer pushlog(t)()
	actual, err := ReadINI([]byte(b), nil)

	if err == nil {
		elog(1, "Expected error, got nil")
	} else {
		dlog(1, "Error returned: ", err)
	}

	if actual != nil {
		elog(1, "Returned map isn't nil")
	}

	return err
}
