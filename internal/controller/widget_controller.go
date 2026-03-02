package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	gatewayv1alpha1 "github.com/TheBranchDriftCatalyst/gateway-arr/api/v1alpha1"
)

// WidgetReconciler reconciles a Widget object
type WidgetReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	TargetNamespace string
	ConfigMapName   string
}

// +kubebuilder:rbac:groups=gateway.catalyst.io,resources=widgets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.catalyst.io,resources=widgets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=gateway.catalyst.io,resources=widgets/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *WidgetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling Widget", "name", req.Name, "namespace", req.Namespace)

	// List all widgets across all namespaces
	var widgetList gatewayv1alpha1.WidgetList
	if err := r.List(ctx, &widgetList); err != nil {
		logger.Error(err, "unable to list widgets")
		return ctrl.Result{}, err
	}

	// Build Homepage services YAML
	servicesYAML, err := r.buildServicesYAML(ctx, widgetList.Items)
	if err != nil {
		logger.Error(err, "unable to build services YAML")
		return ctrl.Result{}, err
	}

	// Create or update ConfigMap
	if err := r.reconcileConfigMap(ctx, servicesYAML); err != nil {
		logger.Error(err, "unable to reconcile ConfigMap")
		return ctrl.Result{}, err
	}

	// Update widget status
	for _, widget := range widgetList.Items {
		if err := r.updateWidgetStatus(ctx, &widget); err != nil {
			logger.Error(err, "unable to update widget status", "widget", widget.Name)
		}
	}

	// Requeue to check health periodically
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *WidgetReconciler) buildServicesYAML(ctx context.Context, widgets []gatewayv1alpha1.Widget) (string, error) {
	// Group widgets by category
	categories := make(map[string][]gatewayv1alpha1.Widget)
	for _, w := range widgets {
		category := w.Labels["gateway.catalyst.io/category"]
		if category == "" {
			category = "Services"
		}
		categories[category] = append(categories[category], w)
	}

	// Sort widgets within each category by order label
	for cat := range categories {
		sort.Slice(categories[cat], func(i, j int) bool {
			orderI := categories[cat][i].Labels["gateway.catalyst.io/order"]
			orderJ := categories[cat][j].Labels["gateway.catalyst.io/order"]
			return orderI < orderJ
		})
	}

	// Build YAML structure
	builder := NewConfigMapBuilder()
	return builder.Build(ctx, r.Client, categories)
}

func (r *WidgetReconciler) reconcileConfigMap(ctx context.Context, servicesYAML string) error {
	logger := log.FromContext(ctx)

	cm := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: r.TargetNamespace,
		Name:      r.ConfigMapName,
	}, cm)

	if errors.IsNotFound(err) {
		// Create new ConfigMap
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.ConfigMapName,
				Namespace: r.TargetNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "gateway-arr",
				},
			},
			Data: map[string]string{
				"services.yaml": servicesYAML,
			},
		}
		logger.Info("Creating ConfigMap", "namespace", r.TargetNamespace, "name", r.ConfigMapName)
		return r.Create(ctx, cm)
	} else if err != nil {
		return err
	}

	// Update existing ConfigMap
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data["services.yaml"] = servicesYAML
	logger.Info("Updating ConfigMap", "namespace", r.TargetNamespace, "name", r.ConfigMapName)
	return r.Update(ctx, cm)
}

func (r *WidgetReconciler) updateWidgetStatus(ctx context.Context, widget *gatewayv1alpha1.Widget) error {
	now := metav1.Now()
	widget.Status.LastSynced = &now
	widget.Status.HomepageSynced = true

	// TODO: Implement actual health checking
	widget.Status.Healthy = true
	widget.Status.LastChecked = &now

	return r.Status().Update(ctx, widget)
}

func (r *WidgetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&gatewayv1alpha1.Widget{}).
		Complete(r)
}

// GetWidgets returns all widgets (used by API server)
func (r *WidgetReconciler) GetWidgets(ctx context.Context) ([]gatewayv1alpha1.Widget, error) {
	var widgetList gatewayv1alpha1.WidgetList
	if err := r.List(ctx, &widgetList); err != nil {
		return nil, fmt.Errorf("failed to list widgets: %w", err)
	}
	return widgetList.Items, nil
}
