import { TokenManager } from "@/utils/TokenManager"
import { API_ROUTES } from "@/constants";

interface LoginCredentials {
    email: string;
    password: string;
}

interface LoginResponse {
    user: {
        id: number;
        username: string;
        email: string;
    };
    message: string;
}

export class AuthService {
    constructor(private httpClient: HttpClient) {}

    async login(credentials: LoginCredentials): Promise<{ token: string; user: any }> {
        try {
            // Post login request
            const response = await this.httpClient.post<LoginResponse>(
                API_ROUTES.Login,
                credentials
            ) as HttpResponse<LoginResponse>;

            // Token is automatically extracted and stored by the adapter
            const tokenManager = TokenManager.getInstance();
            const token = tokenManager.getToken();

            if (!token) {
                throw new Error('No JWT token received in response');
            }

            return {
                token,
                user: response.data
            };
        } catch (error) {
            console.error('Login failed:', error);
            throw error;
        }
    }

    logout(): void {
        TokenManager.getInstance().clearToken();
        // Optionally call logout endpoint
        // this.httpClient.post('/logout', {}).catch(() => {
            // Ignore logout endpoint errors
        // });
    }

    isAuthenticated(): boolean {
        return !!TokenManager.getInstance().getToken();
    }

    getToken(): string | null {
        return TokenManager.getInstance().getToken();
    }
}
