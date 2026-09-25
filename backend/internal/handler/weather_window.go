package handler

import (
	"net/http"

	"github.com/blueship581/port-mooring-window-safety/backend/internal/dto"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/middleware"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/service"
	"github.com/blueship581/port-mooring-window-safety/backend/internal/util"
	"github.com/gin-gonic/gin"
)

type WeatherWindowHandler struct{ service service.WeatherWindowService }

func NewWeatherWindowHandler(s service.WeatherWindowService) *WeatherWindowHandler {
	return &WeatherWindowHandler{service: s}
}

func (h *WeatherWindowHandler) Register(group *gin.RouterGroup) {
	resource := group.Group("/weather-windows")
	resource.GET("", h.list)
	resource.GET("/:id", h.get)
	resource.POST("", middleware.RequireMinimumRole("operator"), h.create)
	resource.PUT("/:id", middleware.RequireMinimumRole("operator"), h.update)
	resource.POST("/:id/transition", middleware.RequireMinimumRole("operator"), h.transition)
	resource.DELETE("/:id", middleware.RequireRoles("admin"), h.remove)
}

func (h *WeatherWindowHandler) list(c *gin.Context) {
	query := bindPage(c)
	result, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		handleError(c, err)
		return
	}
	util.Page(c, result.Items, result.Page, result.PageSize, result.Total)
}

func (h *WeatherWindowHandler) get(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *WeatherWindowHandler) create(c *gin.Context) {
	var input dto.CreateWeatherWindow
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.Created(c, item)
}

func (h *WeatherWindowHandler) update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.UpdateWeatherWindow
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *WeatherWindowHandler) transition(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var input dto.TransitionRequest
	if err := c.ShouldBindJSON(&input); err != nil {
		util.Fail(c, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}
	item, err := h.service.Transition(c.Request.Context(), id, input, actorFromContext(c), requestIDFromContext(c))
	if err != nil {
		handleError(c, err)
		return
	}
	util.OK(c, item)
}

func (h *WeatherWindowHandler) remove(c *gin.Context) {
	if roleFromContext(c) != "admin" {
		util.Fail(c, http.StatusForbidden, "forbidden", "admin role is required")
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), id, actorFromContext(c), requestIDFromContext(c)); err != nil {
		handleError(c, err)
		return
	}
	util.NoContent(c)
}
