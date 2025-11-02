import type { UserRepository } from "@repo/prisma";
import type { User } from "@repo/types";

export default class UserService {
  constructor(private userRepo: UserRepository) {}

  async createUser(user: User) {
    const isExists = await this.userRepo.findByEmail(user.email);
    if (isExists) throw new Error("User already exists");
    return this.userRepo.create(user);
  }

  async getUserById(id: string) {
    return this.userRepo.findById(id);
  }

  async updateUser(id: string, data: Partial<User>) {
    const updated = await this.userRepo.update(id, data);
    if (!updated) throw new Error("User not found");
    return updated;
  }

  async softDeleteUser(id: string) {
    const deleted = await this.userRepo.softDelete(id);
    if (!deleted) throw new Error("User not found");
    return deleted;
  }

  async softDeleteMany(userIds: string[]) {
    if (!userIds?.length) throw new Error("No user IDs provided");
    return this.userRepo.softDeleteMany(userIds);
  }

  async hardDeleteUser(id: string) {
    const existing = await this.userRepo.findById(id);
    if (!existing) throw new Error("User not found");
    return this.userRepo.delete({ id });
  }

  async hardDeleteMany(userIds: string[]) {
    if (!userIds?.length) throw new Error("No user IDs provided");
    return this.userRepo.deleteMany({ id: { in: userIds } });
  }

  async listUsers(params: { skip?: number; take?: number }) {
    return this.userRepo.findMany(params);
  }
}
