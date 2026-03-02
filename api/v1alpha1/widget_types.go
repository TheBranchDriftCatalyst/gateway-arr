package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretKeySelector selects a key from a Secret
type SecretKeySelector struct {
	// Name of the secret
	Name string `json:"name"`
	// Key in the secret
	Key string `json:"key"`
}

// WidgetCredentials defines credential references for widget API access
type WidgetCredentials struct {
	// APIKeySecretRef references a secret containing an API key
	// +optional
	APIKeySecretRef *SecretKeySelector `json:"apiKeySecretRef,omitempty"`
	// UsernameSecretRef references a secret containing a username
	// +optional
	UsernameSecretRef *SecretKeySelector `json:"usernameSecretRef,omitempty"`
	// PasswordSecretRef references a secret containing a password
	// +optional
	PasswordSecretRef *SecretKeySelector `json:"passwordSecretRef,omitempty"`
}

// WidgetConfig defines Homepage widget configuration
type WidgetConfig struct {
	// Type is the Homepage widget type (e.g., sonarr, radarr, plex)
	Type string `json:"type"`
	// EnableQueue enables queue display for *arr widgets
	// +optional
	EnableQueue bool `json:"enableQueue,omitempty"`
	// Credentials for widget API access
	// +optional
	Credentials *WidgetCredentials `json:"credentials,omitempty"`
	// Fields specifies which widget fields to display
	// +optional
	Fields []string `json:"fields,omitempty"`
}

// SiteMonitorConfig defines health monitoring configuration
type SiteMonitorConfig struct {
	// Enabled enables site monitoring
	Enabled bool `json:"enabled"`
	// Path is the health check endpoint path (defaults to /)
	// +optional
	Path string `json:"path,omitempty"`
}

// NavConfig defines navigation overlay configuration
type NavConfig struct {
	// ShowInOverlay determines if widget appears in nav overlay
	ShowInOverlay bool `json:"showInOverlay"`
	// Shortcut is the keyboard shortcut for quick access
	// +optional
	Shortcut string `json:"shortcut,omitempty"`
}

// WidgetSpec defines the desired state of Widget
type WidgetSpec struct {
	// DisplayName is the human-readable name shown in the UI
	DisplayName string `json:"displayName"`
	// Description provides additional context about the service
	// +optional
	Description string `json:"description,omitempty"`
	// Icon is the icon filename or full URL
	// +optional
	Icon string `json:"icon,omitempty"`
	// Href is the external/ingress URL for user access
	Href string `json:"href"`
	// InternalUrl is the cluster-internal service URL
	// +optional
	InternalUrl string `json:"internalUrl,omitempty"`
	// SiteMonitor configures health monitoring
	// +optional
	SiteMonitor *SiteMonitorConfig `json:"siteMonitor,omitempty"`
	// Widget configures Homepage widget integration
	// +optional
	Widget *WidgetConfig `json:"widget,omitempty"`
	// Nav configures navigation overlay behavior
	// +optional
	Nav *NavConfig `json:"nav,omitempty"`
}

// WidgetStatus defines the observed state of Widget
type WidgetStatus struct {
	// Healthy indicates if the service is responding
	Healthy bool `json:"healthy,omitempty"`
	// LastChecked is the timestamp of the last health check
	// +optional
	LastChecked *metav1.Time `json:"lastChecked,omitempty"`
	// HomepageSynced indicates if the widget is synced to Homepage config
	HomepageSynced bool `json:"homepageSynced,omitempty"`
	// LastSynced is the timestamp of the last Homepage sync
	// +optional
	LastSynced *metav1.Time `json:"lastSynced,omitempty"`
	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,shortName=wgt
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Href",type=string,JSONPath=`.spec.href`
// +kubebuilder:printcolumn:name="Healthy",type=boolean,JSONPath=`.status.healthy`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Widget is the Schema for the widgets API
type Widget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WidgetSpec   `json:"spec,omitempty"`
	Status WidgetStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WidgetList contains a list of Widget
type WidgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Widget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Widget{}, &WidgetList{})
}
