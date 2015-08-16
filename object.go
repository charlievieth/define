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
	Methods  []string
}

func newObject(obj types.Object) (*Object, error) {
	o := &Object{
		Name: obj.Name(),
	}
	if obj.Pkg() != nil {
		o.PkgPath = obj.Pkg().Path()
		o.PkgName = obj.Pkg().Name()
	}
	switch typ := obj.(type) {

	case *types.Const:
		o.ObjType = Const

	case *types.Var:
		switch t := typ.Type().(type) {
		case *types.Named:
			o.ObjType = TypeName
			o.IsField = false
			if t.Obj() != nil {
				o.Name = t.Obj().Name()
				if p := t.Obj().Pkg(); p != nil {
					o.PkgPath = p.Path()
					o.PkgName = p.Name()
				}
			}
		case *types.Signature:
			// Need to walk back to the declaration

			// WARN: Remove
			o.ObjType = Var
			o.IsField = typ.IsField()
		default:
			o.ObjType = Var
			o.IsField = typ.IsField()
		}

	case *types.TypeName:
		o.ObjType = TypeName
		if p := typ.Pkg(); p != nil {
			o.PkgPath = p.Path()
			o.PkgName = p.Name()
		}

	case *types.Func:
		o.ObjType = Func
		if p := typ.Pkg(); p != nil {
			o.PkgPath = p.Path()
			o.PkgName = p.Name()
		}
		sig := obj.Type().(*types.Signature)
		if r := sig.Recv(); r != nil {
			o.ObjType = Method
			switch t := r.Type().(type) {
			case *types.Pointer:
				o.Parent = elemTypeName(t.Elem().String())
			case *types.Named:
				if t.Obj() != nil {
					o.Parent = t.Obj().Name()
				}
			case *types.Interface:
				o.ObjType = Interface
				o.Methods = make([]string, t.NumMethods())
				for i := 0; i < t.NumEmbeddeds(); i++ {
					// WARN
					o.Methods[i] = t.Method(i).Name()
				}
				// TODO: This is gonna be hard
			default:
				// WARN
			}
		}

	default:
		o.ObjType = Bad
	}
	return o, nil
}

func elemTypeName(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}
