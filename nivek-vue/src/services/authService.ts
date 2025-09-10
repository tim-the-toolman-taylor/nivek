import { API_ROUTES } from "@/constants";

export class AuthService {
    constructor(private httpClient: HttpClient) {}

    async login(credentials: LoginCredentials): Promise<{ token: string; user: any }> {
        try {
            // Post login request
            const response = await this.httpClient.post<LoginResponse>(
                API_ROUTES.LOGIN,
                credentials
            ) as HttpResponse<LoginResponse>;

            // Extract JWT token from Authorization header
            const authHeader = response.headers.get('Authorization');
            if (!authHeader || !authHeader.startsWith('Bearer ')) {
                throw new Error('No JWT token received in response');
            }

            const token = authHeader.substring(7); // Remove 'Bearer ' prefix

            return {
                token,
                user: response.data.user
            };

        } catch (error) {
            console.error('Login failed:', error);
            throw error;
        }
    }
}
