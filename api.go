package define

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"io/ioutil"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/types"
)

type Config struct {
	UseOffset bool
	Context   build.Context
}

func (c *Config) Define(filename string, cursor int, src interface{}) (*Position, []byte, error) {
	// TODO: Refactor this mess!
	text, err := readSource(filename, src)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSelection(text, cursor); err != nil {
		return nil, nil, err
	}
	af, fset, err := parseFile(filename, text)
	if err != nil {
		return nil, nil, err
	}
	node, err := nodeAtOffset(af, fset, cursor)
	if err != nil {
		return nil, nil, err
	}
	ctx := newContext(filename, af, fset, &c.Context)
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	if _, ok := node.(*ast.SelectorExpr); ok {
		info.Selections = make(map[*ast.SelectorExpr]*types.Selection)
	}
	conf := types.Config{}
	if _, err := conf.Check(ctx.dirname, ctx.fset, ctx.files, info); err != nil {
		// Return error only if missing type info.
		if len(info.Defs) == 0 && len(info.Uses) == 0 {
			return nil, nil, err
		}
	}
	obj, sel, err := lookupType(node, info)
	if err != nil {
		return nil, nil, err
	}
	o, err := newObject(obj, sel)
	if err != nil {
		return nil, nil, err
	}
	f, err := o.Finder()
	if err != nil {
		return nil, nil, err
	}
	tp, objSrc, err := ctx.objectPosition(o.PkgPath, f)
	if err != nil {
		if o.pos.IsValid() {
			if p := positionFor(o.pos, fset); p != nil {
				return newPosition(*p), objSrc, nil
			}
		}
		return nil, nil, err
	}
	return newPosition(*tp), objSrc, nil
}

func (c *Config) Object(filename string, cursor int, src interface{}) (*Object, []byte, error) {
	// TODO: Refactor this mess!
	text, err := readSource(filename, src)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSelection(text, cursor); err != nil {
		return nil, nil, err
	}
	af, fset, err := parseFile(filename, text)
	if err != nil {
		return nil, nil, err
	}
	node, err := nodeAtOffset(af, fset, cursor)
	if err != nil {
		return nil, nil, err
	}
	ctx := newContext(filename, af, fset, &c.Context)
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	if _, ok := node.(*ast.SelectorExpr); ok {
		info.Selections = make(map[*ast.SelectorExpr]*types.Selection)
	}
	conf := types.Config{}
	if _, err := conf.Check(ctx.dirname, ctx.fset, ctx.files, info); err != nil {
		// Return error only if missing type info.
		if len(info.Defs) == 0 && len(info.Uses) == 0 {
			return nil, nil, err
		}
	}
	obj, sel, err := lookupType(node, info)
	if err != nil {
		return nil, nil, err
	}
	o, err := newObject(obj, sel)
	if err != nil {
		return nil, nil, err
	}
	f, err := o.Finder()
	if err != nil {
		return nil, nil, err
	}
	tp, objSrc, err := ctx.objectPosition(o.PkgPath, f)
	if err != nil {
		if o.pos.IsValid() {
			if p := positionFor(o.pos, fset); p != nil {
				o.Position = Position(*p)
				return o, objSrc, nil
			}
		}
		return nil, nil, err
	}
	if tp != nil {
		o.Position = Position(*tp)
	}
	return o, objSrc, nil
}

var DefaultConfig = Config{
	UseOffset: false,
	Context:   build.Default,
}

func Define(filename string, cursor int, src interface{}) (*Position, error) {
	pos, _, err := DefaultConfig.Define(filename, cursor, src)
	return pos, err
}

func NodeAtOffset(filename string, cursor int, src interface{}) (ast.Node, token.Position, error) {
	var pos token.Position
	text, off, err := readSourceOffset(filename, cursor, src)
	if err != nil {
		return nil, pos, err
	}
	if err := checkSelection(text, off); err != nil {
		return nil, pos, err
	}
	af, fset, err := parseFile(filename, text)
	if err != nil {
		return nil, pos, err
	}
	node, err := nodeAtOffset(af, fset, cursor)
	if err != nil {
		return nil, pos, err
	}
	pos = fset.Position(node.Pos())
	return node, pos, nil
}

