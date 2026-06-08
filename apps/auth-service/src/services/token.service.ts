import jwt from "jsonwebtoken";
import { v4 as uuidv4 } from "uuid";
import { redisClient } from "../redis";
import env from "../config/dotenv";

export interface AccessTokenClaims {
  sub: string;
  email: string;
  jti: string;
  exp: number;
  iat: number;
}

const privateKey = Buffer.from(env.JWT_PRIVATE_KEY, "base64").toString("utf-8");
const publicKey = Buffer.from(env.JWT_PUBLIC_KEY, "base64").toString("utf-8");

export function signAccessToken(userID: string, email: string): string {
  const jti = uuidv4();
  return jwt.sign({ email, jti }, privateKey, {
    algorithm: "RS256",
    subject: userID,
    expiresIn: env.ACCESS_TOKEN_EXPIRY,
  });
}

export function verifyAccessToken(token: string): AccessTokenClaims {
  return jwt.verify(token, publicKey, { algorithms: ["RS256"] }) as AccessTokenClaims;
}

export async function storeRefreshToken(
  tokenID: string,
  userID: string,
  family: string,
): Promise<void> {
  const ttl = env.REFRESH_TOKEN_EXPIRY;
  await redisClient.setex(`rt:${tokenID}`, ttl, JSON.stringify({ userID, family }));
  await redisClient.sadd(`rt:family:${userID}`, tokenID);
  await redisClient.expire(`rt:family:${userID}`, ttl);
}

export async function getRefreshToken(
  tokenID: string,
): Promise<{ userID: string; family: string } | null> {
  const raw = await redisClient.get(`rt:${tokenID}`);
  if (!raw) return null;
  return JSON.parse(raw) as { userID: string; family: string };
}

export async function deleteRefreshToken(tokenID: string, userID: string): Promise<void> {
  await redisClient.del(`rt:${tokenID}`);
  await redisClient.srem(`rt:family:${userID}`, tokenID);
}

export async function invalidateUserFamily(userID: string): Promise<void> {
  const tokenIDs = await redisClient.smembers(`rt:family:${userID}`);
  const keys = tokenIDs.map((id) => `rt:${id}`);
  if (keys.length > 0) await redisClient.del(keys);
  await redisClient.del(`rt:family:${userID}`);
}

export async function blacklistJti(jti: string, exp: number): Promise<void> {
  const ttl = Math.max(1, exp - Math.floor(Date.now() / 1000));
  await redisClient.setex(`bl:${jti}`, ttl, "1");
}

export async function isJtiBlacklisted(jti: string): Promise<boolean> {
  return (await redisClient.exists(`bl:${jti}`)) === 1;
}

export function newRefreshTokenID(): string {
  return uuidv4();
}
