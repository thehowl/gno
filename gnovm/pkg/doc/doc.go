// Package doc implements support for documentation of Gno packages and realms,
// in a similar fashion to `go doc`.
// As a reference, the [official implementation] for `go doc` is used.
//
// [official implementation]: https://github.com/golang/go/tree/90dde5dec1126ddf2236730ec57511ced56a512d/src/cmd/doc
package doc

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/multierr"
)

// DocumentOption is used to pass options to the [Documentable].Document.
type DocumentOption func(s *documentOptions)

type documentOptions struct {
	all        bool
	src        bool
	unexported bool
	short      bool
	w          io.Writer
}

// WithShowAll shows all symbols when displaying documentation about a package.
func WithShowAll(b bool) DocumentOption {
	return func(s *documentOptions) { s.all = b }
}

// WithSource shows source when documenting a symbol.
func WithSource(b bool) DocumentOption {
	return func(s *documentOptions) { s.src = b }
}

// WithUnexported shows unexported symbols as well as exported.
func WithUnexported(b bool) DocumentOption {
	return func(s *documentOptions) { s.unexported = b }
}

// WithShort shows a one-line representation for each symbol.
func WithShort(b bool) DocumentOption {
	return func(s *documentOptions) { s.short = b }
}

// WithWriter uses the given writer as an output.
// By default, os.Stdout is used.
func WithWriter(w io.Writer) DocumentOption {
	return func(s *documentOptions) { s.w = w }
}

// Documentable is a package, symbol, or accessible which can be documented.
type Documentable interface {
	Document(...DocumentOption) error
}

type documentable struct {
	Dir
	symbol     string
	accessible string
	pkgData    *pkgData
}

func (d *documentable) Document(opts ...DocumentOption) error {
	o := &documentOptions{w: os.Stdout}
	for _, opt := range opts {
		opt(o)
	}

	var err error
	// pkgData may already be initialised if we already had to look to see
	// if it had the symbol we wanted; otherwise initialise it now.
	if d.pkgData == nil {
		d.pkgData, err = newPkgData(d.Dir, o.unexported)
		if err != nil {
			return err
		}
	}

	astpkg, pkg, err := d.pkgData.docPackage(o)
	if err != nil {
		return err
	}

	// copied from go source - map vars, constants and constructors to their respective types.
	typedValue := make(map[*doc.Value]bool)
	constructor := make(map[*doc.Func]bool)
	for _, typ := range pkg.Types {
		pkg.Consts = append(pkg.Consts, typ.Consts...)
		pkg.Vars = append(pkg.Vars, typ.Vars...)
		pkg.Funcs = append(pkg.Funcs, typ.Funcs...)
		if o.unexported || token.IsExported(typ.Name) {
			for _, value := range typ.Consts {
				typedValue[value] = true
			}
			for _, value := range typ.Vars {
				typedValue[value] = true
			}
			for _, fun := range typ.Funcs {
				// We don't count it as a constructor bound to the type
				// if the type itself is not exported.
				constructor[fun] = true
			}
		}
	}

	pp := &pkgPrinter{
		name:        d.pkgData.name,
		pkg:         astpkg,
		file:        ast.MergePackageFiles(astpkg, 0),
		doc:         pkg,
		typedValue:  typedValue,
		constructor: constructor,
		fs:          d.pkgData.fset,
		opt:         o,
		importPath:  d.importPath,
	}
	pp.buf.pkg = pp

	return d.output(pp)
}

func (d *documentable) output(pp *pkgPrinter) (err error) {
	defer func() {
		pp.flush()
		if err == nil {
			err = pp.err
		}
	}()

	switch {
	case d.symbol == "" && d.accessible == "":
		if pp.opt.all {
			pp.allDoc()
			return
		}
		pp.packageDoc()
	case d.symbol != "" && d.accessible == "":
		pp.symbolDoc(d.symbol)
	default: // both non-empty
		if pp.methodDoc(d.symbol, d.accessible) {
			return
		}
		if pp.fieldDoc(d.symbol, d.accessible) {
			return
		}
	}

	return
}

// set as a variable so it can be changed by testing.
var fpAbs = filepath.Abs

// ResolveDocumentable returns a Documentable from the given arguments.
// Refer to the documentation of gnodev doc for the formats accepted (in general
// the same as the go doc command).
func ResolveDocumentable(dirs *Dirs, args []string, unexported bool) (Documentable, error) {
	parsed, ok := parseArgs(args)
	if !ok {
		return nil, fmt.Errorf("commands/doc: invalid arguments: %v", args)
	}
	return resolveDocumentable(dirs, parsed, unexported)
}

