import createLogger from "utils/logger";

const logger = createLogger("gateway-arr");

// Gateway-arr API endpoint - configurable via environment variable
const GATEWAY_ARR_API = process.env.GATEWAY_ARR_API || "http://gateway-arr-api.gateway-arr.svc.cluster.local:8082";

/**
 * Fetch services from Gateway-arr operator API
 * This provides real-time Widget CRD data without requiring ConfigMap synchronization
 */
export async function servicesFromGatewayArr() {
  try {
    const response = await fetch(`${GATEWAY_ARR_API}/api/widgets?groupBy=category`, {
      headers: {
        "Accept": "application/json",
      },
      // Short timeout for local service
      signal: AbortSignal.timeout(5000),
    });

    if (!response.ok) {
      logger.warn("Gateway-arr API returned non-OK status: %d", response.status);
      return [];
    }

    const groupedWidgets = await response.json();

    // Transform Gateway-arr format to Homepage format
    const serviceGroups = Object.entries(groupedWidgets).map(([category, widgets]) => ({
      name: category,
      type: "group",
      services: widgets.map((widget, index) => ({
        name: widget.displayName,
        icon: widget.icon || undefined,
        href: widget.href,
        description: widget.description || undefined,
        siteMonitor: widget.internalUrl || widget.href,
        weight: parseInt(widget.order, 10) || (index + 1) * 100,
        type: "service",
        // Widget configuration for Homepage widgets
        widget: widget.widget ? {
          type: widget.widget.type,
          url: widget.internalUrl || widget.href,
          // Credentials are handled by Homepage's env var substitution
          // The operator generates {{HOMEPAGE_VAR_*}} references
          ...widget.widget,
        } : undefined,
        // Navigation overlay data (custom extension)
        nav: widget.nav || undefined,
      })),
      groups: [],
    }));

    logger.info("Loaded %d service groups from Gateway-arr", serviceGroups.length);
    return serviceGroups;
  } catch (error) {
    if (error.name === "TimeoutError") {
      logger.debug("Gateway-arr API request timed out - service may not be available");
    } else {
      logger.warn("Failed to fetch services from Gateway-arr: %s", error.message);
    }
    return [];
  }
}

/**
 * Check if Gateway-arr integration is enabled
 */
export function isGatewayArrEnabled() {
  return process.env.GATEWAY_ARR_ENABLED === "true" || process.env.GATEWAY_ARR_API;
}

export default servicesFromGatewayArr;
