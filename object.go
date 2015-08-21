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

func newSelector(sel *types.Selection) (*Object, error) {
	if sel.Obj() == nil {
		return nil, errors.New("nil Selection object")
	}
	o := &Object{
		Name: sel.Obj().Name(),
	}
	if p := sel.Obj().Pkg(); p != nil {
		o.PkgPath = p.Path()
		o.PkgName = p.Name()
	}
	switch t := sel.Recv().(type) {
	case *types.Named:
		if t.Obj() == nil {
			break // WARN: error
		}
		o.Parent = t.Obj().Name()
		if p := t.Obj().Pkg(); p != nil {
			o.PkgPath = p.Path()
			o.PkgName = p.Name()
		}
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
		default:
			o.ObjType = Var
			o.IsField = typ.IsField()
			if p := typ.Pkg(); p != nil {
				o.PkgPath = p.Path()
				o.PkgName = p.Name()
			}
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
				// WARN: This shouldnt happen...
				switch tt := t.Elem().(type) {
				case *types.Named:
					if tt.Obj() != nil {
						o.Parent = tt.Obj().Name()
					}
				case *types.Interface:
					o.ObjType = Interface
					o.Methods = make([]string, tt.NumMethods())
					for i := 0; i < tt.NumEmbeddeds(); i++ {
						// WARN
						o.Methods[i] = tt.Method(i).Name()
					}
				default:
					o.Parent = elemTypeName(t.Elem().String())
				}
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
				o.Parent = elemTypeName(t.String())
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
