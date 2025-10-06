import type { UserRepository } from "@repo/prisma";
import type { User } from "@repo/types";

export default class UserService {
  constructor(private userRepo: UserRepository) {}

  async createUser(user: User) {
    const existing = await this.userRepo.findUsersWithFilter({ email: user.email });
    if (existing.length > 0) throw new Error("User already exists");
    return this.userRepo.create(user);
  }

  async getUserById(id: string) {
    return this.userRepo.findById(id);
  }

  async updateUser(id: string, data: Partial<User>) {
    const existing = await this.userRepo.findById(id);
    if (!existing) throw new Error("User not found");
    return this.userRepo.update(id, data);
  }

  async deleteUser(id: string) {
    const existing = await this.userRepo.findById(id);
    if (!existing) throw new Error("User not found");
    return this.userRepo.delete(id);
  }

  async listUsers(params: { skip?: number; take?: number }) {
    return this.userRepo.findMany(params);
  }
}
