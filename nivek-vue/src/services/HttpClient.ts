
import { TokenManager } from '@/utils/TokenManager'

let instance: HttpClient | null = null;

export function createHttpClient(adapter: HttpAdapter): HttpClient {
    if (instance) return instance; // enforce singleton

    function handleError(err: unknown): never {
        console.error("HTTP Client Error:", err);

        // Check if it's an authentication error
        if (err instanceof Error && err.message.includes('401')) {
            TokenManager.getInstance().clearToken();
            window.dispatchEvent(new CustomEvent('auth:unauthorized'));
        }

        throw err instanceof Error ? err : new Error("Unknown error");
    }

    instance = {
        async get<T>(url, options) {
            try {
                return await adapter.get<T>(url, options);
            } catch (err) {
                handleError(err);
            }
        },
        async post<T>(url, body, options) {
            try {
                return await adapter.post<T>(url, body, options);
            } catch (err) {
                handleError(err);
            }
        },
        async put<T>(url, body, options) {
            try {
                return await adapter.put<T>(url, body, options);
            } catch (err) {
                handleError(err);
            }
        },
        async del<T>(url, options) {
            try {
                return await adapter.del<T>(url, options);
            } catch (err) {
                handleError(err);
            }
        },
    };

    return instance;
}
