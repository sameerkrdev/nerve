import { type UserRepository } from "@repo/prisma";
import { type User } from "@repo/types";

export default class UserService {
  constructor(private userRepo: UserRepository) {}

  async createUser({ name, email, password }: User) {
    const existing = await this.userRepo.findUsersWithFilter({ email });

    if (existing.length > 0) throw new Error("User already exists");

    return this.userRepo.create({
      name,
      email,
      password,
    });
  }
}
