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

	// ShowBuildId toggles the diagnostic build id overlaid on the client's bottom
	// nav. Public and harmless: the build id is in the bundle either way, this only
	// decides whether it is drawn.
	ShowBuildId bool `json:"show_build_id"`

	// ShowLayoutDebug toggles the client's viewport/safe-area diagnostic overlay.
	// Off by default; see config.showLayoutDebug for why this is not a URL parameter.
	ShowLayoutDebug bool `json:"show_layout_debug"`
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
		ShowBuildId:          app.config.showBuildId,
		ShowLayoutDebug:      app.config.showLayoutDebug,
	}
	if err := app.WriteJSON(w, http.StatusOK, cfg, nil); err != nil {
		app.ServerErrorResponse(w, r, err)
	}
}
