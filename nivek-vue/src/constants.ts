
export const API_URL: string = import.meta.env.API_URL ?? window.location.protocol + "//" + window.location.host + '/api'

export const API_ROUTES: object = {
    Login: '/login',
    Signup: '/signup',

    Secure: {
        Profile: '/profile',
        Weather: '/weather',
        Tasks: {
            Create: (id: number) => `/user/${id}/task`,
            GetAll: (id: number) => `/user/${id}/task`
        }
    }
}

export interface User {
    id:        number;
    username:  string;
    role:      string;
    createdAt: number
}

export interface Task {
    id: number
    uuid: string
    user_id: number
    title: string
    description: string
    priority: string
    status: string
    expires_at: string
    completed_at: string
    created_at: string
    updated_at: string
    is_important: boolean
    position: number
    estimated_duration: string
    actual_duration: string
}
