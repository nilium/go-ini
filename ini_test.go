package ini

import (
	"reflect"
	"testing"
)

func TestReadINIEmpty(t *testing.T) {
	testReadINIMatching(t, "\n\t\n;empty\n\t\n\t", map[string][]string{})
}

func TestReadINISectionSpaces(t *testing.T) {
	// Errors
	testReadINIError(t, "\n[newline section\n]\nk = v\n")
	testReadINIError(t, "\n[\nnewline section]\nk = v\n")
	testReadINIError(t, "\n[\nnewline\nsection]\nk = v\n")

	// Good
	expected := map[string][]string{`section.ok.k`: []string{`v`}}
	testReadINIMatching(t, `
		[section ok]
		k = v
		`,
		expected)
}

func TestReadQuotedMulti(t *testing.T) {
	src := `
	[foo "http://git.spiff.io"]
		insteadOf = left
		insteadOf = right
	`
	expected := map[string][]string{
		`foo.http://git.spiff.io.insteadOf`: []string{"left", "right"},
	}

	testReadINIMatching(t, src, expected)
}

func TestReadINISectionValueComment(t *testing.T) {
	testReadINIMatching(t,
		` key = ; `,
		map[string][]string{
			`key`: []string{``},
		},
	)
}

func TestReadINIValueNewline(t *testing.T) {
	expected := map[string][]string{`key`: []string{``}}
	testReadINIMatching(t, " key = \n ", expected)
	testReadINIMatching(t, " key =\n ", expected)
	testReadINIMatching(t, " key=\n ", expected)
	testReadINIMatching(t, "\nkey=\n ", expected)
	testReadINIMatching(t, "key=\n ", expected)
	testReadINIMatching(t, "key\t=\n ", expected)
	testReadINIMatching(t, "key\t=\t\n ", expected)
	testReadINIMatching(t, "key=\t\n ", expected)
}

func TestReadINIValueSimple(t *testing.T) {
	expected := map[string][]string{`key`: []string{`value`}}
	// In the interest of being possibly unusually thorough.
	testReadINIMatching(t, " key = value ", expected)
	testReadINIMatching(t, " key=value ", expected)
	testReadINIMatching(t, " key= value ", expected)
	testReadINIMatching(t, " key =value ", expected)
	testReadINIMatching(t, " key\t=\tvalue ", expected)
	testReadINIMatching(t, "\tkey\t=\tvalue\t", expected)
	testReadINIMatching(t, "\tkey\t=value\t", expected)
	testReadINIMatching(t, "\tkey=\tvalue\t", expected)
	testReadINIMatching(t, "\tkey=value\t", expected)
}

func TestReadINIFlagSimple(t *testing.T) {
	expected := map[string][]string{
		`key`: []string{True},
	}

	testReadINIMatching(t, "key", expected)
	testReadINIMatching(t, " key ", expected)
	testReadINIMatching(t, " key", expected)
	testReadINIMatching(t, " key;comment", expected)
	testReadINIMatching(t, " key ; comment", expected)
	testReadINIMatching(t, " \nkey ", expected)
	testReadINIMatching(t, " \nkey", expected)
	testReadINIMatching(t, " \nkey\n", expected)
	testReadINIMatching(t, " key \n", expected)
	testReadINIMatching(t, " key\n ", expected)
	testReadINIMatching(t, "\tkey\t\n", expected)
	testReadINIMatching(t, "\tkey\t", expected)
	testReadINIMatching(t, "\tkey", expected)
	testReadINIMatching(t, "key\t", expected)

	testReadINIError(t, "key spaced")
}

func TestReadINIUnicode(t *testing.T) {
	expected := map[string][]string{
		"-_kŭjəl_-": []string{"käkə-pō"},
	}
	testReadINIMatching(t, "-_kŭjəl_- = käkə-pō", expected)
	testReadINIMatching(t, "-_kŭjəl_-=käkə-pō", expected)
	testReadINIMatching(t, "\t-_kŭjəl_-\t=\tkäkə-pō\t", expected)

	testReadINIError(t, "-_kŭj′əl_-")
	testReadINIError(t, " -_kŭj′əl_-")
	testReadINIError(t, "-_kŭj′əl_- ")
	testReadINIError(t, " -_kŭj′əl_- ")
	testReadINIError(t, "-_kŭj′əl_-\t")
	testReadINIError(t, "\t-_kŭj′əl_-")
	testReadINIError(t, "\t-_kŭj′əl_-\t")
}

func TestReadMultiline(t *testing.T) {
	expected := map[string][]string{
		`foo`: []string{True},
		`bar`: []string{``},
		`baz`: []string{`value`},
	}
	testReadINIMatching(t, "foo\nbar=;\nbaz=value", expected)
	testReadINIMatching(t, "foo;\nbar=\nbaz=value", expected)
	testReadINIMatching(t, "foo\nbar=\nbaz = value", expected)
	testReadINIMatching(t, "foo\t\n\tbar =\nbaz = value", expected)
	testReadINIMatching(t, "foo\t\n\tbar =\nbaz = \"value\"", expected)
}

func TestReadQuoted(t *testing.T) {
	expected := map[string][]string{
		`normal`:  []string{`  a thing  `},
		`escaped`: []string{string([]byte{0}) + "\a\b\f\n\r\t\v\\\"jkl;"},
	}

	// In the interest of being possibly unusually thorough.
	testReadINIMatching(t, `
		; Test a fairly normal string
		normal	= "  a thing  "
		escaped	= "\0\a\b\f\n\r\t\v\\\"\j\k\l\;"
		`, expected)
	testReadINIMatching(t, `
		; Test one with inline characters that could be escaped.
		normal	= "  a thing  "
		escaped	= "\0\a\b\f
\r	\v\\\"\j\k\l\;" ; Tests escaping non-escape characters as themselves
		`, expected)

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
`

	keys := map[string][]string{
		`a`:              []string{"5\n\n"},
		`prefix.foo.a`:   []string{`value of "a"`},
		`prefix.foo.b`:   []string{`unhandled`},
		`prefix.foo.c`:   []string{`1`},
		`prefix.bar.d`:   []string{``},
		`prefix.bar.efg`: []string{``},
		`prefix.bar.hij`: []string{`1`},
		`prefix.bar.k`:   []string{`1`},
		`prefix.bar.lmn`: []string{`1`},
		`no_prefix`:      []string{`this has no prefix`},
	}

	testReadINIMatching(t, s, keys)
}

func testReadINIMatching(t *testing.T, b string, expected map[string][]string) {
	actual, err := ReadINI([]byte(b), nil)

	if err != nil {
		t.Error("Error reading INI:", err)
	}

	if actual == nil {
		t.Fatalf("Returned map is nil")
	} else if len(actual) != len(expected) {
		t.Errorf("Returned map has %d values, expected %d", len(actual), len(expected))
	}

	for k, v := range expected {
		mv, ok := actual[k]
		if !ok {
			t.Errorf("Result map does not contain key %q", k)
		}

		if !reflect.DeepEqual(v, mv) {
			t.Errorf("Value of %q in result map %q != (expected) %q", k, mv, v)
		}
	}

	for k := range actual {
		_, ok := expected[k]
		if ok {
			continue
		}
		t.Errorf("Key %q in result is not in expected results", k)
	}
}

func testReadINIError(t *testing.T, b string) error {
	actual, err := ReadINI([]byte(b), nil)

	if err == nil {
		t.Errorf("Expected error, got nil")
	} else {
		t.Log("Error returned:", err)
	}

	if actual != nil {
		t.Errorf("Returned map isn't nil")
	}

	return err
}
