package blackstart

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
)

var _ slog.Handler = &textHandler{}

type textHandler struct {
	mu     *sync.Mutex
	out    io.Writer
	group  string
	attrs  []slog.Attr
	level  slog.Leveler
	source bool
}

func NewTextHandler(o io.Writer, opts *slog.HandlerOptions) slog.Handler {
	if opts == nil {
		opts = &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}
	}
	if opts.Level == nil {
		opts.Level = slog.LevelInfo
	}
	return &textHandler{
		out:    o,
		level:  opts.Level,
		source: opts.AddSource,
		mu:     &sync.Mutex{},
	}
}

func (h *textHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &textHandler{out: h.out, mu: h.mu, group: h.group, level: h.level, source: h.source, attrs: attrs}
}

func (h *textHandler) WithGroup(name string) slog.Handler {
	return &textHandler{out: h.out, mu: h.mu, attrs: h.attrs, level: h.level, source: h.source, group: name}
}

func (h *textHandler) Handle(_ context.Context, r slog.Record) error {

	formattedTime := r.Time.Format("2006/01/02 15:04:05.999")
	logStrings := []string{formattedTime, r.Level.String(), r.Message}

	attrAppend := func(a slog.Attr) bool {
		if h.group != "" {
			a.Key = h.group + "." + a.Key
		}
		logStrings = append(logStrings, fmt.Sprintf("%s=%s", a.Key, a.Value.String()))
		return true
	}

	for _, a := range h.attrs {
		attrAppend(a)
	}

	if h.source {
		attrAppend(slog.Any(slog.SourceKey, source(r)))
	}

	r.Attrs(attrAppend)

	result := strings.Join(logStrings, " ") + "\n"
	b := []byte(result)

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(b)
	return err
}

func source(r slog.Record) *slog.Source {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	return &slog.Source{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}
}

func NewTestingLogger() *slog.Logger {
	ll := slog.Level(0)
	logOpts := &slog.HandlerOptions{
		AddSource:   false,
		Level:       ll,
		ReplaceAttr: nil,
	}
	logHandler := NewTextHandler(os.Stdout, logOpts)
	return slog.New(logHandler)
}

func NewLogger(config *RuntimeConfig) *slog.Logger {
	var err error
	var logHandler slog.Handler
	var logWriter io.Writer

	if config == nil {
		config = &RuntimeConfig{
			LogFormat: "",
			LogLevel:  "info",
		}
	}

	// log output writer
	switch config.LogOutput {
	case "", "-":
		logWriter = os.Stdout
	default:
		// open file and set a writer
		var file *os.File
		file, err = os.OpenFile(config.LogOutput, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("failed to open log file: %v", err)
		}
		defer func(file *os.File) {
			err = file.Close()
			if err != nil {
				log.Println("failed to close log file:", err)
			}
		}(file)

		logWriter = file
	}

	// log level
	ll := slog.Level(0)
	err = ll.UnmarshalText([]byte(config.LogLevel))
	if err != nil {
		log.Fatalf("invalid log level: %v: %v", config.LogLevel, err)
	}

	logOpts := &slog.HandlerOptions{
		AddSource:   false,
		Level:       ll,
		ReplaceAttr: nil,
	}

	// log logHandler
	switch config.LogFormat {
	case "json":
		logHandler = slog.NewJSONHandler(logWriter, logOpts)
	default:
		logHandler = NewTextHandler(logWriter, logOpts)
	}

	return slog.New(logHandler)
}
