package main

import "net/http"

// runtimeConfigResponse is the client-visible configuration the SPA fetches at
// startup. Everything here is public by definition: it is handed to any browser
// that asks. Never add a value that must stay server-side.
type runtimeConfigResponse struct {
	// DataforsyningenToken is the quota key for the WMS base layers. Empty when
	// unconfigured, which the map surfaces as a notice rather than showing grey
	// tiles.
	DataforsyningenToken string `json:"dataforsyningen_token"`
}

// runtimeConfigHandler serves configuration the SPA needs but must not have
// baked into its bundle at build time. Vite inlines import.meta.env at build
// time, so anything supplied that way would be frozen into the image; serving it
// here means one published image runs in any environment. Same reasoning as
// /api/push/public-key.
//
// @Summary      Runtime client configuration
// @Description  Public configuration values the SPA reads at startup instead of having them baked into the bundle at build time.
// @Tags         config
// @Produce      json
// @Success      200  {object}  runtimeConfigResponse
// @Router       /config [get]
func (app *application) runtimeConfigHandler(w http.ResponseWriter, r *http.Request) {
	cfg := runtimeConfigResponse{
		DataforsyningenToken: app.config.dataforsyningenToken,
	}
	if err := app.WriteJSON(w, http.StatusOK, cfg, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
