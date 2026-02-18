import { fetchApiCallLogs } from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Navbar } from "@/components/navbar";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export default async function ApiLogsPage() {
  const logs = await fetchApiCallLogs(100);

  return (
    <div className="min-h-screen">
      <header className="border-b-2 border-border bg-background px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Link
              href="/"
              className="text-sm text-muted-foreground hover:text-foreground"
            >
              &larr; Back
            </Link>
            <h1 className="text-2xl font-bold">API Call Log</h1>
          </div>
          <LogoutButton />
        </div>
        <p className="text-sm text-muted-foreground">
          Recent SAM.gov API calls from sync operations
        </p>
      </header>

      <main className="p-6">
        {logs.length === 0 ? (
          <p className="text-center text-muted-foreground py-12">
            No API calls logged yet. Run a sync to see activity here.
          </p>
        ) : (
          <div className="border-2 border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Timestamp</TableHead>
                  <TableHead>Context</TableHead>
                  <TableHead>Date Window</TableHead>
                  <TableHead className="text-right">API Calls</TableHead>
                  <TableHead className="text-right">Records</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((log) => (
                  <TableRow key={log.id}>
                    <TableCell className="font-mono text-xs">
                      {log.timestamp}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{log.context}</Badge>
                    </TableCell>
                    <TableCell className="text-xs">
                      {log.posted_from && log.posted_to
                        ? `${log.posted_from} - ${log.posted_to}`
                        : "-"}
                    </TableCell>
                    <TableCell className="text-right">{log.api_calls}</TableCell>
                    <TableCell className="text-right">
                      {log.records_fetched}
                    </TableCell>
                    <TableCell>
                      {log.rate_limited ? (
                        <Badge variant="destructive">Rate Limited</Badge>
                      ) : log.error_message ? (
                        <Badge variant="destructive">{log.error_message}</Badge>
                      ) : (
                        <Badge className="bg-green-600 text-white">OK</Badge>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </main>
    </div>
  );
}
