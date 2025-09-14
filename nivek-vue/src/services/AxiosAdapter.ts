
import axios, { AxiosRequestConfig } from 'axios'
import { TokenManager } from '@/utils/TokenManager'
import { HttpAdapter } from './HttpClient'
import {API_ROUTES, API_URL} from '@/constants'

const axiosInstance = axios.create({
    baseURL: API_URL,
    timeout: 5000
});

const tokenManager = TokenManager.getInstance();

// Helper function to check if URL should include credentials
// If making a request to an external api, the request path should include http protocol
// ie: https://api.windy.com
// If making a request to this system's api, the path should just be "/user"
// if the request is just "/user", then include auth credentials
const shouldIncludeCredentials = (url: string): boolean => {
    const parts = url.split('/');
    const prefix = parts[1];

    if (!prefix) {
        return false
    }

    const formatted = prefix.charAt(0).toUpperCase() + prefix.slice(1)
    return !!formatted;
};

// Request interceptor to add JWT token
axiosInstance.interceptors.request.use(
    (config) => {
        // Set withCredentials based on URL
        config.withCredentials = shouldIncludeCredentials(config.url || '');

        // Add JWT token to headers only for API requests
        if (config.withCredentials) {
            const authHeaders = tokenManager.getAuthHeader();
            config.headers = {
                ...config.headers,
                ...authHeaders,
            };
        }

        return config;
    },
    (error) => {
        return Promise.reject(error);
    }
);

// Response interceptor to handle token refresh and errors
axiosInstance.interceptors.response.use(
    (response) => {
        // Check for new token in response headers
        const authHeader = response.headers['authorization'] || response.headers['Authorization'];
        if (authHeader && authHeader.startsWith('Bearer ')) {
            const newToken = authHeader.substring(7);
            tokenManager.setToken(newToken);
        }

        return response;
    },
    (error) => {
        // Handle 401 Unauthorized
        if (error.response?.status === 401) {
            tokenManager.clearToken();
            // Redirect to login or emit event
            window.dispatchEvent(new CustomEvent('auth:unauthorized'));
        }

        return Promise.reject(error);
    }
);

export const AxiosAdapter: HttpAdapter = {
    async get<T>(url: string, options?: AxiosRequestConfig): Promise<T> {
        const res = await axiosInstance.get<T>(url, options);
        return {
            data: res.data,
            headers: res.headers as Record<string, string>,
            status: res.status
        } as HttpResponse<T>;
    },
    async post<T>(url: string, body: unknown, options?: AxiosRequestConfig): Promise<T> {
        const res: AxiosResponse<T> = await axiosInstance.post<T>(url, body, options);

        // return full response with headers
        return {
            data: res.data,
            headers: res.headers as Record<string, string>,
            status: res.status
        } as HttpResponse<T>;
    },
    async put<T>(url: string, body: unknown, options?: AxiosRequestConfig): Promise<T> {
        const res = await axiosInstance.put<T>(url, body, options);
        return {
            data: res.data,
            headers: res.headers as Record<string, string>,
            status: res.status
        } as HttpResponse<T>;
    },
    async del<T>(url: string, options?: AxiosRequestConfig): Promise<T> {
        const res = await axiosInstance.delete<T>(url, options);
        return {
            data: res.data,
            headers: res.headers as Record<string, string>,
            status: res.status
        } as HttpResponse<T>;
    },
};
