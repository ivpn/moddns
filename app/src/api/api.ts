import axios from "axios";
import * as client from "@/api/client";

const axiosClient = axios.create({
  baseURL: import.meta.env.VITE_API_URL + "/api/v1",
});

// Instance-specific interceptor (not global)
axiosClient.interceptors.response.use(
  response => response,
  error => {
    const status = error.response?.status;
    
    switch (status) {
      case 401:
        // Handle unauthorized
        if (!isAuthRoute()) {
          window.dispatchEvent(new CustomEvent('auth:logout'));
        }
        break;
      case 403:
        // Handle forbidden - emit custom event
        window.dispatchEvent(new CustomEvent('api:forbidden', { 
          detail: { error, message: 'Access denied' }
        }));
        break;
      case 404:
        // Handle not found - for account/profile routes, treat as session expiry
        if (!isAuthRoute() && isAccountOrProfileRoute()) {
          window.dispatchEvent(new CustomEvent('auth:logout'));
        } else {
          window.dispatchEvent(new CustomEvent('api:notfound', { 
            detail: { error, message: 'Resource not found' }
          }));
        }
        break;
      case 500:
      case 502:
      case 503:
        // Handle server errors
        window.dispatchEvent(new CustomEvent('api:servererror', { 
          detail: { error, message: 'Server error occurred' }
        }));
        break;
    }

    return Promise.reject(error);
  }
);

function isAuthRoute(): boolean {
  const path = window.location.pathname;
  return path.includes("/login") || path.includes("/reset-password");
}

function isAccountOrProfileRoute(): boolean {
  const url = window.location.href;
  return url.includes("/accounts/current") || url.includes("/profiles");
}

const config = new client.Configuration({
  basePath: import.meta.env.VITE_API_URL,
  baseOptions: {
    axios: axiosClient,
    withCredentials: true,
  },
});

const Client = {
  authApi: new client.AuthenticationApi(config),
  accountsApi: new client.AccountApi(config),
  profilesApi: new client.ProfileApi(config),
  queryLogsApi: new client.QueryLogsApi(config),
  blocklistsApi: new client.BlocklistsApi(config),
  servicesApi: new client.ServicesApi(config),
  verificationApi: new client.VerificationApi(config),
  appleMobileconfigApi: new client.AppleMobileconfigApi(config),
  dnsStampsApi: new client.DNSStampsApi(config),
  sessionsApi: new client.SessionsApi(config),
  subscriptionApi: new client.SubscriptionApi(config),
  paSessionApi: new client.PASessionApi(config),
};

function clearSession() {
  // Clear any local authentication state if needed
  // This function is exported for compatibility but
  // authentication is now handled by AuthContext
}

export { clearSession };
export default { Client };
