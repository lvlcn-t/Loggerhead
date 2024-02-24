package logger

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	clog "github.com/charmbracelet/log"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name     string
		handlers []slog.Handler
		wantErr  bool
		levelEnv string
	}{
		{
			name:     "No handler with default log level",
			handlers: nil,
			wantErr:  false,
			levelEnv: "",
		},
		{
			name:     "No handler with DEBUG log level",
			handlers: nil,
			wantErr:  false,
			levelEnv: "DEBUG",
		},
		{
			name:     "No handler with NOTICE log level",
			handlers: nil,
			wantErr:  false,
			levelEnv: "NOTICE",
		},
		{
			name:     "No handler with ERROR log level",
			handlers: nil,
			wantErr:  false,
			levelEnv: "ERROR",
		},
		{
			name:     "Custom handler provided",
			handlers: []slog.Handler{slog.NewJSONHandler(os.Stdout, nil)},
			wantErr:  false,
			levelEnv: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOG_LEVEL", tt.levelEnv)

			log := NewLogger(tt.handlers...)

			if (log == nil) != tt.wantErr {
				t.Errorf("NewLogger() error = %v, expectedErr %v", log == nil, tt.wantErr)
			}

			if tt.levelEnv != "" {
				want := getLevel(tt.levelEnv)
				got := log.Enabled(context.Background(), slog.Level(want))
				if !got {
					t.Errorf("Expected log level: %v", want)
				}
			}

			if len(tt.handlers) > 0 && !reflect.DeepEqual(log.Handler(), tt.handlers[0]) {
				t.Errorf("Handler not set correctly")
			}
		})
	}
}

func TestNewContextWithLogger(t *testing.T) {
	tests := []struct {
		name      string
		parentCtx context.Context
	}{
		{
			name:      "With Background context",
			parentCtx: context.Background(),
		},
		{
			name:      "With already set logger in context",
			parentCtx: context.WithValue(context.Background(), logger{}, NewLogger()),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := NewContextWithLogger(tt.parentCtx)
			defer cancel()

			log := ctx.Value(logger{})
			if _, ok := log.(Logger); !ok {
				t.Errorf("Context does not contain Logger, got %T", log)
			}
			if ctx == tt.parentCtx {
				t.Errorf("NewContextWithLogger returned the same context as the parent")
			}
		})
	}
}

func TestFromContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want Logger
	}{
		{
			name: "Context with logger",
			ctx:  IntoContext(context.Background(), NewLogger(slog.NewJSONHandler(os.Stdout, nil))),
			want: NewLogger(slog.NewJSONHandler(os.Stdout, nil)),
		},
		{
			name: "Context without logger",
			ctx:  context.Background(),
			want: NewLogger(),
		},
		{
			name: "Nil context",
			ctx:  nil,
			want: NewLogger(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FromContext(tt.ctx)
			_, ok := got.(*logger)
			if !ok {
				t.Errorf("FromContext() = %T, want %T", got, tt.want)
			}
		})
	}
}

func TestMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		parentCtx   context.Context
		expectInCtx bool
	}{
		{
			name:        "With logger in parent context",
			parentCtx:   IntoContext(context.Background(), NewLogger()),
			expectInCtx: true,
		},
		{
			name:        "Without logger in parent context",
			parentCtx:   context.Background(),
			expectInCtx: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := Middleware(tt.parentCtx)
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, ok := r.Context().Value(logger{}).(Logger)
				if tt.expectInCtx != ok {
					t.Errorf("Middleware() did not inject logger correctly, got %v, want %v", ok, tt.expectInCtx)
				}
			})

			req := httptest.NewRequest("GET", "/", http.NoBody)
			w := httptest.NewRecorder()

			middleware(handler).ServeHTTP(w, req)
		})
	}
}

func TestNewBaseHandler(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		level     string
		wantLevel int
	}{
		{
			name:      "Default handler",
			format:    "",
			level:     "",
			wantLevel: int(slog.LevelInfo),
		},
		{
			name:      "Text handler with custom log level",
			format:    "TEXT",
			level:     "DEBUG",
			wantLevel: int(clog.DebugLevel),
		},
		{
			name:      "JSON handler with custom log level",
			format:    "JSON",
			level:     "WARN",
			wantLevel: int(slog.LevelWarn),
		},
		{
			name:      "Invalid log level",
			format:    "TEXT",
			level:     "UNKNOWN",
			wantLevel: int(clog.InfoLevel),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LOG_FORMAT", tt.format)
			t.Setenv("LOG_LEVEL", tt.level)

			handler := newBaseHandler()

			if tt.format == "TEXT" {
				if _, ok := handler.(*clog.Logger); !ok {
					t.Errorf("Expected handler to be of type *log.Logger")
				}
			} else {
				if _, ok := handler.(*slog.JSONHandler); !ok {
					t.Errorf("Expected handler to be of type *slog.JSONHandler")
				}
			}

			ok := handler.Enabled(context.Background(), slog.Level(tt.wantLevel))
			if !ok {
				t.Errorf("Expected log level: %v", tt.wantLevel)
			}
		})
	}
}
