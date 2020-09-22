package postgresql

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/cutil"
	"strconv"
	"time"
)

// Constants for the SQL statements that are used to manage search sessions. A search session is just a number. The Exchange
// uses it like a key to indicate that a given policy search should return a single page of results. The Exchange keeps track of
// the timestamp of the node that was most recently returned on a given search (keyed by the session number) so that future searches
// with the same policy and session key will return nodes that have changed since the last one that was returned.
//
// schema:
// policyName:          The fully qualified (org/policy-name) policy being searched
// changedSince:        This is a linux epoch time stamp indicating that the exchange should return nodes that have changed since this time.
// sessionToken:        This is a search session token, used to ensure that all agbots use the same session to search for nodes,
//                      allowing the exchange to return a different page of results to each agbot. It is a number converted to a string.
// sessionEnded:        Indicates that the current session is ended, so a new session can be allocated.
// restartChangedSince: Indicates that an agbot was restarted, so this changedSince should be used when the next session is created.
// updatingAgbot:       The UUID of the agbot that last updated this table/row.
// updated:             The time when the agbot updated this table/row/
//

const SEARCH_SESSIONS_CREATE_MAIN_TABLE = `CREATE TABLE IF NOT EXISTS search_sessions (
	policyName          text    PRIMARY KEY,
	changedSince        int     NOT NULL,
	sessionToken        int     NOT NULL,
	sessionEnded        boolean NOT NULL,
	restartChangedSince int     NOT NULL,
	updatingAgbot       text    NOT NULL,
	updated timestamp with time zone DEFAULT current_timestamp
);`

const SEARCH_SESSIONS_DUMP = `SELECT * FROM search_sessions;`

const SEARCH_SESSIONS_UPDATE_SESSION = `
CREATE OR REPLACE FUNCTION get_session(
	policy CHARACTER VARYING,
	agbot_id CHARACTER VARYING)
	RETURNS TABLE(sess int, cs int) AS $$

BEGIN
LOCK TABLE search_sessions;

IF EXISTS (SELECT sessionToken FROM search_sessions WHERE policyName = policy AND sessionEnded = true) THEN

	/* Update changedSince based on an agbot restart. */
	UPDATE search_sessions
		SET changedSince = (SELECT restartChangedSince from search_sessions WHERE policyName = policy AND sessionEnded = true), restartChangedSince = 0
		WHERE policyName = policy AND sessionEnded = true AND restartChangedSince != 0;

	/* Get a new session token. */
	UPDATE search_sessions
		SET sessionToken = sessionToken + 1, sessionEnded = false, updatingAgbot = agbot_id, updated = current_timestamp
		WHERE policyName = policy AND sessionEnded = true;

	/* Handle session token roll over, note that the session was ended in the previous update. */
	UPDATE search_sessions
		SET sessionToken = 1
		WHERE policyName = policy AND sessionEnded = false AND sessionToken > 2000000000;

	RETURN QUERY SELECT sessionToken, changedSince FROM search_sessions WHERE policyName = policy AND sessionEnded = false;

ELSE
	/* Either the row exists and the session is not ended, or the row doesnt exist at all. */
	IF NOT EXISTS (SELECT sessionToken FROM search_sessions WHERE policyName = policy) THEN
		INSERT INTO search_sessions (policyName, changedSince, sessionToken, sessionEnded, restartChangedSince, updatingAgbot, updated)
			VALUES (policy, 0, 1999999998, false, 0, agbot_id, current_timestamp);
	END IF;

	RETURN QUERY SELECT sessionToken, changedSince FROM search_sessions WHERE policyName = policy;
END IF;

END $$ LANGUAGE plpgsql;
`
const SEARCH_SESSIONS_UPDATE_SESSION_BY_FUNCTION = `SELECT * FROM get_session($1,$2);`

const SEARCH_SESSIONS_UPDATE_CHANGED_SINCE = `UPDATE search_sessions x
	SET changedSince = $2, sessionEnded = true, updatingAgbot = $3, updated = current_timestamp
		FROM (SELECT sessionEnded FROM search_sessions WHERE policyName = $4 FOR UPDATE) old_table
	WHERE x.changedSince = $1 AND x.sessionEnded = false AND policyName = $4
	RETURNING old_table.sessionEnded
;`

const SEARCH_SESSIONS_RESET_CHANGED_SINCE = `
CREATE OR REPLACE FUNCTION reset_changedSince(
	newChangedSince INTEGER,
	agbot_id CHARACTER VARYING)
	RETURNS void AS $$

BEGIN
LOCK TABLE search_sessions;

UPDATE search_sessions
	SET restartChangedSince = newChangedSince, updatingAgbot = agbot_id, updated = current_timestamp
	WHERE sessionEnded = false;

UPDATE search_sessions
	SET changedSince = newChangedSince, updatingAgbot = agbot_id, updated = current_timestamp
	WHERE sessionEnded = true;

END $$ LANGUAGE plpgsql;
`
const SEARCH_SESSIONS_RESET_CHANGEDSINCE_BY_FUNCTION = `SELECT * FROM reset_changedSince($1,$2);`

