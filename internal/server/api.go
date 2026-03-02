package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gatewayv1alpha1 "github.com/TheBranchDriftCatalyst/gateway-arr/api/v1alpha1"
)

// APIServer provides REST API for widget data
type APIServer struct {
	client   client.Client
	addr     string
	upgrader websocket.Upgrader

	// WebSocket clients
	clients   map[*websocket.Conn]bool
	clientsMu sync.RWMutex
}

// WidgetResponse is the API response format for widgets
type WidgetResponse struct {
	Name        string                 `json:"name"`
	Namespace   string                 `json:"namespace"`
	DisplayName string                 `json:"displayName"`
	Description string                 `json:"description,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Href        string                 `json:"href"`
	InternalUrl string                 `json:"internalUrl,omitempty"`
	Category    string                 `json:"category,omitempty"`
	Order       string                 `json:"order,omitempty"`
	Healthy     bool                   `json:"healthy"`
	LastChecked *time.Time             `json:"lastChecked,omitempty"`
	Widget      map[string]interface{} `json:"widget,omitempty"`
	Nav         *NavResponse           `json:"nav,omitempty"`
}

// NavResponse is the nav configuration in API format
type NavResponse struct {
	ShowInOverlay bool   `json:"showInOverlay"`
	Shortcut      string `json:"shortcut,omitempty"`
}

// NewAPIServer creates a new API server
func NewAPIServer(c client.Client, addr string) *APIServer {
	return &APIServer{
		client:  c,
		addr:    addr,
		clients: make(map[*websocket.Conn]bool),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for dev
			},
		},
	}
}

// Start begins serving the API
func (s *APIServer) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware())

	// API routes
	api := r.Group("/api")
	{
		api.GET("/widgets", s.listWidgets)
		api.GET("/widgets/:namespace/:name", s.getWidget)
		api.GET("/health", s.healthCheck)
	}

	// WebSocket for real-time updates
	r.GET("/ws", s.handleWebSocket)

	return r.Run(s.addr)
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (s *APIServer) listWidgets(c *gin.Context) {
	ctx := c.Request.Context()

	var widgetList gatewayv1alpha1.WidgetList
	if err := s.client.List(ctx, &widgetList); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Group by category if requested
	groupBy := c.Query("groupBy")
	if groupBy == "category" {
		grouped := make(map[string][]WidgetResponse)
		for _, w := range widgetList.Items {
			resp := s.widgetToResponse(w)
			category := resp.Category
			if category == "" {
				category = "Services"
			}
			grouped[category] = append(grouped[category], resp)
		}
		c.JSON(http.StatusOK, grouped)
		return
	}

	// Return flat list
	responses := make([]WidgetResponse, 0, len(widgetList.Items))
	for _, w := range widgetList.Items {
		responses = append(responses, s.widgetToResponse(w))
	}
	c.JSON(http.StatusOK, responses)
}

func (s *APIServer) getWidget(c *gin.Context) {
	ctx := c.Request.Context()
	namespace := c.Param("namespace")
	name := c.Param("name")

	var widget gatewayv1alpha1.Widget
	if err := s.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, &widget); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "widget not found"})
		return
	}

	c.JSON(http.StatusOK, s.widgetToResponse(widget))
}

func (s *APIServer) healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (s *APIServer) widgetToResponse(w gatewayv1alpha1.Widget) WidgetResponse {
	resp := WidgetResponse{
		Name:        w.Name,
		Namespace:   w.Namespace,
		DisplayName: w.Spec.DisplayName,
		Description: w.Spec.Description,
		Icon:        w.Spec.Icon,
		Href:        w.Spec.Href,
		InternalUrl: w.Spec.InternalUrl,
		Category:    w.Labels["gateway.catalyst.io/category"],
		Order:       w.Labels["gateway.catalyst.io/order"],
		Healthy:     w.Status.Healthy,
	}

	if w.Status.LastChecked != nil {
		t := w.Status.LastChecked.Time
		resp.LastChecked = &t
	}

	if w.Spec.Widget != nil {
		resp.Widget = map[string]interface{}{
			"type":        w.Spec.Widget.Type,
			"enableQueue": w.Spec.Widget.EnableQueue,
			"fields":      w.Spec.Widget.Fields,
		}
	}

	if w.Spec.Nav != nil {
		resp.Nav = &NavResponse{
			ShowInOverlay: w.Spec.Nav.ShowInOverlay,
			Shortcut:      w.Spec.Nav.Shortcut,
		}
	}

	return resp
}

func (s *APIServer) handleWebSocket(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	s.clientsMu.Lock()
	s.clients[conn] = true
	s.clientsMu.Unlock()

	defer func() {
		s.clientsMu.Lock()
		delete(s.clients, conn)
		s.clientsMu.Unlock()
	}()

	// Send initial widget list
	s.sendWidgetUpdate(conn)

	// Keep connection alive and handle pings
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (s *APIServer) sendWidgetUpdate(conn *websocket.Conn) {
	ctx := context.Background()

	var widgetList gatewayv1alpha1.WidgetList
	if err := s.client.List(ctx, &widgetList); err != nil {
		return
	}

	responses := make([]WidgetResponse, 0, len(widgetList.Items))
	for _, w := range widgetList.Items {
		responses = append(responses, s.widgetToResponse(w))
	}

	conn.WriteJSON(map[string]interface{}{
		"type":    "widgets",
		"widgets": responses,
	})
}

// BroadcastUpdate sends widget updates to all connected clients
func (s *APIServer) BroadcastUpdate() {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()

	for conn := range s.clients {
		s.sendWidgetUpdate(conn)
	}
}
