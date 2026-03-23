package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/husseinbbassam/intelligent-inbox/internal/api"
	"github.com/husseinbbassam/intelligent-inbox/internal/infrastructure"
	"github.com/husseinbbassam/intelligent-inbox/internal/repository"
	"github.com/husseinbbassam/intelligent-inbox/internal/usecase"
)

func main() {
	// ── Configuration ────────────────────────────────────────────────────────
	dsn := mustEnv("POSTGRES_DSN")
	azureEndpoint := mustEnv("AZURE_OPENAI_ENDPOINT")
	azureAPIKey := mustEnv("AZURE_OPENAI_API_KEY")
	azureDeployment := mustEnv("AZURE_OPENAI_DEPLOYMENT")
	port := getEnv("PORT", "8080")

	// ── Infrastructure layer ─────────────────────────────────────────────────
	db, err := repository.NewDB(dsn)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	judge, err := infrastructure.NewAzureOpenAIJudge(azureEndpoint, azureAPIKey, azureDeployment)
	if err != nil {
		log.Fatalf("AI judge init failed: %v", err)
	}

	// ── Repository layer ─────────────────────────────────────────────────────
	recordsRepo := repository.NewIngestedRecordRepository(db)
	feedbackRepo := repository.NewHumanFeedbackRepository(db)

	// ── Use-case layer ───────────────────────────────────────────────────────
	ingestionUC := usecase.NewIngestionUseCase(recordsRepo)
	feedbackUC := usecase.NewFeedbackUseCase(recordsRepo, feedbackRepo)
	aiJudgeUC := usecase.NewAIJudgeUseCase(recordsRepo, feedbackRepo, judge)

	// ── API layer ────────────────────────────────────────────────────────────
	handler := api.NewHandler(ingestionUC, feedbackUC, recordsRepo)
	router := api.NewRouter(handler)

	// ── Background worker ────────────────────────────────────────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go aiJudgeUC.StartWorker(ctx)

	// ── Graceful shutdown ────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("shutdown signal received")
		cancel() // stop the AI Judge worker
	}()

	log.Printf("server listening on :%s", port)
	if err = router.Start(":" + port); err != nil {
		log.Printf("server stopped: %v", err)
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %q is not set", key)
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
