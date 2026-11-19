package server

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/SabhyaAggarwal/gritloom/internal/tracking"
)

const engagedMaxGap = tracking.EngagedMaxGap

// buildSessions uses the same conservative interval accounting as the local
// agent, then adds the server-side reviewer evidence and hash chain.
func buildSessions(userID string, events []storedEvent) []Session {
	input := make([]tracking.Event, len(events))
	for i, event := range events {
		input[i] = tracking.Event(event.Event)
		input[i].ProjectPath = event.ProjectID
	}
	built := tracking.BuildSessions(input)
	sessions := make([]Session, 0, len(built))
	previousHash := ""
	for _, part := range built {
		if part.Seconds < 1 {
			continue
		}
		session := Session{
			ProjectID: part.ProjectPath,
			StartAt:   part.StartAt.UTC(),
			EndAt:     part.EndAt.UTC(),
			Seconds:   part.Seconds,
			Kind:      part.Kind,
			Source:    strings.Join(part.Sources, "+"),
			Evidence:  make([]EvidenceEvent, 0, len(part.Evidence)),
		}
		if session.Source == "" {
			session.Source = "unknown"
		}
		signals := make(map[string]struct{})
		passedTest := false
		hasher := sha256.New()
		fmt.Fprintf(hasher, "gritloom-evidence-v1\n%s\n", previousHash)
		for _, item := range part.Evidence {
			event := EvidenceEvent{
				ID: item.ID, OccurredAt: item.OccurredAt, Source: item.Source, Kind: item.Kind,
				File: item.File, Additions: item.Additions, Deletions: item.Deletions,
				ExitCode: item.ExitCode, AI: item.AI, Background: item.Background,
				Model: item.Model, CommitHash: item.CommitHash, Branch: item.Branch,
			}
			session.Evidence = append(session.Evidence, event)
			switch event.Kind {
			case "edit", "save":
				session.EditCount++
				signals["human"] = struct{}{}
			case "ai_start", "ai_end", "ai_tool":
				session.AICount++
				signals["ai"] = struct{}{}
			case "build", "test", "debug":
				session.TestCount++
				signals["verification"] = struct{}{}
				if event.ExitCode != nil {
					if *event.ExitCode == 0 {
						passedTest = true
					} else if event.Kind == "test" || event.Kind == "build" {
						session.FailedTestCount++
					}
				}
			case "commit":
				session.CommitCount++
				signals["milestone"] = struct{}{}
			}
			fmt.Fprintf(hasher, "%s|%d|%s|%s|%s|%s|%d|%d|%v|%v|%v|%s|%s|%s\n", event.ID, event.OccurredAt.UnixMilli(), session.ProjectID, event.Source, event.Kind, event.File, event.Additions, event.Deletions, event.ExitCode, event.AI, event.Background, event.Model, event.CommitHash, event.Branch)
		}
		if session.Kind == tracking.SessionEngaged {
			if len(signals) >= 2 && (session.CommitCount > 0 || passedTest) && session.EditCount > 0 {
				session.Verification = "review_ready"
			} else if len(signals) >= 2 {
				session.Verification = "corroborated"
			} else {
				session.Verification = "observed"
			}
		} else {
			session.Verification = "observed"
		}
		session.EvidenceHash = hex.EncodeToString(hasher.Sum(nil))
		previousHash = session.EvidenceHash
		identity := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%d|%s", userID, session.ProjectID, session.StartAt.UnixMilli(), session.EndAt.UnixMilli(), session.Kind)))
		session.ID = hex.EncodeToString(identity[:16])
		sessions = append(sessions, session)
	}
	return sessions
}

func loadEvents(ctx interface {
	Query(string, ...any) (*sql.Rows, error)
}, userID string) ([]storedEvent, error) {
	rows, err := ctx.Query(`SELECT id, project_id, occurred_at, branch, source, kind, file, additions, deletions, exit_code, ai, background, model, commit_hash FROM events WHERE user_id = ? ORDER BY occurred_at, id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]storedEvent, 0)
	for rows.Next() {
		var event storedEvent
		var milliseconds int64
		var ai, background int
		if err := rows.Scan(&event.ID, &event.ProjectID, &milliseconds, &event.Branch, &event.Source, &event.Kind, &event.File, &event.Additions, &event.Deletions, &event.ExitCode, &ai, &background, &event.Model, &event.CommitHash); err != nil {
			return nil, err
		}
		event.OccurredAt = time.UnixMilli(milliseconds).UTC()
		event.AI = ai != 0
		event.Background = background != 0
		events = append(events, event)
	}
	return events, rows.Err()
}
