
export interface User {
    id:        number;
    username:  string;
    role:      string;
    createdAt: number
}

export const API_URL: string = import.meta.env.API_URL ?? window.location.protocol + "//" + window.location.host + '/api'

export const API_ROUTES: object = {
    Login: '/login',
    Signup: '/signup',

    Secure: {
        Profile: '/profile',
        Weather: '/weather',
        Tasks: {
            Create: (id: number) => `/user/${id}/task`,
            GetAll: (id: number) => `/user/${id}/tasks`
        }
    }
}
