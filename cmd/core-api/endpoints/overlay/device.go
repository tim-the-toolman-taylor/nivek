package overlay

import (
	"errors"
	"net/http"
	"strconv"

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

// NewCreateDeviceEndpoint mints an overlay device token for the signed-in
// broadcaster.
func NewCreateDeviceEndpoint(svc nivek.NivekService, relay overlayrelay.Service) echo.HandlerFunc {
	return func(c echo.Context) error {
		account, err := utilities.GetUserFromContext(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		var request createDeviceRequest
		if err := c.Bind(&request); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		}
		if len(request.Label) > 100 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "label must be 100 characters or fewer"})
		}

		token, device, err := relay.CreateDevice(account.Id, request.Label)
		if err != nil {
			svc.Logger().Errorf("create overlay device for user %d: %s", account.Id, err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not create device"})
		}

		return c.JSON(http.StatusCreated, createDeviceResponse{Device: device, Token: token})
	}
}

// NewListDevicesEndpoint lists the signed-in broadcaster's active devices.
func NewListDevicesEndpoint(svc nivek.NivekService, relay overlayrelay.Service, registry *overlayrelay.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		account, err := utilities.GetUserFromContext(c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}

		devices, err := relay.ListDevices(account.Id)
		if err != nil {
			svc.Logger().Errorf("list overlay devices for user %d: %s", account.Id, err.Error())
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not list devices"})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"devices": devices,
			// Read straight off the connection registry. With one instance the
			// map is the truth, so this needs no heartbeat table and no TTL to
			// guess at.
			"connected": registry.IsConnected(account.Id),
		})
	}
}

// NewRevokeDeviceEndpoint revokes one of the signed-in broadcaster's devices.
func NewRevokeDeviceEndpoint(svc nivek.NivekService, relay overlayrelay.Service) echo.HandlerFunc {
	return func(c echo.Context) error {
		account, err := utilities.GetUserFromContext(c)
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

		return c.NoContent(http.StatusNoContent)
	}
}
