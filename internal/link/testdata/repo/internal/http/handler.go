package http

import "net/http"

// Handler holds HTTP handlers.
type Handler struct{}

// ReserveProduct is a real HTTP handler (the AST gate must confirm this shape).
func (h *Handler) ReserveProduct(w http.ResponseWriter, r *http.Request) {}
