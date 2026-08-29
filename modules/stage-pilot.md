## Stage: PILOT

Real users are testing it and expect functionality to change without notice, so sweeping changes are still fair game — but the data is real now, and some of it is business data where an error is a monetary liability. Be careful with what already exists: schema changes and backfills are non-destructive by design; rehearse every migration against a copy of real data before running it for real; nothing deletes or rewrites user data without a stated, reviewed plan; flag anything that could interrupt users mid-session.
