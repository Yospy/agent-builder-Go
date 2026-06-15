// Command agent-builder is the single Go service: the orchestrator with the engine,
// tools, and store as packages inside it. It seeds the Agent Builder row, wires the
// tool registry, and serves the HTTP/SSE API on :8080.
package main

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"agent-builder/brain"
	"agent-builder/logx"
	"agent-builder/platformtools"
	"agent-builder/runtime"
	"agent-builder/store"
	"agent-builder/tools"
	"agent-builder/tracing"

	"github.com/openai/openai-go/v3/option"
)

func main() {
	// Best-effort .env load (repo root, one level up, or cwd). Never overrides real env.
	loadDotenv("../.env")
	loadDotenv(".env")

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY is not set (put it in .env or the environment)")
	}
	model := envOr("OPENAI_MODEL", "gpt-5.1")
	titleModel := envOr("OPENAI_TITLE_MODEL", "gpt-5.4-nano")
	dbPath := envOr("DB_PATH", "agent-builder.db")
	logsDir := envOr("LOGS_DIR", "logs")
	workDir := envOr("WORK_DIR", "workspace")
	webDir := envOr("WEB_DIR", "web")
	addr := envOr("ADDR", ":8080")
	openAITimeout := time.Duration(envIntOr("OPENAI_TIMEOUT_SECONDS", 120)) * time.Second

	logger := logx.New()

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if err := os.MkdirAll(workDir, 0o755); err != nil {
		log.Fatalf("create work dir: %v", err)
	}

	reg := tools.NewRegistry()
	reg.Register(tools.Calculator())
	reg.Register(tools.FetchURL())
	reg.Register(tools.ReadFile(workDir))
	reg.Register(tools.WriteFile(workDir))
	platformtools.Register(reg, st)

	if err := platformtools.SeedBuilder(context.Background(), st); err != nil {
		log.Fatalf("seed builder: %v", err)
	}

	// Optional Braintrust tracing (no-op unless BRAINTRUST_API_KEY is set).
	tracingShutdown, traced, err := tracing.Init(envOr("BRAINTRUST_PROJECT", "agent-builder"))
	if err != nil {
		log.Fatalf("tracing: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = tracingShutdown(ctx)
	}()

	var brainOpts []option.RequestOption
	if traced {
		brainOpts = append(brainOpts, tracing.OpenAIMiddleware())
		logger.Info("braintrust tracing enabled", "project", envOr("BRAINTRUST_PROJECT", "agent-builder"))
	}
	b := brain.NewOpenAIBrain(apiKey, model, brainOpts...)
	b.SetLogger(logger)
	b.SetTimeout(openAITimeout)
	srv := runtime.NewServer(st, reg, b, logsDir, webDir, logger)
	srv.SetTitleSummarizer(brain.NewOpenAITitleSummarizer(apiKey, titleModel, brainOpts...))

	// No WriteTimeout: SSE responses are long-lived. ReadHeaderTimeout + IdleTimeout
	// guard against slow-header (Slowloris) and idle-connection exhaustion.
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM so in-flight turns drain and — crucially —
	// the deferred tracingShutdown flushes any buffered Braintrust spans on the way out.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server: %v", err)
		}
	}()
	logger.Info("agent-builder listening", "addr", addr, "model", model, "db", dbPath, "openai_timeout", openAITimeout.String())

	<-ctx.Done()
	logger.Info("shutting down")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		logger.Warn("http shutdown", "err", err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}

// loadDotenv reads KEY=VALUE lines from path into the environment, without overriding
// variables already set. Quiet and best-effort: a missing file is fine.
func loadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if k != "" && os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}
}
