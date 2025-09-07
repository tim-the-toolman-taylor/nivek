
import axios, { AxiosRequestConfig } from 'axios'
import { HttpAdapter } from './HttpClient'
import { API_URL } from '@/constants'

const axiosInstance = axios.create({
    baseURL: API_URL,
    timeout: 5000,
});

export const AxiosAdapter: HttpAdapter = {
    async get<T>(url: string, options?: AxiosRequestConfig): Promise<T> {
        const res = await axiosInstance.get<T>(url, options);
        return res.data;
    },
    async post<T>(url: string, body: unknown, options?: AxiosRequestConfig): Promise<T> {
        const res = await axiosInstance.post<T>(url, body, options);
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
