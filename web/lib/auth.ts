import { SignJWT, jwtVerify } from "jose";

const COOKIE_NAME = "govscout_session";

function getSecret() {
  const secret = process.env.AUTH_SECRET;
  if (!secret) throw new Error("AUTH_SECRET environment variable is required");
  return new TextEncoder().encode(secret);
}

export function getSessionCookieName() {
  return COOKIE_NAME;
}

export async function createSessionToken(username: string): Promise<string> {
  return new SignJWT({ username })
    .setProtectedHeader({ alg: "HS256" })
    .setIssuedAt()
    .setExpirationTime("24h")
    .sign(getSecret());
}

export async function verifySessionToken(
  token: string
): Promise<string | null> {
  try {
    const { payload } = await jwtVerify(token, getSecret());
    return (payload.username as string) ?? null;
  } catch {
    return null;
  }
}

export function validateCredentials(
  username: string,
  password: string
): boolean {
  const expectedUsername = process.env.ADMIN_USERNAME;
  const expectedPassword = process.env.ADMIN_PASSWORD;
  if (!expectedUsername || !expectedPassword) return false;
  return username === expectedUsername && password === expectedPassword;
}
