package define

import (
	"errors"
	"go/scanner"
	"go/token"
)

type tokenPos struct {
	off int
	end int
	pos token.Pos
	tok token.Token
}

func (t tokenPos) Pos() int { return t.off }

func (t tokenPos) End() int { return t.end }

func (t tokenPos) Valid() bool {
	return t.off != 0 && t.end != 0 && t.tok != token.ILLEGAL
}

type iterator struct {
	tokens []tokenPos
	index  int
	scan   *scanner.Scanner
}

func (i *iterator) token() tokenPos {
	return i.tokens[i.index]
}

func (i *iterator) previous() (t tokenPos) {
	if i.index > 0 {
		i.index--
		t = i.tokens[i.index]
	}
	return t
}

func (i *iterator) next() (t tokenPos) {
	if i.index < len(i.tokens)-1 {
		i.index++
		t = i.tokens[i.index]
	}
	return t
}

func NewIterator(file *token.File, fset *token.FileSet, src []byte, cursor int) (*iterator, error) {
	if file.Size() != len(src) {
		return nil, errors.New("token: file size does not match src len")
	}
	iter := iterator{
		scan:   &scanner.Scanner{},
		tokens: make([]tokenPos, 0, 1024),
	}
	iter.scan.Init(file, src, nil, 0)
	for {
		pos, tok, lit := iter.scan.Scan()
		if tok == token.EOF {
			break
		}
		off := fset.Position(pos).Offset
		iter.tokens = append(iter.tokens, tokenPos{
			off: off,
			pos: pos,
			end: off + len(lit),
			tok: tok,
		})
		if cursor <= off {
			break
		}
		iter.index++
	}
	return &iter, nil
}

func (i *iterator) Context() tokenPos {
	switch i.previous().tok {
	case token.PERIOD, token.COMMA, token.LBRACE:
		return i.previous()
	case token.IDENT, token.TYPE, token.CONST, token.VAR, token.FUNC, token.PACKAGE:
		return i.token()
	default:
		return tokenPos{}
	}
}

func (i *iterator) Selector() tokenPos {
	if i.previous().tok == token.PERIOD {
		return i.previous()
	}
	return tokenPos{}
}

func newIterator(file *token.File, fset *token.FileSet, src []byte, offset int) iterator {
	scan := &scanner.Scanner{}
	scan.Init(file, src, nil, 0)
	tokens := make([]tokenPos, 0, 1024)
	index := 0
	for {
		pos, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		off := fset.Position(pos).Offset
		tokens = append(tokens, tokenPos{
			off: off,
			pos: pos,
			end: off + len(lit),
			tok: tok,
		})
		if offset <= off {
			break
		}
		index++
	}
	return iterator{
		tokens: tokens,
		index:  index,
	}
}

func cursorContext(file *token.File, fset *token.FileSet, src []byte, offset int) (current, parent tokenPos) {
	iter := newIterator(file, fset, src, offset)
	switch iter.previous().tok {
	case token.PERIOD, token.COMMA, token.LBRACE:
		current = iter.previous()
	case token.IDENT, token.TYPE, token.CONST, token.VAR, token.FUNC, token.PACKAGE:
		current = iter.token()
	}
	if iter.previous().tok == token.PERIOD {
		parent = iter.previous()
	}
	return current, parent
}
