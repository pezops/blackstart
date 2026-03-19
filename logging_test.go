package blackstart

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogReplaceAttr_CustomKeys(t *testing.T) {
	cfg := &RuntimeConfig{
		LogLevelKey:   "severity",
		LogMessageKey: "event",
	}

	replacer := logReplaceAttr(cfg)
	require.NotNil(t, replacer)

	levelAttr := replacer(nil, slog.Attr{Key: slog.LevelKey})
	msgAttr := replacer(nil, slog.Attr{Key: slog.MessageKey})
	otherAttr := replacer(nil, slog.Attr{Key: "operation"})

	assert.Equal(t, "severity", levelAttr.Key)
	assert.Equal(t, "event", msgAttr.Key)
	assert.Equal(t, "operation", otherAttr.Key)
}

func TestLogReplaceAttr_DefaultKeys(t *testing.T) {
	cfg := &RuntimeConfig{
		LogLevelKey:   " ",
		LogMessageKey: "",
	}

	replacer := logReplaceAttr(cfg)
	require.NotNil(t, replacer)

	levelAttr := replacer(nil, slog.Attr{Key: slog.LevelKey})
	msgAttr := replacer(nil, slog.Attr{Key: slog.MessageKey})

	assert.Equal(t, "level", levelAttr.Key)
	assert.Equal(t, "msg", msgAttr.Key)
}

func TestJSONLogger_DefaultOutputKeys(t *testing.T) {
	var buf bytes.Buffer
	cfg := &RuntimeConfig{
		LogFormat: "json",
		LogLevel:  "info",
	}

	logger := newLoggerForWriter(cfg, &buf)
	logger.Info("workflow started", "operation", "op1")

	line := bytes.TrimSpace(buf.Bytes())
	require.NotEmpty(t, line)

	var got map[string]interface{}
	err := json.Unmarshal(line, &got)
	require.NoError(t, err)

	assert.Contains(t, got, "time")
	assert.Equal(t, "INFO", got["level"])
	assert.Equal(t, "workflow started", got["msg"])
	assert.Equal(t, "op1", got["operation"])
}

func TestJSONLogger_CustomOutputKeys(t *testing.T) {
	var buf bytes.Buffer
	cfg := &RuntimeConfig{
		LogFormat:     "json",
		LogLevel:      "info",
		LogLevelKey:   "severity",
		LogMessageKey: "event",
	}

	logger := newLoggerForWriter(cfg, &buf)
	logger.Info("workflow started", "operation", "op1")

	line := bytes.TrimSpace(buf.Bytes())
	require.NotEmpty(t, line)

	var got map[string]interface{}
	err := json.Unmarshal(line, &got)
	require.NoError(t, err)

	assert.Contains(t, got, "time")
	assert.Equal(t, "INFO", got["severity"])
	assert.Equal(t, "workflow started", got["event"])
	assert.Equal(t, "op1", got["operation"])
	assert.NotContains(t, got, "level")
	assert.NotContains(t, got, "msg")
}

func TestTextLogger_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := &RuntimeConfig{
		LogFormat: "text",
		LogLevel:  "info",
	}

	logger := newLoggerForWriter(cfg, &buf)
	logger.Info("workflow started", "operation", "op1")

	line := string(bytes.TrimSpace(buf.Bytes()))
	require.NotEmpty(t, line)

	assert.Contains(t, line, "INFO")
	assert.Contains(t, line, "workflow started")
	assert.Contains(t, line, "operation=op1")
}

func TestTextLogger_WithAttrs_ChainsWithoutDropping(t *testing.T) {
	var buf bytes.Buffer
	cfg := &RuntimeConfig{
		LogFormat: "text",
		LogLevel:  "info",
	}

	logger := newLoggerForWriter(cfg, &buf).With("workflow", "wf-a").With("namespace", "default")
	logger.Info("starting workflow execution")

	line := string(bytes.TrimSpace(buf.Bytes()))
	require.NotEmpty(t, line)
	assert.Contains(t, line, "workflow=wf-a")
	assert.Contains(t, line, "namespace=default")
}
