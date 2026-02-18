import Link from "next/link";
import { LogoutButton } from "@/components/logout-button";

interface NavbarProps {
  currentPage: "opportunities" | "api-logs" | "detail";
}

const navLinks = [
  { href: "/", label: "Opportunities", key: "opportunities" as const },
  { href: "/api-logs", label: "API Logs", key: "api-logs" as const },
];

export function Navbar({ currentPage }: NavbarProps) {
  return (
    <header className="border-b-2 border-border bg-background px-6 py-4">
      <div className="flex items-center justify-between">
        <Link href="/" className="text-2xl font-bold hover:opacity-80">
          GovScout
        </Link>
        <div className="flex items-center gap-3">
          {navLinks.map((link) => (
            <Link
              key={link.key}
              href={link.href}
              className={
                currentPage === link.key
                  ? "text-sm font-semibold text-foreground"
                  : "text-sm text-muted-foreground hover:text-foreground"
              }
            >
              {link.label}
            </Link>
          ))}
          <LogoutButton />
        </div>
      </div>
    </header>
  );
}
