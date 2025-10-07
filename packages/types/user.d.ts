export type User = {
    name: string;
    email: string;
    password: string;
}

export interface UserData extends User {
    id: string
}