func resolveDocumentable(dirs *Dirs, parsed docArgs, unexported bool) (Documentable, error) {
	var candidates []Dir

	// if we have a candidate package name, search dirs for a dir that matches it.
	// prefer directories whose import path match precisely the package
	if s, err := os.Stat(parsed.pkg); err == nil && s.IsDir() {
		// expand to full path
		absVal, err := fpAbs(parsed.pkg)
		if err == nil {
			candidates = dirs.findDir(absVal)
		}
	}
	// arg is either not a dir, or if it matched a local dir it was not
	// valid (ie. not scanned by dirs). try parsing as a package
	if len(candidates) == 0 {
		candidates = dirs.findPackage(parsed.pkg)
	}

	if len(candidates) == 0 {
		// there are no candidates.
		// if this is ambiguous, remove ambiguity and try parsing args using pkg as the symbol.
		if !parsed.pkgAmbiguous {
			return nil, fmt.Errorf("commands/doc: package not found: %q (note: local packages are not yet supported)", parsed.pkg)
		}
		parsed = docArgs{pkg: ".", sym: parsed.pkg, acc: parsed.sym}
		return resolveDocumentable(dirs, parsed, unexported)
	}
	// we wanted documentation about a package, and we found one!
	if parsed.sym == "" {
		return &documentable{Dir: candidates[0]}, nil
	}

	// we also have a symbol, and maybe accessible.
	// search for the symbol through the candidates

	doc := &documentable{
		symbol:     parsed.sym,
		accessible: parsed.acc,
	}

	var matchFunc func(s symbolData) bool
	if parsed.acc == "" {
		matchFunc = func(s symbolData) bool {
			return (s.accessible == "" && symbolMatch(parsed.sym, s.symbol)) ||
				(s.typ == symbolDataMethod && symbolMatch(parsed.sym, s.accessible))
		}
	} else {
		matchFunc = func(s symbolData) bool {
			return symbolMatch(parsed.sym, s.symbol) && symbolMatch(parsed.acc, s.accessible)
		}
	}

	var errs []error
	for _, candidate := range candidates {
		pd, err := newPkgData(candidate, unexported)
		if err != nil {
			// silently ignore errors -
			// likely invalid AST in a file.
			errs = append(errs, err)
			continue
		}
		for _, sym := range pd.symbols {
			if matchFunc(sym) {
				doc.Dir = candidate
				doc.pkgData = pd
				// match found. return this as documentable.
				return doc, multierr.Combine(errs...)
			}
		}
	}
	return nil, multierr.Append(
		fmt.Errorf("commands/doc: could not resolve arguments: %+v", parsed),
		multierr.Combine(errs...),
	)
}

// docArgs represents the parsed args of the doc command.
// sym could be a symbol, but the accessibles of types should also be shown if they match sym.
type docArgs struct {
	pkg string // always set
	sym string
	acc string // short for "accessible". only set if sym is also set

	// pkg could be a symbol in the local dir.
	// if that is the case, and sym != "", then sym, acc = pkg, sym
	pkgAmbiguous bool
}

func parseArgs(args []string) (docArgs, bool) {
	switch len(args) {
	case 0:
		return docArgs{pkg: "."}, true
	case 1:
		// allowed syntaxes (acc is method or field, [] marks optional):
		// <pkg>
		// [<pkg>.]<sym>[.<acc>]
		// [<pkg>.][<sym>.]<acc>
		// if the (part) argument contains a slash, then it is most certainly
		// a pkg.
		// note: pkg can be a relative path. this is mostly problematic for ".." and
		// ".". so we count full stops from the last slash.
		slash := strings.LastIndexByte(args[0], '/')
		if args[0] == "." || args[0] == ".." ||
			(slash != -1 && args[0][slash+1:] == "..") {
			// special handling for common ., .. and ./..
			// these will generally work poorly if you try to use the one-argument
			// syntax to access a symbol/accessible.
			return docArgs{pkg: args[0]}, true
		}
		switch strings.Count(args[0][slash+1:], ".") {
		case 0:
			if slash != -1 {
				return docArgs{pkg: args[0]}, true
			}
			return docArgs{pkg: args[0], pkgAmbiguous: true}, true
		case 1:
			pos := strings.IndexByte(args[0][slash+1:], '.') + slash + 1
			if slash != -1 {
				return docArgs{pkg: args[0][:pos], sym: args[0][pos+1:]}, true
			}
			if token.IsExported(args[0]) {
				// See rationale here:
				// https://github.com/golang/go/blob/90dde5dec1126ddf2236730ec57511ced56a512d/src/cmd/doc/main.go#L265
				return docArgs{pkg: ".", sym: args[0][:pos], acc: args[0][pos+1:]}, true
			}
			return docArgs{pkg: args[0][:pos], sym: args[0][pos+1:], pkgAmbiguous: true}, true
		case 2:
			// pkg.sym.acc
			parts := strings.Split(args[0][slash+1:], ".")
			return docArgs{
				pkg: args[0][:slash+1] + parts[0],
				sym: parts[1],
				acc: parts[2],
			}, true
		default:
			return docArgs{}, false
		}
	case 2:
		switch strings.Count(args[1], ".") {
		case 0:
			return docArgs{pkg: args[0], sym: args[1]}, true
		case 1:
			pos := strings.IndexByte(args[1], '.')
			return docArgs{pkg: args[0], sym: args[1][:pos], acc: args[1][pos+1:]}, true
		default:
			return docArgs{}, false
		}
	default:
		return docArgs{}, false
	}
}