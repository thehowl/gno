package gnoweb

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/gnolang/gno/gno.land/pkg/gnoweb/components"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm" // for error types
)

type StaticMetadata struct {
	AssetsPath string
	ChromaPath string
	RemoteHelp string
	ChaindID   string
}

type WebHandlerConfig struct {
	Meta         StaticMetadata
	RenderClient *WebClient
	Formatter    Formatter
}

type WebHandler struct {
	formatter Formatter

	logger *slog.Logger
	static StaticMetadata
	webcli *WebClient

	// bufferPool is used to reuse Buffer instances
	// to reduce memory allocations and improve performance.
	// XXX: maybe this is a too early optimization
	bufferPool sync.Pool
}

func NewWebHandler(logger *slog.Logger, cfg WebHandlerConfig) *WebHandler {
	if cfg.RenderClient == nil {
		logger.Error("no renderer has been defined")
	}

	return &WebHandler{
		formatter: cfg.Formatter,
		webcli:    cfg.RenderClient,
		logger:    logger,
		static:    cfg.Meta,
		// Initialize the pool with bytes.Buffer factory
		bufferPool: sync.Pool{
			New: func() interface{} {
				return &bytes.Buffer{}
			},
		},
	}
}

func (h *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("receiving request", "method", r.Method, "path", r.URL.Path)

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.Get(w, r)
}

type PathKind string

func (h *WebHandler) Get(w http.ResponseWriter, r *http.Request) {
	gnourl, err := ParseGnoURL(r.URL)
	if err != nil {
		h.logger.Error("invalid url", "err", err)
		http.Error(w, "not found: "+r.URL.Path, http.StatusNotFound)
		return
	}

	body := h.getBuffer()
	defer h.putBuffer(body)

	// Render the page body into the buffer
	var status int
	switch gnourl.Kind {
	case KindRealm, KindPure:
		status, err = h.renderRealm(body, gnourl)
	case KindUser:
		// XXX
		fallthrough
	default:
		h.logger.Warn("invalid page kind", "kind", gnourl.Kind)
		status, err = http.StatusNotFound, components.RenderStatusComponent(body, "page not found")
	}

	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)

	var indexData components.IndexData

	// Head
	indexData.HeadData.AssetsPath = h.static.AssetsPath
	indexData.HeadData.ChromaPath = h.static.ChromaPath

	// Header
	indexData.HeaderData.RealmPath = gnourl.Path
	indexData.HeaderData.Breadcrumb.Parts = generateBreadcrumbPaths(gnourl.Path)
	indexData.HeaderData.WebQuery = gnourl.WebQuery

	indexData.Body = template.HTML(body.String())
	// Render the final page with the rendered body
	err = components.RenderIndexComponent(w, indexData)

	if err != nil {
		h.logger.Error("failed to render index component", "err", err)
	}

	return
}

