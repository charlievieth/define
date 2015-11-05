package define

import (
	"errors"
	"fmt"
	"go/token"
	"golang.org/x/tools/go/types"
)

type Type int

const (
	Invalid Type = iota
	Bad
	Const
	Var
	TypeName
	Func
	Method
	Interface
	Package
)

var typeNames = [...]string{
	"Invalid",
	"Bad",
	"Const",
	"Var",
	"TypeName",
	"Func",
	"Method",
	"Interface",
	"Package",
}

func (t Type) String() string {
	if int(t) < len(typeNames) {
		return typeNames[t]
	}
	return typeNames[Invalid]
}

// Same as token.Position
type Position struct {
	Filename string // filename, if any
	Offset   int    // offset, starting at 0
	Line     int    // line number, starting at 1
	Column   int    // column number, starting at 1 (character count)
}

func newPosition(tp token.Position) *Position {
	p := Position(tp)
	return &p
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
	pos      token.Pos
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

func (o *Object) Finder() (f posFinder, err error) {
	switch o.ObjType {
	case Const, TypeName, Func, Interface:
		f = declFinder{Name: o.Name}
	case Var:
		if o.IsField {
			if o.Parent == "" {
				err = fmt.Errorf("Finder: missing Parent for field: %s", o.Name)
			}
			f = fieldFinder{
				Name:   o.Name,
				Parent: o.Parent,
			}
		} else {
			f = declFinder{Name: o.Name}
		}
	case Method:
		f = methodFinder{Name: o.Name, TypeName: o.Parent}
	case Package:
		f = docFinder{}
	case Bad:
		err = errors.New("Finder: bad object type")
	}
	return
}

func newSelector(sel *types.Selection) (*Object, error) {
	if sel.Obj() == nil {
		return nil, errors.New("nil Selection object")
	}
	o := &Object{
		Name: sel.Obj().Name(),
		pos:  sel.Obj().Pos(),
	}
	o.setPkg(sel.Obj().Pkg())
	switch t := derefType(sel.Recv()).(type) {
	case *types.Named:
		o.setParent(t.Obj())
	default:
		// TODO: log type
		// Locally declared type, maybe an anonymous struct.
		return nil, fmt.Errorf("unexpected Recv type: %#v for object: %#v", t, sel.Obj())
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

func newObject(obj types.Object, sel *types.Selection) (*Object, error) {
	// WARN: Dev only
	if sel != nil {
		return newSelector(sel)
	}
	o := &Object{
		Name: obj.Name(),
		pos:  obj.Pos(),
	}
	o.setPkg(obj.Pkg())
	switch typ := obj.(type) {
	case *types.PkgName:
		o.ObjType = Package
		if p := typ.Imported(); p != nil {
			o.setPkg(p)
		}
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
				o.pos = obj.Pos() // WARN
			}
		}
	case *types.Func:
		if sig := typ.Type().(*types.Signature); sig != nil {
			o.ObjType = Func
		} else {
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
