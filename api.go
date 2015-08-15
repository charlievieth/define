package define

import (
	"errors"
	"go/build"
	"io/ioutil"
	"unicode"
	"unicode/utf8"
)

// Same as token.Position
type Position struct {
	Filename string // filename, if any
	Offset   int    // offset, starting at 0
	Line     int    // line number, starting at 1
	Column   int    // column number, starting at 1 (character count)
}

type Config struct {
	UseOffset bool
	Context   build.Context
}

type Definition struct {
	Name string
	Type string
	Pos  Position
}

func Define(filename string, cursor int, src interface{}, ctxt *build.Context) (*Position, error) {
	text, off, err := readSourceOffset(filename, cursor, src)
	if err != nil {
		return nil, err
	}
	if err := checkSelection(text, off); err != nil {
		return nil, err
	}
	if ctxt == nil {
		ctxt = &build.Default // TODO: Improve
	}
	_ = text
	_ = off
	return nil, nil
}

func checkSelection(src []byte, off int) error {
	// Just to be safe
	if 0 > off || off > len(src) {
		return errors.New("invalid source offset")
	}
	r, _ := utf8.DecodeRune(src[off:])
	if !unicode.IsPrint(r) {
		return errors.New("invalid Go source")
	}
	if unicode.IsSpace(r) {
		return errors.New("nothing to find: whitespace")
	}
	switch src[off] {
	case '!', '%', '&', '(', ')', '*', '+', ',', '-', '/', ':', ';', '<', '=',
		'>', '[', ']', '^', '{', '|', '}':
		return errors.New("nothing to find: reserved Go token")
	}
	return nil
}

func readSourceOffset(filename string, cursor int, src interface{}) ([]byte, int, error) {
	if cursor < 0 {
		return nil, -1, errors.New("non-positive cursor offset")
	}
	switch s := src.(type) {
	case string:
		if cursor >= len(s) {
			return nil, -1, errors.New("invalid cursor offset")
		}
		return []byte(s), stringOffset(s, cursor), nil
	case []byte:
		if cursor >= len(s) {
			return nil, -1, errors.New("invalid cursor offset")
		}
		return s, byteOffset(s, cursor), nil
	case nil:
		b, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, -1, err
		}
		if cursor >= len(b) {
			return nil, -1, errors.New("invalid cursor offset")
		}
		return b, byteOffset(b, cursor), nil
	}
	return nil, -1, errors.New("invalid source")
}

func stringOffset(src string, off int) int {
	for i := range src {
		if off == 0 {
			return i
		}
		off--
	}
	return -1
}

func byteOffset(src []byte, off int) int {
	var i int
	for len(src) != 0 {
		if off == 0 {
			return i
		}
		_, n := utf8.DecodeRune(src)
		src = src[n:]
		i += n
		off--
	}
	return -1
}

func readSource(filename string, src interface{}) ([]byte, error) {
	switch s := src.(type) {
	case nil:
		return ioutil.ReadFile(filename)
	case string:
		return []byte(s), nil
	case []byte:
		return s, nil
	}
	return nil, errors.New("invalid source")
}
