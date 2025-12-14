package hchandler

import (
	"net/http"

	"github.com/Seann-Moser/hypr-config-manager/pkg/hyprconfig"
	"github.com/Seann-Moser/mserve"
	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	configManager hyprconfig.ConfigManager
}

func NewHandler(configManager hyprconfig.ConfigManager) (*Handler, error) {
	return &Handler{
		configManager: configManager,
	}, nil
}

func (h *Handler) GetEndpoints() []*mserve.Endpoint {
	endpoints := []*mserve.Endpoint{
		{
			Name:    "New Config",
			Handler: h.NewConfig,
			Path:    "/config/new",
			Methods: []string{http.MethodPost},
			Request: mserve.Request{
				Body: hyprconfig.HyprConfig{},
			},
			Responses: []mserve.Response{
				{
					Status:  http.StatusCreated,
					Message: "Config created successfully",
					Body:    hyprconfig.HyprConfig{},
				},
				{
					Status:  http.StatusBadRequest,
					Message: "Invalid request body",
					Body:    mserve.ErrorResponse{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to create config",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
		{
			Name:    "Search Configs",
			Handler: h.SearchConfigs,
			Path:    "/config/search",
			Methods: []string{"GET", "POST"},
			Request: mserve.Request{
				Params: map[string]mserve.ROption{
					"q": {Required: false},
				},
				Body: hyprconfig.ConfigSearchFilters{},
			},
			Responses: []mserve.Response{
				{
					Status:  http.StatusOK,
					Message: "Search results",
					Body:    mserve.Page[hyprconfig.HyprConfig]{},
				},
				{
					Status:  http.StatusBadRequest,
					Message: "Invalid request body",
					Body:    mserve.ErrorResponse{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to search configs",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
		{
			Name:    "Add Program Config",
			Path:    "/config/{config_id}/program/add",
			Handler: h.AddProgramConfig,
			Methods: []string{http.MethodPost},
			Request: mserve.Request{
				Body: hyprconfig.HyprProgramConfig{},
				Params: map[string]mserve.ROption{
					"parent_id": {Required: false},
				},
			},
			Responses: []mserve.Response{
				{
					Status:  http.StatusOK,
					Message: "Program added successfully",
					Body:    map[string]string{},
				},
				{
					Status:  http.StatusBadRequest,
					Message: "Invalid request body or parameters",
					Body:    mserve.ErrorResponse{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to add program config",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
		{
			Name:    "Remove Program Config",
			Path:    "/config/{config_id}/program/remove",
			Handler: h.RemoveProgramConfig,
			Methods: []string{http.MethodDelete},
			Request: mserve.Request{
				Params: map[string]mserve.ROption{
					"prog_id": {Required: true},
				},
			},
			Responses: []mserve.Response{
				{
					Status:  http.StatusOK,
					Message: "Program removed successfully",
					Body:    map[string]string{},
				},
				{
					Status:  http.StatusBadRequest,
					Message: "Missing prog_id",
					Body:    mserve.ErrorResponse{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to remove program",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
		{
			Name:    "Update Program Config",
			Path:    "/config/{config_id}/program/update",
			Handler: h.UpdateProgramConfig,
			Methods: []string{http.MethodPut},
			Request: mserve.Request{
				Body: hyprconfig.HyprProgramConfig{},
				Params: map[string]mserve.ROption{
					"prog_id": {Required: true},
				},
			},
			Responses: []mserve.Response{
				{
					Status:  http.StatusOK,
					Message: "Program updated successfully",
					Body:    map[string]string{},
				},
				{
					Status:  http.StatusBadRequest,
					Message: "Invalid request body or missing prog_id",
					Body:    mserve.ErrorResponse{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to update program config",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
		{
			Name:    "Move Program Config",
			Path:    "/config/{config_id}/program/move",
			Handler: h.MoveProgramConfig,
			Methods: []string{http.MethodPut},
			Request: mserve.Request{
				Params: map[string]mserve.ROption{
					"prog_id":       {Required: true},
					"new_parent_id": {Required: false},
				},
			},
			Responses: []mserve.Response{
				{
					Status:  http.StatusOK,
					Message: "Program moved successfully",
					Body:    map[string]string{},
				},
				{
					Status:  http.StatusBadRequest,
					Message: "Missing prog_id",
					Body:    mserve.ErrorResponse{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to move program",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
		{
			Name:    "List Favorites",
			Path:    "/config/favorites",
			Handler: h.ListFavorites,
			Methods: []string{http.MethodGet},
			Responses: []mserve.Response{
				{
					Status:  http.StatusOK,
					Message: "Favorites listed successfully",
					Body:    mserve.Page[hyprconfig.HyprConfig]{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to list favorites",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
		{
			Name:    "Count Users Using Config",
			Path:    "/config/{config_id}/users/count",
			Handler: h.CountUsersUsingConfig,
			Methods: []string{http.MethodGet},
			Request: mserve.Request{
				Params: map[string]mserve.ROption{
					"config_id": {Required: true},
				},
			},
			Responses: []mserve.Response{
				{
					Status:  http.StatusOK,
					Message: "Count retrieved successfully",
					Body:    map[string]int64{},
				},
				{
					Status:  http.StatusBadRequest,
					Message: "Missing config_id",
					Body:    mserve.ErrorResponse{},
				},
				{
					Status:  http.StatusInternalServerError,
					Message: "Failed to count users",
					Body:    mserve.ErrorResponse{},
				},
			},
		},
	}
	// --- Missing endpoints ---
	endpoints = append(endpoints,
		&mserve.Endpoint{
			Name:    "Get Config",
			Path:    "/config/{config_id}",
			Handler: h.GetConfig,
			Methods: []string{http.MethodGet},
			Request: mserve.Request{
				Params: map[string]mserve.ROption{
					"config_id": {Required: true},
				},
			},
			Responses: []mserve.Response{
				{Status: http.StatusOK, Message: "Config retrieved", Body: hyprconfig.HyprConfig{}},
				{Status: http.StatusBadRequest, Message: "Missing config_id", Body: mserve.ErrorResponse{}},
				{Status: http.StatusInternalServerError, Message: "Failed to get config", Body: mserve.ErrorResponse{}},
			},
		},
		&mserve.Endpoint{
			Name:    "Update Config",
			Path:    "/config/{config_id}",
			Handler: h.UpdateConfig,
			Methods: []string{http.MethodPut},
			Request: mserve.Request{
				Body: hyprconfig.HyprConfig{},
				Params: map[string]mserve.ROption{
					"config_id": {Required: true},
				},
			},
			Responses: []mserve.Response{
				{Status: http.StatusOK, Message: "Config updated", Body: map[string]string{}},
				{Status: http.StatusBadRequest, Message: "Invalid request or missing config_id", Body: mserve.ErrorResponse{}},
				{Status: http.StatusInternalServerError, Message: "Failed to update config", Body: mserve.ErrorResponse{}},
			},
		},
		&mserve.Endpoint{
			Name:    "Delete Config",
			Path:    "/config/{config_id}",
			Handler: h.DeleteConfig,
			Methods: []string{http.MethodDelete},
			Request: mserve.Request{
				Params: map[string]mserve.ROption{
					"config_id": {Required: true},
				},
			},
			Responses: []mserve.Response{
				{Status: http.StatusOK, Message: "Config deleted", Body: map[string]string{}},
				{Status: http.StatusBadRequest, Message: "Missing config_id", Body: mserve.ErrorResponse{}},
				{Status: http.StatusInternalServerError, Message: "Failed to delete config", Body: mserve.ErrorResponse{}},
			},
		},
		&mserve.Endpoint{
			Name:    "List All Configs",
			Path:    "/configs",
			Handler: h.ListConfigs,
			Methods: []string{http.MethodGet},
			Request: mserve.Request{},
			Responses: []mserve.Response{
				{Status: http.StatusOK, Message: "Configs listed", Body: mserve.Page[hyprconfig.HyprConfig]{}},
				{Status: http.StatusInternalServerError, Message: "Failed to list configs", Body: mserve.ErrorResponse{}},
			},
		},
	)
	return endpoints
}

func (h *Handler) NewConfig(w http.ResponseWriter, r *http.Request) {
	hc, err := mserve.ReadBody[hyprconfig.HyprConfig](r)
	if err != nil {
		mserve.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	created, err := h.configManager.CreateConfig(r.Context(), hc)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, created)
}

func (h *Handler) SearchConfigs(w http.ResponseWriter, r *http.Request) {
	currentPage, limit := mserve.QueryParams(r, 10)

	filter, err := mserve.ReadBody[hyprconfig.ConfigSearchFilters](r)
	if err != nil {
		mserve.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	page, err := h.configManager.ListConfigsWithFilters(r.Context(), currentPage, limit, *filter, nil)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, page)
}

func (h *Handler) ListMyConfigs(w http.ResponseWriter, r *http.Request) {
	page, limit := mserve.QueryParams(r, 10)

	result, err := h.configManager.ListMyConfigs(r.Context(), page, limit, nil)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, result)
}

func (h *Handler) FavoriteConfig(w http.ResponseWriter, r *http.Request) {
	configID := r.URL.Query().Get("config_id")
	if configID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "config_id is required")
		return
	}

	if err := h.configManager.FavoriteConfig(r.Context(), configID); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "favorited"})
}

func (h *Handler) UnfavoriteConfig(w http.ResponseWriter, r *http.Request) {
	configID := r.URL.Query().Get("config_id")
	if configID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "config_id is required")
		return
	}

	if err := h.configManager.UnfavoriteConfig(r.Context(), configID); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "unfavorited"})
}

func (h *Handler) ApplyConfig(w http.ResponseWriter, r *http.Request) {
	configID := r.URL.Query().Get("config_id")
	if configID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "config_id is required")
		return
	}

	if err := h.configManager.ApplyConfig(r.Context(), configID); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "applied"})
}

func (h *Handler) GetAppliedConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.configManager.GetAppliedConfig(r.Context())
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, cfg)
}

func (h *Handler) AddProgramConfig(w http.ResponseWriter, r *http.Request) {
	prog, err := mserve.ReadBody[hyprconfig.HyprProgramConfig](r)
	if err != nil {
		mserve.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	configID := mserve.PathParam(r, "config_id")
	parentID := mserve.QueryParam(r, "parent_id")

	var parentPtr *string
	if parentID != "" {
		parentPtr = &parentID
	}

	if err := h.configManager.AddProgramConfig(r.Context(), configID, *prog, parentPtr); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "added"})
}

func (h *Handler) RemoveProgramConfig(w http.ResponseWriter, r *http.Request) {
	configID := mserve.PathParam(r, "config_id")
	progID := mserve.QueryParam(r, "prog_id")
	if progID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "prog_id is required")
		return
	}

	if err := h.configManager.RemoveProgramConfig(r.Context(), configID, progID); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "removed"})
}

func (h *Handler) UpdateProgramConfig(w http.ResponseWriter, r *http.Request) {
	configID := mserve.PathParam(r, "config_id")
	progID := mserve.QueryParam(r, "prog_id")
	if progID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "prog_id is required")
		return
	}

	updates, err := mserve.ReadBody[hyprconfig.HyprProgramConfig](r)
	if err != nil {
		mserve.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.configManager.UpdateProgramConfig(r.Context(), configID, progID, *updates); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "updated"})
}

func (h *Handler) MoveProgramConfig(w http.ResponseWriter, r *http.Request) {
	configID := mserve.PathParam(r, "config_id")
	progID := mserve.QueryParam(r, "prog_id")
	if progID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "prog_id is required")
		return
	}

	newParentID := mserve.QueryParam(r, "new_parent_id")
	var parentPtr *string
	if newParentID != "" {
		parentPtr = &newParentID
	}

	if err := h.configManager.MoveProgramConfig(r.Context(), configID, progID, parentPtr); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "moved"})
}

func (h *Handler) ListFavorites(w http.ResponseWriter, r *http.Request) {
	page, limit := mserve.QueryParams(r, 10)

	result, err := h.configManager.ListFavorites(r.Context(), page, limit)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, result)
}

