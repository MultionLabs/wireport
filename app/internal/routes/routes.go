package routes

import (
	"net/http"
	"wireport/internal/commands"

	"gorm.io/gorm"
)

func Router(db *gorm.DB) http.Handler {
	mux := http.NewServeMux()

	commands.RegisterRoutes(mux, db)

	return mux
}
