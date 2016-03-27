package ini

import (
	"reflect"
	"strings"
	"testing"
)

func TestValues_copy(t *testing.T) {
	expectedCopied := Values{
		"foo": []string{"baz", "quux"},
		"wub": []string{"456"},
	}
	expectedCopiedTo := Values{
		"foo": []string{"bar", "baz", "quux"},
		"wub": []string{"456"},
	}

	v := Values{}

	v.Add("foo", "baz")
	v.Add("foo", "quux")
	v.Set("wub", "123")
	v.Set("wub", "456")
	v.Set("bob", "someone")
	v.Del("bob")

	dup := v.Copy(nil)
	duppedTo := v.Copy(Values{"foo": []string{"bar"}})

	if !reflect.DeepEqual(v, expectedCopied) {
		t.Errorf("v = %#v; want %#v", v, expectedCopied)
	}
	if !reflect.DeepEqual(dup, expectedCopied) {
		t.Errorf("dup = %#v; want %#v", dup, expectedCopied)
	}
	if !reflect.DeepEqual(duppedTo, expectedCopiedTo) {
		t.Errorf("duppedTo = %#v; want %#v", duppedTo, expectedCopiedTo)
	}

	check := func(k, want string) {
		if got := expectedCopiedTo.Get(k); got != want {
			t.Errorf("duppedTo.Get(%q) = %q; want %q", k, got, want)
		}
	}

	check("foo", "bar")
	check("nothing", "")
}

func TestValues_contains(t *testing.T) {
	v := Values{"foo": nil}
	check := func(k string, want bool) {
		if got := v.Contains(k); want != got {
			t.Errorf("v.Contains(%q) = %t; want %t", k, got, want)
		}
	}

	check("foo", true)
	check("not.present", false)
}

func TestValues_matching(t *testing.T) {
	v := Values{
		"foo.bar":  nil,
		"foo.baz":  []string{"x"},
		"quux.bar": []string{"wop"},
		"foo":      []string{"a thing"},
	}

	check := func(got, expected Values) {
		if !reflect.DeepEqual(expected, got) {
			t.Errorf("v.Matching(...) = %#v; want %#v", got, expected)
		} else {
			t.Log("v.Matching(...) = %#v", got)
		}
	}

	check(v.Matching(nil, func(s string, _ []string) bool {
		return strings.HasPrefix(s, "foo.")
	}), Values{
		"foo.bar": nil,
		"foo.baz": []string{"x"},
	})

	check(v.Matching(Values{"foo.baz": []string{"y"}}, func(s string, _ []string) bool {
		return strings.HasPrefix(s, "foo.")
	}), Values{
		"foo.bar": nil,
		"foo.baz": []string{"y", "x"},
	})
}
