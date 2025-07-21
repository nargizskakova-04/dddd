// internal/adapters/handler/http/v1/modeHandler.go
package v1

import (
	"encoding/json"
	"net/http"

	"cryptomarket/internal/core/port"
)

type ModeHandler struct {
	modeService port.ModeService
}

type ModeResponse struct {
	Mode    string `json:"mode"`
	Message string `json:"message"`
}

func NewModeHandler(modeService port.ModeService) *ModeHandler {
	return &ModeHandler{
		modeService: modeService,
	}
}

func (h *ModeHandler) SwitchToTestMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.modeService == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "mode service unavailable")
		return
	}

	if err := h.modeService.SwitchToTestMode(r.Context()); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to switch to test mode: "+err.Error())
		return
	}

	response := ModeResponse{
		Mode:    "test",
		Message: "Successfully switched to test mode",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *ModeHandler) SwitchToLiveMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.modeService == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "mode service unavailable")
		return
	}

	if err := h.modeService.SwitchToLiveMode(r.Context()); err != nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "failed to switch to live mode: "+err.Error())
		return
	}

	response := ModeResponse{
		Mode:    "live",
		Message: "Successfully switched to live mode",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *ModeHandler) GetCurrentMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if h.modeService == nil {
		h.writeErrorResponse(w, http.StatusInternalServerError, "mode service unavailable")
		return
	}

	currentMode := h.modeService.GetCurrentMode()

	response := ModeResponse{
		Mode:    currentMode,
		Message: "Current mode retrieved successfully",
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

func (h *ModeHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal_error","message":"failed to encode response"}`))
	}
}

func (h *ModeHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorType := "bad_request"
	switch statusCode {
	case http.StatusNotFound:
		errorType = "not_found"
	case http.StatusInternalServerError:
		errorType = "internal_error"
	case http.StatusMethodNotAllowed:
		errorType = "method_not_allowed"
	}

	response := ErrorResponse{
		Error:   errorType,
		Message: message,
	}

	h.writeJSONResponse(w, statusCode, response)
}
