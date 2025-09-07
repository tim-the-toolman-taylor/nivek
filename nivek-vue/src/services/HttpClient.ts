
export interface HttpAdapter {
    get<T>(url: string, options?: unknown): Promise<T>;
    post<T>(url: string, body: unknown, options?: unknown): Promise<T>;
    put<T>(url: string, body: unknown, options?: unknown): Promise<T>;
    del<T>(url: string, options?: unknown): Promise<T>;
}

export interface HttpClient extends HttpAdapter {}

let instance: HttpClient | null = null;

export function createHttpClient(adapter: HttpAdapter): HttpClient {
    if (instance) return instance; // enforce singleton

    function handleError(err: unknown): never {
        console.error("HTTP Client Error:", err);
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
