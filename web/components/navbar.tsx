import Link from "next/link";
import { LogoutButton } from "@/components/logout-button";

interface NavbarProps {
  title: string;
  subtitle?: string;
  children?: React.ReactNode;
}

export function Navbar({ title, subtitle, children }: NavbarProps) {
  return (
    <header className="border-b-2 border-border bg-background px-6 py-4">
      <div className="flex items-center justify-between">
        <Link href="/" className="text-2xl font-bold hover:opacity-80">
          GovScout
        </Link>
        <div className="flex items-center gap-3">
          <Link
            href="/"
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            Opportunities
          </Link>
          <Link
            href="/api-logs"
            className="text-sm text-muted-foreground hover:text-foreground"
          >
            API Logs
          </Link>
          <LogoutButton />
        </div>
      </div>
      <div className="mt-1 flex items-center gap-3">
        {children}
        <div>
          <h1 className="text-lg font-semibold">{title}</h1>
          {subtitle && (
            <p className="text-sm text-muted-foreground">{subtitle}</p>
          )}
        </div>
      </div>
    </header>
  );
}
