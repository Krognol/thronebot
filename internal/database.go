package internal

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/Krognol/thronebot/internal/router"
)

// GetBannedHandler returns a router handler which checks for banned weekly items
func GetBannedHandler(db *sql.DB) router.HandlerFunc {
	return func(ctx *router.Context) {
		rows, err := db.Query("SELECT * FROM weekly_banned;")
		if err != nil {
			log.Println("getBanned: failed to query db:", err)
			ctx.Reply("Failed to retrieve banned items.")
			return
		}

		defer rows.Close()

		var (
			char, crown, wep    string
			chars, crowns, weps []string
		)

		// TODO find more efficient solution
		for rows.Next() {
			err = rows.Scan(&char, &crown, &wep)
			if err != nil {
				log.Fatal("getBanned: failed to scan row:", err)
				ctx.Reply("Failed to retrieve banned items.")
				return
			}

			if char != "" {
				chars = append(chars, char)
			}

			if crown != "" {
				crowns = append(crowns, crown)
			}

			if wep != "" {
				weps = append(weps, wep)
			}
		}

		var buf strings.Builder

		buf.WriteString("Currently banned items:\n\n")

		if len(chars) > 0 {
			buf.WriteString("**Characters:**\n  ")
			buf.WriteString(strings.Join(chars, "\n  "))
		}

		if len(crowns) > 0 {
			buf.WriteString("**Crowns:**\n  ")
			buf.WriteString(strings.Join(crowns, "\n  "))
		}

		if len(weps) > 0 {
			buf.WriteString("**Weapons:**\n  ")
			buf.WriteString(strings.Join(weps, "\n  "))
		}

		ctx.Reply(buf.String())
	}
}

// IsBanned checks if one or more items are currently banned from the weekly
func IsBanned(db *sql.DB, char, weap, crown string) (bool, error) {
	rows, err := db.Query(
		"SELECT * FROM weekly_banned WHERE chars = ? OR weaps = ? OR crowns = ?;",
		char,
		weap,
		crown,
	)

	if err != nil {
		log.Println("checkBanned: failed to retrieve rows:", err)
		return false, err
	}

	defer rows.Close()

	// Should maybe cache result
	var bannedChar, bannedCrown, bannedWeap string
	for rows.Next() {
		rows.Scan(&bannedChar, &bannedCrown, &bannedWeap)
		if bannedChar != "" || bannedCrown != "" || bannedWeap != "" {
			return true, nil
		}
	}

	return false, nil
}

// GetUserSuggestionCount returns the requesting users suggestion count
func GetUserSuggestionCount(db *sql.DB, id string) int {
	rows, err := db.Query("SELECT count FROM user_suggestions WHERE id = ?;", id)
	if err != nil {
		log.Println("getUserSuggestionCount: failed to get rows:", err)
		return -1
	}

	defer rows.Close()

	var count int
	for rows.Next() {
		rows.Scan(&count)
	}

	return count
}

// InsertSuggestion inserts a weekly suggestion into the database
func InsertSuggestion(db *sql.DB, uid, char, weap, crown string, skin bool) error {
	// order is uid, char, skin, weap, crown
	stmt, err := db.Prepare("INSERT INTO weekly_suggestions VALUES(?, ?, ?, ?);")
	if err != nil {
		log.Println("inserSuggestion: failed to prepare stmt:", err)
		return err
	}

	defer stmt.Close()

	useSkin := 0
	if skin {
		useSkin = 1
	}

	_, err = stmt.Exec(uid, char, useSkin, weap, crown)
	if err != nil {
		log.Println("inserSuggestion: failed to insert into table stmt:", err)
		return err
	}

	return updateSuggestionCount(db, uid)
}

func updateSuggestionCount(db *sql.DB, uid string) (err error) {
	var (
		tx   *sql.Tx
		stmt *sql.Stmt
	)

	tx, err = db.Begin()
	if err != nil {
		log.Println("updateSuggestionCount: failed to begin Tx:", err)
		return
	}

	stmt, err = tx.Prepare("INSERT INTO user_suggestions(id, count) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET count = count + 1;")
	if err != nil {
		log.Println("updateSuggestionCount: failed to prepare stmt:", err)
		return tx.Rollback()
	}

	defer stmt.Close()

	_, err = stmt.Exec(uid, 1)
	if err != nil {
		log.Println("updateSuggestionCount: failed to exec stmt:", err)
		return tx.Rollback()
	}
	return tx.Commit()
}

// WeeklyBanAdd adds an item as banned for the weekly unless it already exists in the column
func WeeklyBanAdd(db *sql.DB, kind string, val int) error {
	stmt, err := db.Prepare(fmt.Sprintf("INSERT OR FAIL INTO weekly_banned(%s) VALUES(?);", kind))
	if err != nil {
		log.Println("weeklyBanAdd: failed to prepare stmt: ", err)
		return err
	}

	_, err = stmt.Exec(val)
	if err != nil {
		log.Println("weeklyBanAdd: failed to insert item into db: ", err)
	}
	return err
}

// WeeklyBanDel removes an item from the banned list
func WeeklyBanDel(db *sql.DB, kind string, val int) error {
	stmt, err := db.Prepare(fmt.Sprintf("DELETE FROM weekly_banned WHERE %s = ?;", kind))
	if err != nil {
		log.Println("weeklyBanDel: failed to prepare stmt: ", err)
		return err
	}

	_, err = stmt.Exec(val)
	if err != nil {
		log.Println("weeklyBanDel: failed to remove item: ", err)
	}
	return err
}
