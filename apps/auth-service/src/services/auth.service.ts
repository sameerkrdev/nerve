import bcrypt from "bcryptjs";
import createHttpError from "http-errors";
import { userRepository, type User } from "@repo/prisma";
import {
  signAccessToken,
  storeRefreshToken,
  getRefreshToken,
  deleteRefreshToken,
  invalidateUserFamily,
  blacklistJti,
  newRefreshTokenID,
} from "./token.service";

export async function register(email: string, username: string | undefined, password: string) {
  const exists = await userRepository.existsByEmail(email);
  if (exists) throw createHttpError(409, "Email already registered");

  const hashed = await bcrypt.hash(password, 12);
  let user: User;
  try {
    user = await userRepository.create({
      email,
      ...(username ? { username } : {}),
      password: hashed,
    });
  } catch (err: unknown) {
    if (err instanceof Error && "code" in err && err.code === "P2002") {
      throw createHttpError(409, "Username already taken");
    }
    throw err;
  }

  const accessToken = signAccessToken(user.id, user.email);
  const rtID = newRefreshTokenID();
  await storeRefreshToken(rtID, user.id, user.id);

  return {
    accessToken,
    refreshToken: rtID,
    user: { id: user.id, email: user.email, username: user.username },
  };
}

export async function login(email: string, password: string) {
  const user = (await userRepository.findByEmail(email, true)) as User | null;
  if (!user) throw createHttpError(401, "Invalid credentials");
  if (user.deleted) throw createHttpError(403, "Account suspended");

  const valid = await bcrypt.compare(password, user.password);
  if (!valid) throw createHttpError(401, "Invalid credentials");

  const accessToken = signAccessToken(user.id, user.email);
  const rtID = newRefreshTokenID();
  await storeRefreshToken(rtID, user.id, user.id);

  return {
    accessToken,
    refreshToken: rtID,
    user: { id: user.id, email: user.email, username: user.username },
  };
}

export async function refresh(refreshToken: string) {
  const record = await getRefreshToken(refreshToken);
  if (!record) throw createHttpError(401, "Invalid or expired refresh token");

  const { userID, family } = record;
  await deleteRefreshToken(refreshToken, userID);

  const user = await userRepository.findById(userID);
  if (!user || user.deleted) {
    await invalidateUserFamily(userID);
    throw createHttpError(401, "User not found or suspended");
  }

  const accessToken = signAccessToken(user.id, user.email);
  const newRtID = newRefreshTokenID();
  await storeRefreshToken(newRtID, userID, family);

  return { accessToken, refreshToken: newRtID };
}

export async function logout(jti: string, exp: number, refreshToken: string, userID: string) {
  await Promise.all([blacklistJti(jti, exp), deleteRefreshToken(refreshToken, userID)]);
}

export async function me(userID: string) {
  const user = await userRepository.findById(userID);
  if (!user) throw createHttpError(404, "User not found");
  return user;
}
