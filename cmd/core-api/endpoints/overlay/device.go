package overlay

import (
	"errors"
	"net/http"
	"strconv"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/nivek"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/overlayrelay"
	"github.com/tim-the-toolman-taylor/nivek/internal/libraries/utilities"
)

type createDeviceRequest struct {
	Label string `json:"label"`
}

type createDeviceResponse struct {
	Device overlayrelay.Device `json:"device"`
	// Token is returned exactly once, on creation. It is not recoverable
	// afterwards -- only its hash is stored -- so the dashboard must make the
	// streamer copy it now.
	Token string `json:"token"`
}

// deviceView is a registered device plus whether it is the one currently
// attached. One overlay per streamer is live at a time, so at most one device
// in the list is connected.
type deviceView struct {
	overlayrelay.Device
	Connected bool `json:"connected"`
}

// NewCreateDeviceEndpoint mints an overlay device token for the signed-in
// broadcaster. Minting replaces: it revokes any prior active token (one live
// overlay per streamer), so any overlay still connected on the old token is now
// using a revoked one and is dropped here, as revoke does.
func NewCreateDeviceEndpoint(svc nivek.NivekService, relay overlayrelay.Service, registry *overlayrelay.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		account, err := utilities.GetUserFromContext(c, svc.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		var request createDeviceRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
		// Count characters, not bytes: the column is VARCHAR(100) characters, so a
		// byte length would over-strictly reject valid multibyte labels (emoji,
		// non-Latin scripts).
		if utf8.RuneCountInString(request.Label) > 100 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "label must be 100 characters or fewer"})
		}

		token, device, err := relay.CreateDevice(account.Id, request.Label)
		if err != nil {
			svc.Logger().Errorf("create overlay device for user %d: %s", account.Id, err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not create device"})
		}

		// The just-minted token has not connected yet, so any live connection for
		// this user is on the token we just revoked. Drop it so the stale overlay
		// stops immediately; the streamer reconnects with the new token.
		if oldID, ok := registry.ConnectedDeviceID(account.Id); ok {
			registry.DisconnectDevice(account.Id, oldID)
		}

		return c.JSON(http.StatusCreated, createDeviceResponse{Device: device, Token: token})
	}
}

// NewListDevicesEndpoint lists the signed-in broadcaster's active devices.
func NewListDevicesEndpoint(svc nivek.NivekService, relay overlayrelay.Service, registry *overlayrelay.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		account, err := utilities.GetUserFromContext(c, svc.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		devices, err := relay.ListDevices(account.Id)
		if err != nil {
			svc.Logger().Errorf("list overlay devices for user %d: %s", account.Id, err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not list devices"})
		}

		// Which device is live comes straight off the connection registry: with
		// one instance the map is the truth, so this needs no heartbeat table and
		// no TTL to guess at. Annotate each device so the dashboard can show which
		// machine is attached, not just whether some machine is.
		connectedID, isConnected := registry.ConnectedDeviceID(account.Id)
		views := make([]deviceView, len(devices))
		for i, d := range devices {
			views[i] = deviceView{Device: d, Connected: isConnected && d.Id == connectedID}
		}

		return c.JSON(http.StatusOK, map[string]any{"devices": views})
	}
}

// NewRevokeDeviceEndpoint revokes one of the signed-in broadcaster's devices.
func NewRevokeDeviceEndpoint(svc nivek.NivekService, relay overlayrelay.Service, registry *overlayrelay.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		account, err := utilities.GetUserFromContext(c, svc.Logger())
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		deviceID, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid device id"})
		}

		// Scoped by user id, so a guessed device id belonging to someone else
		// is a 404 rather than a revocation.
		if err := relay.RevokeDevice(account.Id, deviceID); err != nil {
			if errors.Is(err, overlayrelay.ErrDeviceNotFound) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "device not found"})
			}
			svc.Logger().Errorf("revoke overlay device %d for user %d: %s", deviceID, account.Id, err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not revoke device"})
		}

		// Revocation is only enforced at the handshake, so an already-connected
		// overlay would keep receiving events on its open socket until it next
		// reconnected. Tear it down now if this is the attached device, making
		// revocation effective immediately.
		registry.DisconnectDevice(account.Id, deviceID)

		return c.NoContent(http.StatusNoContent)
	}
}
