package main

import (
	"errors"
	"net/http"
	"time"

	"nathejk.dk/internal/push"
)

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

type pushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// createPushSubscriptionHandler stores the caller's Web Push subscription,
// associated with their session user. Idempotent per (user, endpoint). Runs
// behind requireAuth.
//
// @Summary      Register a push subscription
// @Description  Stores the client's Web Push subscription for the signed-in user (delivery is a later feature).
// @Tags         push
// @Accept       json
// @Produce      json
// @Param        request  body      pushSubscriptionRequest  true  "Web Push subscription"
// @Success      201      {object}  map[string]string
// @Failure      400      {object}  map[string]string
// @Failure      401      {object}  map[string]string
// @Router       /push/subscription [post]
func (app *application) createPushSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	s, ok := contextGetSession(r)
	if !ok {
		app.AuthenticationRequiredResponse(w, r)
		return
	}

	var input pushSubscriptionRequest
	if err := app.ReadJSON(w, r, &input); err != nil {
		app.BadRequestResponse(w, r, err)
		return
	}
	if input.Endpoint == "" || input.Keys.P256dh == "" || input.Keys.Auth == "" {
		app.BadRequestResponse(w, r, errors.New("endpoint and keys (p256dh, auth) are required"))
		return
	}

	app.pushStore.Save(push.Subscription{
		UserID:    s.UserID,
		Endpoint:  input.Endpoint,
		P256dh:    input.Keys.P256dh,
		Auth:      input.Keys.Auth,
		CreatedAt: time.Now(),
	})

	if err := app.WriteJSON(w, http.StatusCreated, map[string]string{"message": "subscribed"}, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
