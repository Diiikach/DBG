package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/term-paper-2026/backend/internal/auth"
	"github.com/term-paper-2026/backend/internal/handlers"
)

// Deps — зависимости роутера, чтобы не плодить позиционных параметров.
type Deps struct {
	Logger    *slog.Logger
	Issuer    *auth.Issuer
	CORSOrigs []string

	Auth     *handlers.AuthHandler
	Patient  *handlers.PatientHandler
	Variant  *handlers.VariantHandler
	Sample   *handlers.SampleHandler
	Export   *handlers.ExportHandler
}

// New собирает маршруты REST API.
//
// Публично доступны только:
//   - GET  /healthz
//   - POST /api/auth/register
//   - POST /api/auth/login
//
// Всё остальное закрыто RequireAuth.
func New(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogRequestLogger(d.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Minute))

	allowedOrigins := d.CORSOrigs
	if len(allowedOrigins) == 0 {
		allowedOrigins = []string{"*"}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Route("/api", func(r chi.Router) {
		// Public auth routes.
		r.Route("/auth", func(r chi.Router) {
			r.Post("/register", d.Auth.Register)
			r.Post("/login", d.Auth.Login)
			// /me требует токена.
			r.Group(func(r chi.Router) {
				r.Use(RequireAuth(d.Issuer))
				r.Get("/me", d.Auth.Me)
			})
		})

		// Protected routes.
		r.Group(func(r chi.Router) {
			r.Use(RequireAuth(d.Issuer))

			r.Route("/patients", func(r chi.Router) {
				// Экспорт регистрируем до /{id}, чтобы chi не интерпретировал
				// "export" как patient id.
				r.Get("/export", d.Export.ExportPatients)

				r.Post("/", d.Patient.Create)
				r.Get("/", d.Patient.List)
				r.Get("/{id}", d.Patient.Get)
				r.Patch("/{id}", d.Patient.Update)
				r.Delete("/{id}", d.Patient.Delete)

				// deprecated sync alignment
				r.Post("/{id}/align", d.Variant.Align)
				r.Get("/{id}/variants", d.Variant.ListVariants)
				r.Get("/{id}/variants/export", d.Export.ExportPatientVariants)

				// Variant import endpoints (модули 3/4/5).
				r.Post("/{id}/variants", d.Variant.AddManualVariant)
				r.Post("/{id}/variants/vcf", d.Variant.ImportVCF)
				r.Post("/{id}/variants/csv", d.Variant.ImportCSV)

				// samples
				r.Post("/{id}/samples", d.Sample.Upload)
				r.Get("/{id}/samples", d.Sample.List)
			})

			r.Route("/samples", func(r chi.Router) {
				r.Get("/", d.Sample.ListAll)
				r.Get("/{id}", d.Sample.Get)
			})

			r.Route("/variants", func(r chi.Router) {
				// Поиск по rs_id/гену/координатам (модуль 6).
				r.Get("/", d.Variant.SearchVariants)
				r.Get("/{id}", d.Variant.GetVariantDetails)
				r.Get("/{id}/patients", d.Variant.CohortByVariant)
			})
		})
	})

	return r
}
