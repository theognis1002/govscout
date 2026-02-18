import { NextRequest, NextResponse } from "next/server";
import {
  validateCredentials,
  createSessionToken,
  getSessionCookieName,
} from "@/lib/auth";

export async function POST(request: NextRequest) {
  const body = await request.json();
  const { username, password } = body;

  if (!username || !password) {
    return NextResponse.json(
      { error: "Username and password are required" },
      { status: 400 }
    );
  }

  if (!validateCredentials(username, password)) {
    return NextResponse.json(
      { error: "Invalid credentials" },
      { status: 401 }
    );
  }

  const token = await createSessionToken(username);
  const response = NextResponse.json({ ok: true });

  response.cookies.set(getSessionCookieName(), token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    maxAge: 60 * 60 * 24, // 24 hours
    path: "/",
  });

  return response;
}
