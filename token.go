package find

import (
	"go/scanner"
	"go/token"
)

type Token struct {
	off int
	end int
	pos token.Pos
	tok token.Token
}

func (t Token) Pos() int { return t.off }

func (t Token) End() int { return t.end }

func (t Token) Valid() bool {
	return t.off != 0 && t.end != 0 && t.tok != token.ILLEGAL
}

type Iterator struct {
	tokens []Token
	index  int
}

func (i *Iterator) Token() Token {
	return i.tokens[i.index]
}

func (i *Iterator) Previous() (t Token) {
	if i.index > 0 {
		i.index--
		t = i.tokens[i.index]
	}
	return t
}

func (i *Iterator) Next() (t Token) {
	if i.index < len(i.tokens)-1 {
		i.index++
		t = i.tokens[i.index]
	}
	return t
}

func newIterator(file *token.File, fset *token.FileSet, src []byte, offset int) Iterator {
	scan := &scanner.Scanner{}
	scan.Init(file, src, nil, 0)
	tokens := make([]Token, 0, 1024)
	index := 0
	for {
		pos, tok, lit := scan.Scan()
		if tok == token.EOF {
			break
		}
		off := fset.Position(pos).Offset
		tokens = append(tokens, Token{
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
	return Iterator{
		tokens: tokens,
		index:  index,
	}
}

func cursorContext(file *token.File, fset *token.FileSet, src []byte, offset int) (current, parent Token) {
	iter := newIterator(file, fset, src, offset)
	switch iter.Previous().tok {
	case token.PERIOD, token.COMMA, token.LBRACE:
		current = iter.Previous()
	case token.IDENT, token.TYPE, token.CONST, token.VAR, token.FUNC, token.PACKAGE:
		current = iter.Token()
	}
	if iter.Previous().tok == token.PERIOD {
		parent = iter.Previous()
	}
	return current, parent
}
