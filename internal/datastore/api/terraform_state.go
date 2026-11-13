package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"
	storageerrors "github.com/padok-team/burrito/internal/datastore/storage/error"
)

func getTerraformStateArgs(c *echo.Context) (string, string, error) {
	namespace := c.QueryParam("namespace")
	layer := c.QueryParam("layer")
	if namespace == "" || layer == "" {
		return "", "", fmt.Errorf("missing namespace or layer query parameter")
	}
	return namespace, layer, nil
}

func (a *API) authorizeTerraformStateRequest(c *echo.Context, namespace string) error {
	serviceAccount, _ := c.Get("serviceAccount").(string)
	if !strings.HasPrefix(serviceAccount, "system:serviceaccount:"+namespace+":") {
		return c.NoContent(http.StatusForbidden)
	}
	return nil
}

func (a *API) GetTerraformStateHandler(c *echo.Context) error {
	namespace, layer, err := getTerraformStateArgs(c)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if err := a.authorizeTerraformStateRequest(c, namespace); err != nil {
		return err
	}
	state, err := a.Storage.GetTerraformState(namespace, layer)
	if storageerrors.NotFound(err) {
		return c.NoContent(http.StatusNotFound)
	}
	if err != nil {
		return c.String(http.StatusInternalServerError, "could not read Terraform state")
	}
	return c.Blob(http.StatusOK, "application/json", state)
}

func (a *API) PutTerraformStateHandler(c *echo.Context) error {
	namespace, layer, err := getTerraformStateArgs(c)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if err := a.authorizeTerraformStateRequest(c, namespace); err != nil {
		return err
	}
	state, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.String(http.StatusBadRequest, "could not read Terraform state")
	}
	if !json.Valid(state) {
		return c.String(http.StatusBadRequest, "Terraform state must be valid JSON")
	}
	key := namespace + "/" + layer
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if _, ok := a.locks[key]; !ok {
		return c.String(http.StatusConflict, "Terraform state must be locked before it can be updated")
	}
	if err := a.Storage.PutTerraformState(namespace, layer, state); err != nil {
		return c.String(http.StatusInternalServerError, "could not store Terraform state")
	}
	return c.NoContent(http.StatusOK)
}

func (a *API) LockTerraformStateHandler(c *echo.Context) error {
	namespace, layer, err := getTerraformStateArgs(c)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if err := a.authorizeTerraformStateRequest(c, namespace); err != nil {
		return err
	}
	lockInfo, err := io.ReadAll(c.Request().Body)
	if err != nil || !json.Valid(lockInfo) {
		return c.String(http.StatusBadRequest, "invalid Terraform lock information")
	}
	key := namespace + "/" + layer
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	if existing, ok := a.locks[key]; ok {
		return c.Blob(http.StatusLocked, "application/json", existing)
	}
	a.locks[key] = lockInfo
	return c.Blob(http.StatusOK, "application/json", lockInfo)
}

func (a *API) GetTerraformStateLockHandler(c *echo.Context) error {
	namespace, layer, err := getTerraformStateArgs(c)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if err := a.authorizeTerraformStateRequest(c, namespace); err != nil {
		return err
	}
	key := namespace + "/" + layer
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	lockInfo, ok := a.locks[key]
	if !ok {
		return c.NoContent(http.StatusNotFound)
	}
	return c.Blob(http.StatusOK, "application/json", lockInfo)
}

func (a *API) UnlockTerraformStateHandler(c *echo.Context) error {
	namespace, layer, err := getTerraformStateArgs(c)
	if err != nil {
		return c.String(http.StatusBadRequest, err.Error())
	}
	if err := a.authorizeTerraformStateRequest(c, namespace); err != nil {
		return err
	}
	lockInfo, err := io.ReadAll(c.Request().Body)
	if err != nil || !json.Valid(lockInfo) {
		return c.String(http.StatusBadRequest, "invalid Terraform lock information")
	}
	var requested, existing struct {
		ID string `json:"ID"`
	}
	if err := json.Unmarshal(lockInfo, &requested); err != nil {
		return c.String(http.StatusBadRequest, "invalid Terraform lock information")
	}
	key := namespace + "/" + layer
	a.lockMu.Lock()
	defer a.lockMu.Unlock()
	current, ok := a.locks[key]
	if !ok {
		return c.String(http.StatusConflict, "Terraform state is not locked")
	}
	if err := json.Unmarshal(current, &existing); err != nil {
		return c.String(http.StatusInternalServerError, "stored Terraform lock information is invalid")
	}
	if requested.ID == "" || requested.ID != existing.ID {
		return c.String(http.StatusConflict, "Terraform lock ID does not match")
	}
	delete(a.locks, key)
	return c.NoContent(http.StatusOK)
}
