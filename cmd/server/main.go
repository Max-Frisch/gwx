package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"gwx/internal/config"
	"gwx/internal/engine"
	"gwx/internal/store"
	"gwx/internal/web"
)

func main() {
	log.Println("[INIT] gwx starting...")

	if err := godotenv.Load(); err != nil {
		log.Printf("[INIT] no .env loaded, using process environment")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[FATAL] config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := store.NewPostgres(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] postgres: %v", err)
	}
	defer db.Close()

	if err := upsertUniverse(ctx, db, cfg); err != nil {
		log.Fatalf("[FATAL] universe config: %v", err)
	}
	log.Printf("[INIT] universe %q ready (game x%.2f, fleet x%.2f)",
		cfg.UniverseCode, cfg.GameSpeed, cfg.FleetSpeed)

	eventEngine := engine.NewEventEngine(db)
	go eventEngine.StartScheduler(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "universe": cfg.UniverseCode})
	})
	web.New(db, cfg.UniverseCode).Mount(mux)

	server := &http.Server{Addr: cfg.HTTPAddr, Handler: mux}
	go func() {
		log.Printf("[INIT] listening on http://localhost%s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("[SHUTDOWN] stopping...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	log.Println("[SHUTDOWN] done.")
}

// upsertUniverse applies the operator's env configuration to the universe row on
// every start. This is the "manual config on server start" knob.
func upsertUniverse(ctx context.Context, db *sql.DB, cfg *config.Config) error {
	var endsAt any
	if cfg.EndsAt != nil {
		endsAt = *cfg.EndsAt
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO universes (
			code_name, display_name, game_speed, resource_speed, fleet_speed,
			max_galaxies, max_systems, ends_at, is_active
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE)
		ON CONFLICT (code_name) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			game_speed = EXCLUDED.game_speed,
			resource_speed = EXCLUDED.resource_speed,
			fleet_speed = EXCLUDED.fleet_speed,
			max_galaxies = EXCLUDED.max_galaxies,
			max_systems = EXCLUDED.max_systems,
			ends_at = EXCLUDED.ends_at,
			is_active = TRUE
	`, cfg.UniverseCode, cfg.UniverseName, cfg.GameSpeed, cfg.GameSpeed, cfg.FleetSpeed,
		cfg.MaxGalaxies, cfg.MaxSystems, endsAt)
	return err
}
