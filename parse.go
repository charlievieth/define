package define

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"golang.org/x/tools/go/types"

	_ "golang.org/x/tools/go/gcimporter"
)

type context struct {
	filename string
	dirname  string
	incTest  bool // include test files
	src      []byte
	ctx      *build.Context

	info    *types.Info
	infoErr error

	af    *ast.File   // Source file
	files []*ast.File // Package files
	fset  *token.FileSet
}

func newContext(filename string, af *ast.File, fset *token.FileSet, ctx *build.Context) *context {
	if ctx == nil {
		ctx = &build.Default
	}
	name := filepath.Clean(filename)
	c := context{
		filename: name,
		dirname:  filepath.Dir(name),
		incTest:  hasSuffix(name, "_test.go"),
		ctx:      ctx,
		af:       af,
		fset:     fset,
		files:    []*ast.File{af},
	}
	c.parseTargetDir()
	return &c
}

type findRes struct {
	pos *token.Position
	src []byte
	err error
}

func (c *context) objectPosition(pkgpath string, f posFinder) (*token.Position, []byte, error) {
	if f == nil {
		// should not happen
		return nil, nil, errors.New("define: nil finder")
	}
	path, err := c.pkgPath(pkgpath)
	if err != nil {
		return nil, nil, err
	}
	names, err := pkgFiles(&build.Default, path, false)
	if err != nil {
		return nil, nil, err
	}
	chs := make([]chan *findRes, 0, len(names))
	for _, name := range names {
		chs = append(chs, searchAstFile(name, f))
	}
	var first error
	for _, ch := range chs {
		switch p := <-ch; {
		case p == nil:
			// should not happen
			first = errors.New("define: nil response on find chan")
		case p.pos != nil:
			// Exit: success
			return p.pos, p.src, nil
		case p.err != nil && first == nil:
			first = err
		}
	}
	if first == nil {
		// WARN: Dev only
		first = fmt.Errorf("pkgpath: %s finder: %#v", pkgpath, f)
	}
	return nil, nil, first
}

func searchAstFile(path string, f posFinder) chan *findRes {
	ch := make(chan *findRes)
	go func() {
		b, err := ioutil.ReadFile(path)
		if b != nil && f.Candidate(b) {
			af, fset, err := parseFile(path, b)
			pos := f.Find(af, fset)
			ch <- &findRes{pos: pos, src: b, err: err}
		} else {
			// Send read err if b is nil.
			ch <- &findRes{err: err}
		}
	}()
	return ch
}

func (c *context) findPkgDoc(pkgPath, pkgName string) (*token.Position, error) {
	path, err := c.pkgPath(pkgPath)
	if err != nil {
		return nil, err
	}
	names, err := pkgFiles(c.ctx, path, false)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	fset := token.NewFileSet()
	doc := filepath.Join(pkgPath, "doc.go")
	if n := sort.SearchStrings(names, doc); n < len(names) {
		af, _ := parser.ParseFile(fset, doc, nil, parser.PackageClauseOnly)
		if af.Comments != nil && len(af.Comments) != 0 {
			p := fset.Position(af.Comments[0].Pos())
			return &p, nil
		}
	}
	return nil, nil
}

func (c *context) pkgPath(name string) (string, error) {
	for _, dir := range c.ctx.SrcDirs() {
		path := filepath.Join(dir, name)
		if isGoPkgDir(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("path not found for pacakge: %s", name)
}

func (c *context) parseTargetDir() error {
	// TODO: don't wait for all files to be read before parsing
	srcs, err := c.readDirSource(c.filename)
	if err != nil {
		return err
	}
	if len(srcs) != 0 {
		files, _ := parseDir(srcs, c.fset)
		c.files = append(c.files, files...)
	}
	return nil
}

func parseDir(srcs map[string][]byte, fset *token.FileSet) ([]*ast.File, error) {
	// TODO: don't wait for all files to be read before parsing
	var first error
	files := make([]*ast.File, 0, len(srcs))
	mu := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	wg.Add(len(srcs))
	for name, src := range srcs {
		go func(name string, src []byte) {
			defer wg.Done()
			af, err := parser.ParseFile(fset, name, src, parser.ParseComments)
			mu.Lock()
			if af != nil {
				files = append(files, af)
			}
			if err != nil && first == nil {
				first = err
			}
			mu.Unlock()
		}(name, src)
	}
	wg.Wait()
	return files, first
}

func (c *context) readDirSource(skip string) (map[string][]byte, error) {
	// TOOD: Remove
	names, err := c.pkgFiles(c.dirname)
	if err != nil {
		return nil, err
	}
	srcs := make(map[string][]byte)
	for _, name := range names {
		if name != skip {
			if src, _ := ioutil.ReadFile(name); src != nil {
				srcs[name] = src
			}
		}
	}
	return srcs, nil
}

func (c *context) pkgFiles(dir string) ([]string, error) {
	return pkgFiles(c.ctx, dir, c.incTest)
}

func pkgFiles(c *build.Context, dir string, test bool) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	names, err := f.Readdirnames(-1)
	if err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	var n int
	for i := 0; i < len(names); i++ {
		if matchFile(c, dir, names[i], test) {
			names[n] = filepath.Join(dir, names[i])
			n++
		}
	}
	return names[:n], nil
}

func parseFile(filename string, src []byte) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if af == nil {
		return nil, nil, err
	}
	return af, fset, nil
}

func (c *context) MatchFile(dir, name string) (match bool) {
	return matchFile(c.ctx, dir, name, c.incTest)
}

func matchFile(c *build.Context, dir, name string, test bool) (match bool) {
	if isGoSource(name, test) {
		match, _ = c.MatchFile(dir, name)
	}
	return match
}

func isGoPkgDir(dirname string) bool {
	f, err := os.Open(dirname)
	if err != nil {
		return false
	}
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return false
	}
	for _, s := range names {
		if isGoSource(s, false) {
			return true
		}
	}
	return false
}

func isGoSource(s string, includeTest bool) bool {
	return len(s) > len(".go") && s[0] != '_' && s[0] != '.' &&
		hasSuffix(s, ".go") && (includeTest || !hasSuffix(s, "_test.go"))
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[0:len(prefix)] == prefix
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}
