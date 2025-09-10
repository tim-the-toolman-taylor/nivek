
export interface User {
    id:        number;
    username:  string;
    role:      string;
    createdAt: number
}

export const API_URL: string = import.meta.env.API_URL ?? window.location.protocol + "//" + window.location.host + '/api'

export const API_ROUTES: object = {
    LOGIN: '/login',
    SIGNUP: '/signup',
    GET_WEATHER: '/weather'
}
