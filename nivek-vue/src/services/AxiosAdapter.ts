
import axios, { AxiosRequestConfig } from 'axios'
import { TokenManager } from '@/utils/TokenManager'
import { HttpAdapter } from './HttpClient'
import { API_URL } from '@/constants'

const axiosInstance = axios.create({
    baseURL: API_URL,
    timeout: 5000,
    withCredentials: true, // Include cookies in requests
});

const tokenManager = TokenManager.getInstance();

// Request interceptor to add JWT token
axiosInstance.interceptors.request.use(
    (config) => {
        // Add JWT token to headers
        const authHeaders = tokenManager.getAuthHeader();
        config.headers = {
            ...config.headers,
            ...authHeaders,
        };

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
        return res.data;
    },
    async post<T>(url: string, body: unknown, options?: AxiosRequestConfig): Promise<T> {
        const res: AxiosResponse<T> = await axiosInstance.post<T>(url, body, options);

        // For login endpoint, return full response with headers
        if (url === '/login') {
            return {
                data: res.data,
                headers: res.headers as Record<string, string>,
                status: res.status
            } as HttpResponse<T>;
        }

        // For other endpoints, return just data
        return res.data;
    },
    async put<T>(url: string, body: unknown, options?: AxiosRequestConfig): Promise<T> {
        const res = await axiosInstance.put<T>(url, body, options);
        return res.data;
    },
    async del<T>(url: string, options?: AxiosRequestConfig): Promise<T> {
        const res = await axiosInstance.delete<T>(url, options);
        return res.data;
    },
};