const SEARCH_SESSIONS_RESET_CHANGED_SINCE_FOR_POLICY = `UPDATE search_sessions
	SET restartChangedSince = $1, updatingAgbot = $3, updated = current_timestamp
	WHERE policyName = $2 AND (restartChangedSince = 0 OR restartChangedSince > $1);
`

// Functions related to the search session table.

// Get the current search session from the DB. If the current session is ended, then a new session token will
// be allocated and stored in the DB.
func (db *AgbotPostgresqlDB) ObtainSearchSession(policyName string) (string, uint64, error) {
	var ss sql.NullInt64
	var cs sql.NullInt64
	if err := db.db.QueryRow(SEARCH_SESSIONS_UPDATE_SESSION_BY_FUNCTION, policyName, db.identity).Scan(&ss, &cs); err != nil {
		return "", 0, errors.New(fmt.Sprintf("error obtaining %v search session, error: %v", policyName, err))
	} else if !ss.Valid {
		return "", 0, errors.New(fmt.Sprintf("returned search session for %v is not a valid integer, error: %v", policyName, err))
	} else if !cs.Valid {
		return "", 0, errors.New(fmt.Sprintf("returned changedSince for %v is not a valid integer, error: %v", policyName, err))
	} else {
		return strconv.FormatInt(ss.Int64, 10), uint64(cs.Int64), nil
	}
}

// Update the changed since time in the DB and mark the current session as ended. This is done when a node scan has completed
// successfully and all pages of nodes have been processed. The returned boolean indicates whether or not the session was
// already ended. If true, it means that another agbot ended the session before the caller did, which can happen normally.
// However, it is an indication to the calling agbot that it processing the current search session overlapping the other agbot.
// This usually means the agbot should do one more node search, just to be sure nothing was missed.
func (db *AgbotPostgresqlDB) UpdateSearchSessionChangedSince(currentChangedSince uint64, newChangedSince uint64, policyName string) (bool, error) {
	var se sql.NullBool
	glog.V(3).Infof("AgreementBot updating changedSince from %v to %v for %v search session", time.Unix(int64(currentChangedSince), 0).Format(cutil.ExchangeTimeFormat), time.Unix(int64(newChangedSince), 0).Format(cutil.ExchangeTimeFormat), policyName)
	if err := db.db.QueryRow(SEARCH_SESSIONS_UPDATE_CHANGED_SINCE, currentChangedSince, newChangedSince, db.identity, policyName).Scan(&se); err != nil {
		return false, errors.New(fmt.Sprintf("error updating %v search session changedSince, error: %v", policyName, err))
	} else if !se.Valid {
		return false, errors.New(fmt.Sprintf("returned search session state for %v is not a valid boolean, error: %v", policyName, err))
	} else {
		return se.Bool, nil
	}
}

// Update all search session with a new changed Since to account for possible lost search results when an agbot restarts.
func (db *AgbotPostgresqlDB) ResetAllChangedSince(newChangedSince uint64) error {
	if _, err := db.db.Exec(SEARCH_SESSIONS_RESET_CHANGEDSINCE_BY_FUNCTION, newChangedSince, db.identity); err != nil {
		return errors.New(fmt.Sprintf("error resetting changed since in all search sessions, error: %v", err))
	}
	return nil
}

// Update search session for a specific policy with a new changed Since to account for possible lost search results.
func (db *AgbotPostgresqlDB) ResetPolicyChangedSince(policy string, newChangedSince uint64) error {
	if _, err := db.db.Exec(SEARCH_SESSIONS_RESET_CHANGED_SINCE_FOR_POLICY, newChangedSince, policy, db.identity); err != nil {
		return errors.New(fmt.Sprintf("error resetting changed since in %v search sessions, error: %v", policy, err))
	}
	return nil
}

type ssRecord struct {
	pn string
	cs int64
	st int64
	se bool
	r  int64
	ua string
	up string
}

func (r ssRecord) String() string {
	return fmt.Sprintf("Policy: %v, ChangedSince: %v, SessionToken: %v, SessionEnded: %v, RestartCS: %v, Agbot: %v, Updated: %v", r.pn, r.cs, r.st, r.se, r.r, r.ua, r.up)
}

// Update all search session with a new changed Since to account for possible lost search results when an agbot restarts.
func (db *AgbotPostgresqlDB) DumpSearchSessions() error {
	if rows, err := db.db.Query(SEARCH_SESSIONS_DUMP); err != nil {
		return errors.New(fmt.Sprintf("error dumping search sessions, error: %v", err))
	} else {
		defer rows.Close()
		for rows.Next() {
			out := ssRecord{}
			if err := rows.Scan(&out.pn, &out.cs, &out.st, &out.se, &out.r, &out.ua, &out.up); err != nil {
				glog.Errorf("AgbotDB: error dumping search sessions table, error: %v", err)
			} else {
				glog.V(4).Infof("Search Session: %v", out)
			}
		}
	}
	return nil
}