func ObjectOf(filename string, cursor int) (types.Object, *types.Selection, error) {
	text, off, err := readSourceOffset(filename, cursor, nil)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSelection(text, off); err != nil {
		return nil, nil, err
	}
	af, fset, err := parseFile(filename, text)
	if err != nil {
		return nil, nil, err
	}
	node, err := nodeAtOffset(af, fset, cursor)
	if err != nil {
		return nil, nil, err
	}
	ctx := newContext(filename, af, fset, &build.Default)
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	if _, ok := node.(*ast.SelectorExpr); ok {
		info.Selections = make(map[*ast.SelectorExpr]*types.Selection)
	}
	conf := types.Config{}
	if _, err := conf.Check(ctx.dirname, ctx.fset, ctx.files, info); err != nil {
		// Return error only if missing type info.
		if len(info.Defs) == 0 && len(info.Uses) == 0 {
			return nil, nil, err
		}
	}
	return lookupType(node, info)
}

func FindObject(filename string, cursor int) (*Object, error) {
	text, off, err := readSourceOffset(filename, cursor, nil)
	if err != nil {
		return nil, err
	}
	if err := checkSelection(text, off); err != nil {
		return nil, err
	}
	af, fset, err := parseFile(filename, text)
	if err != nil {
		return nil, err
	}
	node, err := nodeAtOffset(af, fset, cursor)
	if err != nil {
		return nil, err
	}
	ctx := newContext(filename, af, fset, &build.Default)
	info := &types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	if _, ok := node.(*ast.SelectorExpr); ok {
		info.Selections = make(map[*ast.SelectorExpr]*types.Selection)
	}
	conf := types.Config{}
	if _, err := conf.Check(ctx.dirname, ctx.fset, ctx.files, info); err != nil {
		// Return error only if missing type info.
		if len(info.Defs) == 0 && len(info.Uses) == 0 {
			return nil, err
		}
	}
	obj, sel, err := lookupType(node, info)
	if err != nil {
		return nil, err
	}
	if sel != nil {
		return newSelector(sel)
	}
	return newObject(obj, nil)
}

func newTypeInfo(node ast.Node) *types.Info {
	info := types.Info{
		Defs: make(map[*ast.Ident]types.Object),
		Uses: make(map[*ast.Ident]types.Object),
	}
	if _, ok := node.(*ast.SelectorExpr); ok {
		info.Selections = make(map[*ast.SelectorExpr]*types.Selection)
	}
	return &info
}

func lookupType(node ast.Node, info *types.Info) (types.Object, *types.Selection, error) {
	switch n := node.(type) {
	case *ast.Ident:
		return info.ObjectOf(n), nil, nil
	case *ast.ImportSpec:
		return info.ObjectOf(n.Name), nil, nil
	case *ast.SelectorExpr:
		if sel, ok := info.Selections[n]; ok {
			return sel.Obj(), sel, nil
		}
		return info.ObjectOf(n.Sel), nil, nil
	case *ast.StructType:
		return nil, nil, fmt.Errorf("unexpected struct: %#v", node)
	default:
		return nil, nil, fmt.Errorf("unexpected type: %#v", node)
	}
	return nil, nil, errors.New("object not found...")
}

func checkSelection(src []byte, off int) error {
	// Just to be safe
	if off < 0 {
		return errors.New("invalid selection: non-positive offset")
	}
	if off >= len(src) {
		return errors.New("invalid selection: offset out of range")
	}
	switch src[off] {
	case '!', '%', '&', '(', ')', '*', '+', ',', '-', '/', ':', ';', '<', '=',
		'>', '[', ']', '^', '{', '|', '}':
		return errors.New("invalid selection: reserved Go token")
	}
	r, _ := utf8.DecodeRune(src[off:])
	if !unicode.IsPrint(r) {
		return errors.New("invalid selection: not valid Go source")
	}
	if unicode.IsSpace(r) {
		return errors.New("invalid selection: whitespace")
	}
	return nil
}

func readSourceOffset(filename string, cursor int, src interface{}) ([]byte, int, error) {
	if cursor < 0 {
		return nil, -1, errors.New("non-positive offset")
	}
	var (
		b   []byte
		n   int
		err error
	)
	switch s := src.(type) {
	case []byte:
		b = s
	case string:
		if cursor < len(s) {
			n = stringOffset(s, cursor)
			b = []byte(s)
		}
	case nil:
		b, err = ioutil.ReadFile(filename)
	default:
		err = errors.New("invalid source")
	}
	if err == nil && n == 0 {
		if cursor < len(b) {
			n = byteOffset(b, cursor)
		} else {
			err = errors.New("offset out of range")
		}
	}
	return b, n, err
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
	// TODO: This needs to tested.
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