func (h *Handler) CountUsersUsingConfig(w http.ResponseWriter, r *http.Request) {
	configID := mserve.PathParam(r, "config_id")
	if configID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "config_id is required")
		return
	}

	count, err := h.configManager.CountUsersUsingConfig(r.Context(), configID)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]int64{"count": count})
}
func (h *Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	configID := mserve.PathParam(r, "config_id")
	if configID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "config_id is required")
		return
	}

	cfg, err := h.configManager.GetConfig(r.Context(), configID)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, cfg)
}

func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	configID := mserve.PathParam(r, "config_id")
	if configID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "config_id is required")
		return
	}

	// Read incoming updates
	updatesBody, err := mserve.ReadBody[hyprconfig.HyprConfig](r)
	if err != nil {
		mserve.WriteError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Fetch the existing config
	existing, err := h.configManager.GetConfig(r.Context(), configID)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// Build a bson.M with only changed fields
	updates := bson.M{}
	if updatesBody.Title != "" && updatesBody.Title != existing.Title {
		updates["title"] = updatesBody.Title
	}
	if updatesBody.Description != "" && updatesBody.Description != existing.Description {
		updates["description"] = updatesBody.Description
	}
	if len(updatesBody.ProgramConfigs) > 0 {
		updates["program_configs"] = updatesBody.ProgramConfigs
	}
	if updatesBody.Private != existing.Private {
		updates["private"] = updatesBody.Title
	}
	if len(updatesBody.Tags) > 0 && !hyprconfig.StringSlicesEqual(updatesBody.Tags, existing.Tags) {
		updates["tags"] = updatesBody.Tags
	}
	// add any other fields you want to update here...

	if len(updates) == 0 {
		mserve.WriteBody(w, r, map[string]string{"status": "no changes"})
		return
	}

	if err := h.configManager.UpdateConfig(r.Context(), configID, updates); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "updated"})
}

func (h *Handler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	configID := mserve.PathParam(r, "config_id")
	if configID == "" {
		mserve.WriteError(w, r, http.StatusBadRequest, "config_id is required")
		return
	}

	if err := h.configManager.DeleteConfig(r.Context(), configID); err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, map[string]string{"status": "deleted"})
}

func (h *Handler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	page, limit := mserve.QueryParams(r, 10)

	result, err := h.configManager.ListConfigs(r.Context(), page, limit, nil)
	if err != nil {
		mserve.WriteError(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	mserve.WriteBody(w, r, result)
}
