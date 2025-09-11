
export interface HttpResponse<T> {
    data: T;
    headers: Record<string, string>;
    status: number;
}

export interface HttpAdapter {
    get<T>(url: string, options?: unknown): Promise<T>;
    post<T>(url: string, body: unknown, options?: unknown): Promise<T | HttpResponse<T>>;
    put<T>(url: string, body: unknown, options?: unknown): Promise<T>;
    del<T>(url: string, options?: unknown): Promise<T>;
}

export interface HttpClient extends HttpAdapter {}
