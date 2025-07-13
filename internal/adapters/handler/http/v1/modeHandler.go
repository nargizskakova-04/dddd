package v1

import (
	"net/http"

	"crypto/internal/core/port"
)

type ModeHandler struct {
	modeService port.ModeService
}

func NewModeHandler(
	modeService port.ModeService,
) *ModeHandler {
	return &ModeHandler{
		modeService: modeService,
	}
}

func (h *ModeHandler) SwitchToTestMode(w http.ResponseWriter, r *http.Request) {
}

func (h *ModeHandler) SwitchToLiveMode(w http.ResponseWriter, r *http.Request) {
}

func (h *ModeHandler) GetCurrentMode(w http.ResponseWriter, r *http.Request) {
}
