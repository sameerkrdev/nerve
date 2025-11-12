export type User = {
  username: string;
  email: string;
  password: string;
};

export interface UserData extends User {
  id: string;
}
