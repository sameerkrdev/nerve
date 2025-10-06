import { PrismaClient } from "../generated/prisma";
import { withAccelerate } from "@prisma/extension-accelerate";

declare global {
  var prisma: ReturnType<typeof createPrismaClient> | undefined;
}

function createPrismaClient() {
  return new PrismaClient({
    log: process.env.NODE_ENV === "development" ? ["query", "error", "warn"] : ["error"],
  }).$extends(withAccelerate());
}

export const prisma = global.prisma ?? createPrismaClient();

// Export the type
export type PrismaClientType = typeof prisma;

if (process.env.NODE_ENV !== "production") {
  global.prisma = prisma;
}
