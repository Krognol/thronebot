package main

import (
	"database/sql"
	"database/sql/driver"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Krognol/thronebot/internal"
	"github.com/Krognol/thronebot/internal/router"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

var (
	tbAPIKey          = flag.String("tb", "", "Thronebot API key")
	discordBotKey     = flag.String("t", "", "Discord bot key")
	githubAPIKey      = flag.String("git", "", "Github API key. For archiving of pins")
	githubArchiveRepo = flag.String("rep", "", "Repo name")
	dbName            = flag.String("db", "db.sqlite", "Path to SQLite DB file")
	logFile           = flag.String("log", "", "Path to file to print logging to")
)

func main() {
	if *logFile == "" {
		log.SetOutput(os.Stdout)
	} else {
		f, err := os.Open(*logFile)
		if err != nil {
			// Can't open/create file, something's wrong
			panic(err)
		}
		log.SetOutput(f)
		defer f.Close()
	}

	db, err := sql.Open("sqlite3", *dbName)
	if err != nil {
		log.Fatal(err)
	}

	if *discordBotKey == "" {
		log.Fatal("Discord bot key can't be nil:", flag.ErrHelp)
	}

	ses, err := discordgo.New(*discordBotKey)
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()
	defer ses.Close()

	bot := internal.NewBot(ses, db, router.NewRoute())

	// Commands
	bot.Route.On("pingdb", pingdbHandler(bot.DB)).Use(internal.ElevatedUser).Desc("Pings the database for a connection.")

	var BotID = ses.State.User.ID

	bot.Ses.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		bot.Route.FindAndExecute(s, "thronebot", BotID, m.Message)
	})

	closer := make(chan os.Signal, 1)
	signal.Notify(closer, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-closer
}

func pingdbHandler(db *sql.DB) router.HandlerFunc {
	return func(ctx *router.Context) {
		res := db.Ping()
		if res == driver.ErrBadConn {
			log.Print("bot: bad DB conn:", res)
		}
		ctx.Reply(res.Error())
	}
}
