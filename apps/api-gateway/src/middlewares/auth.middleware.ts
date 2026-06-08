import type { NextFunction, Request, Response } from "express";
import jwt from "jsonwebtoken";
import createHttpError from "http-errors";
import { redisClient } from "@/redis";
import env from "@/config/dotenv";

interface JWTClaims {
  sub: string;
  email: string;
  jti: string;
  exp: number;
}

declare global {
  // eslint-disable-next-line @typescript-eslint/no-namespace
  namespace Express {
    interface Request {
      user?: { id: string; email: string; jti: string; exp: number };
    }
  }
}

const publicKey = Buffer.from(env.JWT_PUBLIC_KEY, "base64").toString("utf-8");

export async function authMiddleware(req: Request, _res: Response, next: NextFunction) {
  const auth = req.headers.authorization;
  if (!auth?.startsWith("Bearer ")) {
    return next(createHttpError(401, "Missing authorization"));
  }

  const token = auth.slice(7);
  try {
    const claims = jwt.verify(token, publicKey, { algorithms: ["RS256"] }) as JWTClaims;

    const blacklisted = await redisClient.exists(`bl:${claims.jti}`);
    if (blacklisted) return next(createHttpError(401, "Token revoked"));

    req.user = { id: claims.sub, email: claims.email, jti: claims.jti, exp: claims.exp };
    next();
  } catch {
    next(createHttpError(401, "Invalid or expired token"));
  }
}
