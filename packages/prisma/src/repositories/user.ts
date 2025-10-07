import type { PrismaClientType } from "../client";
import { prisma } from "../client";
import type { Prisma, User } from "../../generated/prisma";

export class UserRepository {
  constructor(private client: PrismaClientType = prisma) {}

  /**
   * Find user by ID
   * Automatically hides password unless includePassword = true
   */
  async findById(
    id: string,
    includePassword = false,
  ): Promise<User | Omit<User, "password"> | null> {
    return this.client.user.findUnique({
      where: { id },
      ...(includePassword ? {} : { omit: { password: true } }),
      cacheStrategy: { ttl: 60, swr: 30 },
    });
  }

  /**
   * Find user by email
   * Automatically hides password unless includePassword = true
   */
  async findByEmail(
    email: string,
    includePassword = false,
  ): Promise<User | Omit<User, "password"> | null> {
    return this.client.user.findUnique({
      where: { email },
      ...(includePassword ? {} : { omit: { password: true } }),
      cacheStrategy: { ttl: 60 },
    });
  }

  /**
   * Check if a user exists by email
   */
  async existsByEmail(email: string): Promise<boolean> {
    const user = await this.client.user.findUnique({
      where: { email },
      select: { id: true },
    });
    return !!user;
  }

  /**
   * Search users by email pattern (case-insensitive)
   */
  async searchByEmail(emailPattern: string): Promise<Omit<User, "password">[]> {
    return this.client.user.findMany({
      where: {
        email: { contains: emailPattern, mode: "insensitive" },
      },
      omit: { password: true },
      cacheStrategy: { ttl: 60 },
    });
  }

  /**
   * Filter users by custom criteria
   */
  async findUsersWithFilter(filter: Prisma.UserWhereInput): Promise<Omit<User, "password">[]> {
    return this.client.user.findMany({
      where: filter,
      orderBy: { created_at: "desc" },
      omit: { password: true },
    });
  }

  /**
   * Create a new user (password is not omitted during create)
   */
  async create(data: Prisma.UserCreateInput): Promise<User> {
    return this.client.user.create({ data });
  }

  /**
   * Create multiple users
   */
  async createMany(data: Prisma.UserCreateManyInput[]): Promise<Prisma.BatchPayload> {
    return this.client.user.createMany({ data });
  }

  /**
   * Update a user (returns user without password)
   */
  async update(id: string, data: Prisma.UserUpdateInput): Promise<Omit<User, "password">> {
    return this.client.user.update({
      where: { id },
      data,
      omit: { password: true },
    });
  }

  /**
   * Find many users (supports pagination, sorting, filtering)
   */
  async findMany(params?: {
    skip?: number;
    take?: number;
    where?: Prisma.UserWhereInput;
    orderBy?: Prisma.UserOrderByWithRelationInput;
  }): Promise<Omit<User, "password">[]> {
    return this.client.user.findMany({
      ...(params?.skip ? { skip: params.skip } : {}),
      ...(params?.take ? { take: params.take } : {}),
      ...(params?.where ? { where: params.where } : {}),
      orderBy: params?.orderBy ?? { created_at: "desc" },
      omit: { password: true },
    });
  }

  /**
   * Soft delete a single user
   */
  async softDelete(id: string): Promise<Omit<User, "password">> {
    const user = await this.client.user.update({
      where: { id },
      data: { deleted: true },
    });
    // Remove password field before returning
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const { password, ...rest } = user;
    return rest;
  }

  /**
   * Soft delete multiple users by ID
   */
  async softDeleteMany(ids: string[]): Promise<Prisma.BatchPayload> {
    return this.client.user.updateMany({
      where: { id: { in: ids } },
      data: { deleted: true },
    });
  }

  /**
   * Delete a single user
   */
  async delete(where: Prisma.UserWhereUniqueInput): Promise<User> {
    return this.client.user.delete({ where });
  }

  /**
   * Delete multiple users
   */
  async deleteMany(where: Prisma.UserWhereInput): Promise<Prisma.BatchPayload> {
    return this.client.user.deleteMany({ where });
  }

  /**
   * Transaction wrapper
   */
  async transaction<T>(callback: (tx: PrismaClientType) => Promise<T>): Promise<T> {
    return this.client.$transaction((tx) => callback(tx as unknown as PrismaClientType));
  }
}

// Singleton export
export const userRepository = new UserRepository();
