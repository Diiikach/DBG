package httpapi

import "net/http"

func NewRouter(app *App) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", app.healthHandler)
	mux.HandleFunc("/api/auth/register", app.authRegisterHandler)
	mux.HandleFunc("/api/auth/login", app.authLoginHandler)
	mux.Handle("/api/auth/me", app.authMiddleware(http.HandlerFunc(app.authMeHandler)))
	mux.Handle("/api/auth/logout", app.authMiddleware(http.HandlerFunc(app.authLogoutHandler)))

	mux.Handle("/api/patients", app.authMiddleware(http.HandlerFunc(app.patientsHandler)))
	mux.Handle("/api/variants", app.authMiddleware(http.HandlerFunc(app.variantsHandler)))
	mux.Handle("/api/patient-variants", app.authMiddleware(http.HandlerFunc(app.patientVariantsHandler)))
	mux.Handle("/api/variant-annotations", app.authMiddleware(http.HandlerFunc(app.variantAnnotationsHandler)))
	mux.Handle("/api/variants/similar", app.authMiddleware(http.HandlerFunc(app.similarVariantsHandler)))
	mux.Handle("/api/variants/enrich", app.authMiddleware(http.HandlerFunc(app.enrichVariantHandler)))
	mux.Handle("/api/variants/clinical", app.authMiddleware(http.HandlerFunc(app.variantClinicalHandler)))
	mux.Handle("/api/external/variants", app.authMiddleware(http.HandlerFunc(app.externalVariantsHandler)))
	mux.Handle("/api/export/patients", app.authMiddleware(http.HandlerFunc(app.exportPatientsHandler)))
	mux.Handle("/api/export/variants", app.authMiddleware(http.HandlerFunc(app.exportVariantsHandler)))
	mux.Handle("/api/export/patient-variants", app.authMiddleware(http.HandlerFunc(app.exportPatientVariantsHandler)))
	mux.Handle("/api/upload/vcf", app.authMiddleware(http.HandlerFunc(app.uploadVCFHandler)))
	mux.Handle("/api/upload/csv", app.authMiddleware(http.HandlerFunc(app.uploadCSVHandler)))
	mux.Handle("/api/upload/fastq", app.authMiddleware(http.HandlerFunc(app.uploadFASTQHandler)))
	mux.Handle("/api/jobs", app.authMiddleware(http.HandlerFunc(app.jobsHandler)))

	return mux
}
