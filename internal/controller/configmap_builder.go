package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	gatewayv1alpha1 "github.com/TheBranchDriftCatalyst/gateway-arr/api/v1alpha1"
)

// ConfigMapBuilder builds Homepage-compatible services.yaml
type ConfigMapBuilder struct{}

// NewConfigMapBuilder creates a new ConfigMapBuilder
func NewConfigMapBuilder() *ConfigMapBuilder {
	return &ConfigMapBuilder{}
}

// slugToTitle converts a kebab-case slug to Title Case
// e.g., "media-management" -> "Media Management"
func slugToTitle(slug string) string {
	words := strings.Split(slug, "-")
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(string(word[0])) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

// ServiceEntry represents a single service in Homepage format
type ServiceEntry struct {
	Name        string                 `json:"name,omitempty"`
	Icon        string                 `json:"icon,omitempty"`
	Href        string                 `json:"href,omitempty"`
	Description string                 `json:"description,omitempty"`
	SiteMonitor string                 `json:"siteMonitor,omitempty"`
	Widget      map[string]interface{} `json:"widget,omitempty"`
}

// Build generates the services.yaml content
func (b *ConfigMapBuilder) Build(ctx context.Context, c client.Client, categories map[string][]gatewayv1alpha1.Widget) (string, error) {
	// Homepage services.yaml format is a list of category objects
	// Each category object has the category name as key and list of services as value
	result := make([]map[string][]map[string]interface{}, 0)

	// Sort categories for consistent output
	categoryNames := make([]string, 0, len(categories))
	for name := range categories {
		categoryNames = append(categoryNames, name)
	}

	for _, categorySlug := range categoryNames {
		widgets := categories[categorySlug]
		services := make([]map[string]interface{}, 0, len(widgets))

		for _, widget := range widgets {
			service, err := b.buildServiceEntry(ctx, c, widget)
			if err != nil {
				return "", fmt.Errorf("failed to build service entry for %s: %w", widget.Name, err)
			}
			services = append(services, service)
		}

		// Convert slug to human-readable display name
		displayName := slugToTitle(categorySlug)
		categoryEntry := map[string][]map[string]interface{}{
			displayName: services,
		}
		result = append(result, categoryEntry)
	}

	yamlBytes, err := yaml.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal services YAML: %w", err)
	}

	return string(yamlBytes), nil
}

func (b *ConfigMapBuilder) buildServiceEntry(ctx context.Context, c client.Client, widget gatewayv1alpha1.Widget) (map[string]interface{}, error) {
	// Each service is a map with the service name as key
	serviceData := make(map[string]interface{})

	if widget.Spec.Icon != "" {
		serviceData["icon"] = widget.Spec.Icon
	}
	if widget.Spec.Href != "" {
		serviceData["href"] = widget.Spec.Href
	}
	if widget.Spec.Description != "" {
		serviceData["description"] = widget.Spec.Description
	}

	// Site monitor - use internal URL if available, otherwise external
	if widget.Spec.SiteMonitor != nil && widget.Spec.SiteMonitor.Enabled {
		monitorURL := widget.Spec.InternalUrl
		if monitorURL == "" {
			monitorURL = widget.Spec.Href
		}
		if widget.Spec.SiteMonitor.Path != "" {
			monitorURL = strings.TrimSuffix(monitorURL, "/") + widget.Spec.SiteMonitor.Path
		}
		serviceData["siteMonitor"] = monitorURL
	}

	// Widget configuration
	if widget.Spec.Widget != nil {
		widgetConfig := make(map[string]interface{})
		widgetConfig["type"] = widget.Spec.Widget.Type

		// Use internal URL for widget API calls
		if widget.Spec.InternalUrl != "" {
			widgetConfig["url"] = widget.Spec.InternalUrl
		}

		// Handle credentials - generate HOMEPAGE_VAR references
		if widget.Spec.Widget.Credentials != nil {
			if widget.Spec.Widget.Credentials.APIKeySecretRef != nil {
				// Generate env var name from secret reference
				envVar := b.generateEnvVarName(widget.Namespace, widget.Name, "apikey")
				widgetConfig["key"] = fmt.Sprintf("{{HOMEPAGE_VAR_%s}}", envVar)
			}
			if widget.Spec.Widget.Credentials.UsernameSecretRef != nil {
				envVar := b.generateEnvVarName(widget.Namespace, widget.Name, "username")
				widgetConfig["username"] = fmt.Sprintf("{{HOMEPAGE_VAR_%s}}", envVar)
			}
			if widget.Spec.Widget.Credentials.PasswordSecretRef != nil {
				envVar := b.generateEnvVarName(widget.Namespace, widget.Name, "password")
				widgetConfig["password"] = fmt.Sprintf("{{HOMEPAGE_VAR_%s}}", envVar)
			}
		}

		if widget.Spec.Widget.EnableQueue {
			widgetConfig["enableQueue"] = true
		}

		if len(widget.Spec.Widget.Fields) > 0 {
			widgetConfig["fields"] = widget.Spec.Widget.Fields
		}

		serviceData["widget"] = widgetConfig
	}

	// Return as map with display name as key (Homepage format)
	return map[string]interface{}{
		widget.Spec.DisplayName: serviceData,
	}, nil
}

// generateEnvVarName creates a consistent env var name from widget identity
func (b *ConfigMapBuilder) generateEnvVarName(namespace, name, credType string) string {
	// Convert to uppercase and replace invalid chars
	result := strings.ToUpper(fmt.Sprintf("%s_%s_%s", namespace, name, credType))
	result = strings.ReplaceAll(result, "-", "_")
	return result
}

// ResolveSecretValue fetches a secret value (used when direct injection is needed)
func (b *ConfigMapBuilder) ResolveSecretValue(ctx context.Context, c client.Client, namespace string, ref *gatewayv1alpha1.SecretKeySelector) (string, error) {
	if ref == nil {
		return "", nil
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, types.NamespacedName{
		Namespace: namespace,
		Name:      ref.Name,
	}, secret); err != nil {
		return "", fmt.Errorf("failed to get secret %s/%s: %w", namespace, ref.Name, err)
	}

	value, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s/%s", ref.Key, namespace, ref.Name)
	}

	return string(value), nil
}