func (h *WebHandler) renderRealmHelp(w io.Writer, gnourl *GnoURL) (status int, err error) {
	fsigs, err := h.webcli.Functions(gnourl.Path)
	if err != nil {
		h.logger.Error("unable to fetch path functions", "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	// Catch last name of the path
	// XXX: we should probably add a condition within the template
	realmName := filepath.Base(gnourl.Path)
	err = components.RenderHelpComponent(w, components.HelpData{
		RealmName: realmName,
		ChainId:   h.static.ChaindID,
		PkgPath:   gnourl.HostPath(),
		Remote:    h.static.RemoteHelp,
		Functions: fsigs,
	})
	if err != nil {
		h.logger.Error("unable to render helper", "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	return http.StatusOK, nil
}

func (h *WebHandler) renderRealmSource(w io.Writer, gnourl *GnoURL) (status int, err error) {
	pkgPath := gnourl.Path

	files, err := h.webcli.Sources(pkgPath)
	if err != nil {
		h.logger.Error("unable to list sources file", "path", gnourl.Path, "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	if len(files) == 0 {
		h.logger.Debug("no file(s) available", "path", gnourl.Path)
		return http.StatusOK, components.RenderStatusComponent(w, "no files available")
	}

	var fileName string
	file := gnourl.WebQuery.Get("file")
	if file == "" {
		fileName = files[0]
	} else if contains(files, file) {
		fileName = file
	} else {
		h.logger.Error("unable to render source", "file", file, "err", "file does not exist")
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	source, err := h.webcli.SourceFile(pkgPath, fileName)
	if err != nil {
		h.logger.Error("unable to get source file", "file", fileName, "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	fileLines := strings.Count(string(source), "\n")
	fileSizeKb := float64(len(source)) / 1024.0
	fileSizeStr := fmt.Sprintf("%.2f Kb", fileSizeKb)

	hsource, err := h.highlightSource(fileName, source)
	if err != nil {
		h.logger.Error("unable to highlight source file", "file", fileName, "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	err = components.RenderSourceComponent(w, components.SourceData{
		PkgPath:     gnourl.Path,
		Files:       files,
		FileName:    fileName,
		FileCounter: len(files),
		FileLines:   fileLines,
		FileSize:    fileSizeStr,
		FileSource:  template.HTML(hsource),
	})
	if err != nil {
		h.logger.Error("unable to render helper", "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	return http.StatusOK, nil
}

func (h *WebHandler) renderRealmDirectory(w io.Writer, gnourl *GnoURL) (status int, err error) {
	pkgPath := gnourl.Path

	files, err := h.webcli.Sources(pkgPath)
	if err != nil {
		h.logger.Error("unable to list sources file", "path", gnourl.Path, "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	if len(files) == 0 {
		h.logger.Debug("no file(s) available", "path", gnourl.Path)
		return http.StatusOK, components.RenderStatusComponent(w, "no files available")
	}

	err = components.RenderDirectoryComponent(w, components.DirData{
		PkgPath:     gnourl.Path,
		Files:       files,
		FileCounter: len(files),
	})
	if err != nil {
		h.logger.Error("unable to render directory", "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	return http.StatusOK, nil
}

func (h *WebHandler) renderRealm(w io.Writer, gnourl *GnoURL) (status int, err error) {
	h.logger.Info("component render", "path", gnourl.Path, "args", gnourl.Args)

	// Display realm help page ?
	if gnourl.WebQuery.Has("help") {
		return h.renderRealmHelp(w, gnourl)
	}

	// Display realm source page ?
	switch {
	// XXX: it would probably be better to have this as a middleware somehow
	case endsWithRune(gnourl.Path, '/') || isFile(gnourl.Path):
		i := strings.LastIndexByte(gnourl.Path, '/')
		if i < 0 {
			return http.StatusInternalServerError, fmt.Errorf("unable get ending slash for %q", gnourl.Path)
		}

		// Fill webquery with file infos
		gnourl.WebQuery.Set("source", "") // set source
		if file := gnourl.Path[i+1:]; file != "" {
			gnourl.WebQuery.Set("file", file)
		}
		gnourl.Path = gnourl.Path[:i]

		fallthrough // render realm source
	case gnourl.WebQuery.Has("source"):
		return h.renderRealmSource(w, gnourl)
	}

	// TODO: Display realm dir page (TO REMOVE)
	if gnourl.WebQuery.Has("dir") {
		return h.renderRealmDirectory(w, gnourl)
	}

	// Render content into the content buffer

	content := h.getBuffer()
	defer h.putBuffer(content)

	meta, err := h.webcli.Render(content, gnourl.Path, gnourl.EncodeArgs())
	if err != nil {
		if errors.Is(err, vm.InvalidPkgPathError{}) {
			return http.StatusNotFound, components.RenderStatusComponent(w, "not found")
		}

		h.logger.Error("unable to render markdown", "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	err = components.RenderRealmComponent(w, components.RealmData{
		TocItems: &components.RealmTOCData{
			Items: meta.Items,
		},
		Content: template.HTML(content.String()),
	})

	if err != nil {
		h.logger.Error("unable to render template", "err", err)
		return http.StatusInternalServerError, components.RenderStatusComponent(w, "internal error")
	}

	// Write the rendered content to the response writer
	return http.StatusOK, nil
}

func (h *WebHandler) highlightSource(fileName string, src []byte) ([]byte, error) {
	var lexer chroma.Lexer
	switch strings.ToLower(filepath.Ext(fileName)) {
	case ".gno":
		lexer = lexers.Get("go")
	case ".md":
		lexer = lexers.Get("markdown")
	case ".mod":
		lexer = lexers.Get("gomod")
	default:
		return nil, fmt.Errorf("unsupported extension for highlighting source file: %q", fileName)
	}

	if lexer == nil {
		return nil, fmt.Errorf("unsuported lexer for file %q", fileName)
	}

	iterator, err := lexer.Tokenise(nil, string(src))
	if err != nil {
		h.logger.Error("unable to ", "fileName", fileName, "err", err)
	}

	var buff bytes.Buffer
	if err := h.formatter.Format(&buff, iterator); err != nil {
		return nil, fmt.Errorf("unable to format source file %q: %w", fileName, err)
	}

	return buff.Bytes(), nil
}

// getBuffer retrieves a buffer from the sync.Pool
func (h *WebHandler) getBuffer() *bytes.Buffer {
	return h.bufferPool.Get().(*bytes.Buffer)
}

// putBuffer resets and puts a buffer back into the sync.Pool
func (h *WebHandler) putBuffer(buf *bytes.Buffer) {
	buf.Reset()
	h.bufferPool.Put(buf)
}

func generateBreadcrumbPaths(path string) []components.BreadcrumbPart {
	split := strings.Split(path, "/")
	parts := []components.BreadcrumbPart{}

	var name string
	for i := range split {
		if name = split[i]; name == "" {
			continue
		}

		parts = append(parts, components.BreadcrumbPart{
			Name: name,
			Path: strings.Join(split[:i+1], "/"),
		})
	}

	return parts
}

func contains(files []string, file string) bool {
	for _, f := range files {
		if f == file {
			return true
		}
	}
	return false
}

// EndsWithRune checks if the given path ends with the specified rune.
func endsWithRune(path string, r rune) bool {
	if len(path) == 0 {
		return false
	}
	return rune(path[len(path)-1]) == r
}

// IsFile checks if the last element of the path is a file (has an extension).
func isFile(path string) bool {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return ext != ""
}