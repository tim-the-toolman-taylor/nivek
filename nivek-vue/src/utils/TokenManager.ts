
export class TokenManager {
    private static instance: TokenManager;
    private token: string | null = null;

    private constructor() {
        this.loadToken();
    }

    static getInstance(): TokenManager {
        if (!TokenManager.instance) {
            TokenManager.instance = new TokenManager();
        }
        return TokenManager.instance;
    }

    private loadToken(): void {
        // Try to get token from cookie first, then localStorage
        this.token = this.getTokenFromCookie() || localStorage.getItem('jwt_token');
    }

    private getTokenFromCookie(): string | null {
        if (typeof document === 'undefined') return null;

        const cookies = document.cookie.split(';');
        for (let cookie of cookies) {
            const [name, value] = cookie.trim().split('=');
            if (name === 'jwt_token') {
                return decodeURIComponent(value);
            }
        }
        return null;
    }

    setToken(token: string): void {
        this.token = token;

        // Store in localStorage
        localStorage.setItem('jwt_token', token);

        // Store in secure cookie
        // this.setSecureCookie('jwt_token', token);
    }

    private setSecureCookie(name: string, value: string): void {
        if (typeof document === 'undefined') return;

        const expires = new Date();
        expires.setTime(expires.getTime() + (24 * 60 * 60 * 1000)); // 24 hours

        document.cookie = `${name}=${encodeURIComponent(value)}; ` +
            `expires=${expires.toUTCString()}; ` +
            `path=/; ` +
            `secure; ` +
            `httpOnly=false; ` + // Set to false so JS can access it
            `samesite=strict`;
    }

    getToken(): string | null {
        if (!this.token) {
            this.loadToken();
        }
        return this.token;
    }

    clearToken(): void {
        this.token = null;
        localStorage.removeItem('jwt_token');

        // Clear cookie
        if (typeof document !== 'undefined') {
            document.cookie = 'jwt_token=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/; secure; samesite=strict';
        }
    }

    getAuthHeader(): Record<string, string> {
        const token = this.getToken();
        return token ? { 'Authorization': `Bearer ${token}` } : {};
    }
}
