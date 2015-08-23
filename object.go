package define

import (
	"errors"
	"fmt"
	"go/token"
	"golang.org/x/tools/go/types"
)

type Type int

const (
	Bad Type = iota
	Const
	Var
	TypeName
	Func
	Method
	Interface
)

var typeNames = [...]string{
	"Bad",
	"Const",
	"Var",
	"TypeName",
	"Func",
	"Method",
	"Interface",
}

func (t Type) String() string {
	return typeNames[t]
}

// Same as token.Position
type Position struct {
	Filename string // filename, if any
	Offset   int    // offset, starting at 0
	Line     int    // line number, starting at 1
	Column   int    // column number, starting at 1 (character count)
}

func newPosition(p token.Position) *Position {
	return &Position{
		Filename: p.Filename,
		Line:     p.Line,
		Column:   p.Column,
		Offset:   p.Offset,
	}
}

func (p Position) IsValid() bool { return p.Line > 0 }

func (p Position) String() string {
	s := p.Filename
	if p.IsValid() {
		if s != "" {
			s += ":"
		}
		s += fmt.Sprintf("%d:%d", p.Line, p.Column)
	}
	if s == "" {
		s = "-"
	}
	return s
}

type Object struct {
	Name     string
	Parent   string // parent type
	PkgName  string
	PkgPath  string
	ObjType  Type // only relevanty when finding imported types
	Position Position
	IsField  bool // only relevant when finding imported types
	Pos      token.Pos
}

func (o *Object) setPkg(p *types.Package) {
	if p != nil {
		o.PkgPath = p.Path()
		o.PkgName = p.Name()
	}
}

func (o *Object) setParent(obj types.Object) {
	if obj != nil {
		o.Parent = obj.Name()
		o.setPkg(obj.Pkg())
	}
}

func newSelector(sel *types.Selection) (*Object, error) {
	if sel.Obj() == nil {
		return nil, errors.New("nil Selection object")
	}
	o := &Object{
		Name: sel.Obj().Name(),
		Pos:  sel.Obj().Pos(),
	}
	o.setPkg(sel.Obj().Pkg())
	switch t := derefType(sel.Recv()).(type) {
	case *types.Named:
		o.setParent(t.Obj())
	default:
		// TODO: log type
		// Locally declared type, maybe an anonymous struct.
	}
	switch sel.Kind() {
	case types.FieldVal:
		o.IsField = true
		o.ObjType = Var
	case types.MethodVal:
		o.ObjType = Method
	case types.MethodExpr:
		// TODO: Fix
		o.ObjType = Method
	}
	return o, nil
}

func newObject(obj types.Object) (*Object, error) {
	o := &Object{
		Name: obj.Name(),
		Pos:  obj.Pos(),
	}
	o.setPkg(obj.Pkg())
	switch typ := obj.(type) {
	case *types.Const:
		o.ObjType = Const
	case *types.TypeName:
		o.ObjType = TypeName
	case *types.Var:
		o.ObjType = Var
		o.IsField = typ.IsField()
		if t, ok := derefType(typ.Type()).(*types.Named); ok {
			o.ObjType = TypeName
			// WARN: This looks wrong
			o.IsField = false
			if obj := t.Obj(); obj != nil {
				o.Name = obj.Name()
				o.setPkg(obj.Pkg())
				o.Pos = obj.Pos() // WARN
			}
		}
	case *types.Func:
		switch sig := obj.Type().(type) {
		case nil:
			o.ObjType = Func
		case *types.Signature:
			switch r := derefType(sig.Recv().Type()).(type) {
			case *types.Named:
				o.ObjType = Method
				o.setParent(r.Obj())
			case *types.Interface:
				o.ObjType = Interface
			default:
				// This should never happen
			}
		}
	default:
		// TODO: log type
		o.ObjType = Bad
	}
	return o, nil
}

func derefType(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

func elemTypeName(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}
