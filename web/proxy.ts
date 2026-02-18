import { NextRequest, NextResponse } from "next/server";
import { verifySessionToken, getSessionCookieName } from "@/lib/auth";

export async function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // Skip auth for public routes
  if (
    pathname.startsWith("/_next") ||
    pathname.startsWith("/favicon") ||
    pathname === "/login" ||
    pathname.startsWith("/api/auth/")
  ) {
    return NextResponse.next();
  }

  const token = request.cookies.get(getSessionCookieName())?.value;

  if (!token) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  const username = await verifySessionToken(token);

  if (!username) {
    const response = NextResponse.redirect(new URL("/login", request.url));
    response.cookies.set(getSessionCookieName(), "", {
      maxAge: 0,
      path: "/",
    });
    return response;
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/((?!_next/static|_next/image|favicon.ico).*)"],
};
