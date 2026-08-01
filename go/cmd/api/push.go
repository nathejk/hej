package main

import "net/http"

// pushPublicKeyHandler returns the Web Push VAPID public key the client needs to
// create a push subscription. Empty when push is not configured.
//
// @Summary      Web Push public key
// @Description  Returns the VAPID public key for creating a push subscription (empty when push is not configured).
// @Tags         push
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /push/public-key [get]
func (app *application) pushPublicKeyHandler(w http.ResponseWriter, r *http.Request) {
	if err := app.WriteJSON(w, http.StatusOK, map[string]string{"public_key": app.config.vapidPublicKey}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
