package routes

import (
	"net/http"
	join_requests "wireport/internal/join-requests"

	"gorm.io/gorm"
)

func Router(db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	join_requests.RegisterRoutes(mux, db)

	return mux
}
