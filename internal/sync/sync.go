package sync

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/theognis1002/govscout/internal/db"
	"github.com/theognis1002/govscout/internal/samgov"
)

const (
	backfillWindowDays = 90
	incrementalDays    = 3
	dateFmt            = "01/02/2006"
)

type Options struct {
	MaxCalls int
	DryRun   bool
	From     string
}

func Run(database *sql.DB, client *samgov.Client, opts Options) error {
	if opts.MaxCalls <= 0 {
		opts.MaxCalls = 18
	}
	apiCallsUsed := 0
	today := time.Now()

	// Phase 1: Incremental (last 3 days)
	incrFrom := today.AddDate(0, 0, -incrementalDays).Format(dateFmt)
	incrTo := today.Format(dateFmt)

	log.Printf("incremental sync: %s to %s", incrFrom, incrTo)
	if opts.DryRun {
		log.Printf("[dry-run] would fetch %s to %s", incrFrom, incrTo)
	} else {
		result, err := client.SearchWindow(incrFrom, incrTo, func(opps []map[string]any) error {
			for _, opp := range opps {
				if err := db.UpsertOpportunityFromAPI(database, opp); err != nil {
					log.Printf("upsert error: %v", err)
				}
			}
			return nil
		})
		if err != nil {
			errMsg := err.Error()
			db.InsertSyncRun(database, "incremental", incrFrom, incrTo, 0, 0, false, &errMsg)
			return fmt.Errorf("incremental sync: %w", err)
		}
		apiCallsUsed += result.APICalls
		db.InsertSyncRun(database, "incremental", incrFrom, incrTo, result.APICalls, result.TotalFetched, result.RateLimited, nil)
		log.Printf("incremental: %d records, %d api calls, rate_limited=%v", result.TotalFetched, result.APICalls, result.RateLimited)

		if result.RateLimited {
			log.Println("rate limited during incremental, stopping")
			return nil
		}
	}

	// Phase 2: Backfill
	remaining := opts.MaxCalls - apiCallsUsed
	if remaining < 2 {
		log.Println("no budget remaining for backfill")
		return nil
	}

	cursor, err := resolveBackfillCursor(database, today)
	if err != nil {
		return fmt.Errorf("resolve cursor: %w", err)
	}

	var backfillFloor *time.Time
	if opts.From != "" {
		t, err := time.Parse(dateFmt, opts.From)
		if err != nil {
			return fmt.Errorf("parse --from: %w", err)
		}
		backfillFloor = &t
	}

	for apiCallsUsed+2 <= opts.MaxCalls {
		if backfillFloor != nil && !cursor.After(*backfillFloor) {
			log.Printf("reached backfill floor %s", backfillFloor.Format(dateFmt))
			break
		}

		windowTo := cursor
		windowFrom := cursor.AddDate(0, 0, -backfillWindowDays)

		fromStr := windowFrom.Format(dateFmt)
		toStr := windowTo.Format(dateFmt)
		log.Printf("backfill window: %s to %s", fromStr, toStr)

		if opts.DryRun {
			log.Printf("[dry-run] would fetch %s to %s", fromStr, toStr)
			cursor = windowFrom
			apiCallsUsed += 2
			continue
		}

		result, err := client.SearchWindow(fromStr, toStr, func(opps []map[string]any) error {
			for _, opp := range opps {
				if err := db.UpsertOpportunityFromAPI(database, opp); err != nil {
					log.Printf("upsert error: %v", err)
				}
			}
			return nil
		})
		if err != nil {
			errMsg := err.Error()
			db.InsertSyncRun(database, "backfill", fromStr, toStr, 0, 0, false, &errMsg)
			return fmt.Errorf("backfill: %w", err)
		}

		apiCallsUsed += result.APICalls
		db.InsertSyncRun(database, "backfill", fromStr, toStr, result.APICalls, result.TotalFetched, result.RateLimited, nil)
		log.Printf("backfill: %d records, %d api calls, rate_limited=%v", result.TotalFetched, result.APICalls, result.RateLimited)

		cursor = windowFrom
		db.SetSyncState(database, "backfill_cursor", cursor.Format(dateFmt))

		if result.RateLimited {
			log.Println("rate limited during backfill, stopping")
			break
		}
	}

	db.SetSyncState(database, "last_sync", today.Format(dateFmt))
	return nil
}

func resolveBackfillCursor(database *sql.DB, today time.Time) (time.Time, error) {
	cursorStr, err := db.GetSyncState(database, "backfill_cursor")
	if err != nil {
		return time.Time{}, err
	}
	if cursorStr != "" {
		return time.Parse(dateFmt, cursorStr)
	}

	earliest, err := db.GetEarliestPostedDate(database)
	if err != nil {
		return time.Time{}, err
	}
	if earliest != "" {
		return time.Parse(dateFmt, earliest)
	}

	return today.AddDate(0, 0, -incrementalDays), nil
}
