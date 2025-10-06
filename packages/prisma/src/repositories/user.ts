import type { PrismaClientType } from "../client";
import { prisma } from "../client";
import type { Prisma } from "../../generated/prisma";

export class UsersRepository {
  constructor(private client: PrismaClientType = prisma) {}

  async findById(id: string) {
    return this.client.user.findUnique({
      where: { id },
      cacheStrategy: { ttl: 60, swr: 30 }, // Accelerate caching
    });
  }

  async searchByEmail(emailPattern: string) {
    return this.client.user.findMany({
      where: {
        email: {
          contains: emailPattern,
          mode: "insensitive",
        },
      },
      cacheStrategy: { ttl: 60 },
    });
  }

  async findUsersWithFilter(filter: Prisma.UserWhereInput) {
    return this.client.user.findMany({
      where: filter,
    });
  }

  async create(data: Prisma.UserCreateInput) {
    return this.client.user.create({ data });
  }

  async update(id: string, data: Prisma.UserUpdateInput) {
    return this.client.user.update({
      where: { id },
      data,
    });
  }

  async delete(id: string) {
    return this.client.user.delete({ where: { id } });
  }

  async findMany(params?: {
    skip?: number;
    take?: number;
    where?: Prisma.UserWhereInput;
    orderBy?: Prisma.UserOrderByWithRelationInput;
  }) {
    return this.client.user.findMany(params);
  }
}

// Export singleton instance
export const usersRepository = new UsersRepository();
