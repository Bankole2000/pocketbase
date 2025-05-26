package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bankole2000/pocketbase"
	"github.com/bankole2000/pocketbase/apis"
	"github.com/bankole2000/pocketbase/core"
	"github.com/bankole2000/pocketbase/plugins/ghupdate"
	"github.com/bankole2000/pocketbase/plugins/jsvm"
	"github.com/bankole2000/pocketbase/plugins/migratecmd"
	"github.com/bankole2000/pocketbase/tools/hook"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
)

type PBRecordEvent struct {
	Action     string       `json:"action"`
	Collection string       `json:"collection"`
	Record     *core.Record `json:"record"`
}

type PBCollectionEvent struct {
	Action     string           `json:"action"`
	Collection *core.Collection `json:"collection"`
}
type PBModelEvent struct {
	Action     string     `json:"action"`
	Collection string     `json:"collection"`
	Model      core.Model `json:"model"`
}

type RecordServiceEvent struct {
	EventType string        `json:"type"`
	Data      PBRecordEvent `json:"data"`
	Origin    string        `json:"origin"`
}

type CollectionServiceEvent struct {
	EventType string            `json:"type"`
	Data      PBCollectionEvent `json:"data"`
	Origin    string            `json:"origin"`
}

type ModelServiceEvent struct {
	EventType string       `json:"type"`
	Data      PBModelEvent `json:"data"`
	Origin    string       `json:"origin"`
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

// use godot package to load/read the .env file and
// return the value of the key
func goDotEnvVariable(key string) string {

	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	return os.Getenv(key)
}

func sendRecordEventMessage(eventType string, action string, record *core.Record) {
	message := PBRecordEvent{Action: strings.ToUpper(action), Collection: strings.ToUpper(record.Collection().Name), Record: record}
	eventMessage := RecordServiceEvent{EventType: strings.ToUpper(eventType), Data: message, Origin: "POCKETBASE"}
	jsonBytes, error := json.Marshal(eventMessage)
	failOnError(error, "Failed to convert message to JsonBytes")
	rmqUrl := goDotEnvVariable("RABBITMQ_URL")
	exchange := goDotEnvVariable("RABBITMQ_EXCHANGE")
	queue := goDotEnvVariable("RABBITMQ_QUEUE")
	// rmqUrl := os.Getenv("RABBITMQ_URL")
	// exchange := os.Getenv("RABBITMQ_EXCHANGE")
	// queue := os.Getenv("RABBITMQ_QUEUE")
	conn, err := amqp.Dial(rmqUrl)
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		exchange, // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	q, err := ch.QueueDeclare(
		queue, // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// body := bodyFrom(os.Args)
	err = ch.PublishWithContext(ctx,
		exchange, // exchange
		"",       // routing key
		false,    // mandatory
		false,    // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        jsonBytes,
			// Body:        []byte(body),
		})
	failOnError(err, "Failed to publish to exchange")

	err = ch.PublishWithContext(ctx,
		"", // exchange
		q.Name,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        jsonBytes,
			// Body:        []byte(body),
		})
	failOnError(err, "Failed to publish to queue")

	log.Printf("Record [x] Sent %v => %v", message.Collection, message.Action)
}

// func sendCollectionEventMessage(eventType string, action string, collection *core.Collection) {
// 	message := PBCollectionEvent{Action: strings.ToUpper(action), Collection: collection}
// 	eventMessage := CollectionServiceEvent{EventType: strings.ToUpper(eventType), Data: message, Origin: "POCKETBASE"}
// 	jsonBytes, error := json.Marshal(eventMessage)
// 	failOnError(error, "Failed to convert message to JsonBytes")
// 	rmqUrl := goDotEnvVariable("RABBITMQ_URL")
// 	exchange := goDotEnvVariable("RABBITMQ_EXCHANGE")
// 	// rmqUrl := os.Getenv("RABBITMQ_URL")
// 	// exchange := os.Getenv("RABBITMQ_EXCHANGE")
// 	conn, err := amqp.Dial(rmqUrl)
// 	failOnError(err, "Failed to connect to RabbitMQ")
// 	defer conn.Close()

// 	ch, err := conn.Channel()
// 	failOnError(err, "Failed to open a channel")
// 	defer ch.Close()

// 	err = ch.ExchangeDeclare(
// 		exchange, // name
// 		"fanout", // type
// 		true,     // durable
// 		false,    // auto-deleted
// 		false,    // internal
// 		false,    // no-wait
// 		nil,      // arguments
// 	)
// 	failOnError(err, "Failed to declare an exchange")

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	err = ch.PublishWithContext(ctx,
// 		exchange, // exchange
// 		"",       // routing key
// 		false,    // mandatory
// 		false,    // immediate
// 		amqp.Publishing{
// 			ContentType: "text/plain",
// 			Body:        jsonBytes,
// 			// Body:        []byte(body),
// 		})
// 	failOnError(err, "Failed to publish a message")

// 	log.Printf("Collection [x] Sent %v => %v", message.Collection.Name, message.Action)
// }

// func sendModelEventMessage(eventType string, action string, collection string, model core.Model) {
// 	message := PBModelEvent{Action: strings.ToUpper(action), Collection: collection, Model: model}
// 	eventMessage := ModelServiceEvent{EventType: strings.ToUpper(eventType), Data: message, Origin: "POCKETBASE"}
// 	jsonBytes, error := json.Marshal(eventMessage)
// 	failOnError(error, "Failed to convert message to JsonBytes")
// 	rmqUrl := goDotEnvVariable("RABBITMQ_URL")
// 	exchange := goDotEnvVariable("RABBITMQ_EXCHANGE")
// 	// rmqUrl := os.Getenv("RABBITMQ_URL")
// 	// exchange := os.Getenv("RABBITMQ_EXCHANGE")
// 	conn, err := amqp.Dial(rmqUrl)
// 	failOnError(err, "Failed to connect to RabbitMQ")
// 	defer conn.Close()

// 	ch, err := conn.Channel()
// 	failOnError(err, "Failed to open a channel")
// 	defer ch.Close()

// 	err = ch.ExchangeDeclare(
// 		exchange, // name
// 		"fanout", // type
// 		true,     // durable
// 		false,    // auto-deleted
// 		false,    // internal
// 		false,    // no-wait
// 		nil,      // arguments
// 	)
// 	failOnError(err, "Failed to declare an exchange")

// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	err = ch.PublishWithContext(ctx,
// 		exchange, // exchange
// 		"",       // routing key
// 		false,    // mandatory
// 		false,    // immediate
// 		amqp.Publishing{
// 			ContentType: "text/plain",
// 			Body:        jsonBytes,
// 			// Body:        []byte(body),
// 		})
// 	failOnError(err, "Failed to publish a message")

// 	log.Printf("Model [x] Sent %v => %v", message.Collection, message.Action)
// }

func main() {
	app := pocketbase.New()

	// ---------------------------------------------------------------
	// Optional plugin flags:
	// ---------------------------------------------------------------

	// // fires for every collection
	// app.OnCollectionAfterCreateSuccess().BindFunc(func(e *core.CollectionEvent) error {
	// 	// e.App
	// 	// e.Collection
	// 	sendCollectionEventMessage("COLLECTION", "CREATED", e.Collection)
	// 	return e.Next()
	// })

	// // fires for every collection
	// app.OnCollectionAfterUpdateSuccess().BindFunc(func(e *core.CollectionEvent) error {
	// 	// e.App
	// 	// e.Collection
	// 	sendCollectionEventMessage("COLLECTION", "UPDATED", e.Collection)
	// 	return e.Next()
	// })

	// // fires for every collection
	// app.OnCollectionAfterDeleteSuccess().BindFunc(func(e *core.CollectionEvent) error {
	// 	// e.App
	// 	// e.Collection
	// 	sendCollectionEventMessage("COLLECTION", "DELETED", e.Collection)
	// 	return e.Next()
	// })

	// // fires for every model
	// app.OnModelAfterCreateSuccess().BindFunc(func(e *core.ModelEvent) error {
	// 	// e.App
	// 	// e.Model
	// 	if e.Model.TableName() != "_logs" {
	// 		sendModelEventMessage("MODEL", "CREATED", e.Model.TableName(), e.Model)
	// 	}
	// 	return e.Next()
	// })

	// // fires for every model
	// app.OnModelAfterUpdateSuccess().BindFunc(func(e *core.ModelEvent) error {
	// 	// e.App
	// 	// e.Model
	// 	if e.Model.TableName() != "_logs" {
	// 		sendModelEventMessage("MODEL", "UPDATED", e.Model.TableName(), e.Model)
	// 	}
	// 	return e.Next()
	// })

	// // fires for every model
	// app.OnModelAfterDeleteSuccess().BindFunc(func(e *core.ModelEvent) error {
	// 	// e.App
	// 	// e.Model
	// 	if e.Model.TableName() != "_logs" {
	// 		sendModelEventMessage("MODEL", "DELETED", e.Model.TableName(), e.Model)
	// 	}
	// 	return e.Next()
	// })

	// fires for every record
	app.OnRecordAfterCreateSuccess().BindFunc(func(e *core.RecordEvent) error {
		// e.App
		// e.Record
		sendRecordEventMessage("RECORD", "CREATED", e.Record)
		return e.Next()
	})

	// fires for every record
	app.OnRecordAfterUpdateSuccess().BindFunc(func(e *core.RecordEvent) error {
		// e.App
		// e.Record
		sendRecordEventMessage("RECORD", "UPDATED", e.Record)
		return e.Next()
	})

	// fires for every record
	app.OnRecordAfterDeleteSuccess().BindFunc(func(e *core.RecordEvent) error {
		// e.App
		// e.Record
		sendRecordEventMessage("RECORD", "DELETED", e.Record)
		return e.Next()
	})

	var hooksDir string
	app.RootCmd.PersistentFlags().StringVar(
		&hooksDir,
		"hooksDir",
		"",
		"the directory with the JS app hooks",
	)

	var hooksWatch bool
	app.RootCmd.PersistentFlags().BoolVar(
		&hooksWatch,
		"hooksWatch",
		true,
		"auto restart the app on pb_hooks file change; it has no effect on Windows",
	)

	var hooksPool int
	app.RootCmd.PersistentFlags().IntVar(
		&hooksPool,
		"hooksPool",
		15,
		"the total prewarm goja.Runtime instances for the JS app hooks execution",
	)

	var migrationsDir string
	app.RootCmd.PersistentFlags().StringVar(
		&migrationsDir,
		"migrationsDir",
		"",
		"the directory with the user defined migrations",
	)

	var automigrate bool
	app.RootCmd.PersistentFlags().BoolVar(
		&automigrate,
		"automigrate",
		true,
		"enable/disable auto migrations",
	)

	var publicDir string
	app.RootCmd.PersistentFlags().StringVar(
		&publicDir,
		"publicDir",
		defaultPublicDir(),
		"the directory to serve static files",
	)

	var indexFallback bool
	app.RootCmd.PersistentFlags().BoolVar(
		&indexFallback,
		"indexFallback",
		true,
		"fallback the request to index.html on missing static path, e.g. when pretty urls are used with SPA",
	)

	app.RootCmd.ParseFlags(os.Args[1:])

	// ---------------------------------------------------------------
	// Plugins and hooks:
	// ---------------------------------------------------------------

	// load jsvm (pb_hooks and pb_migrations)
	jsvm.MustRegister(app, jsvm.Config{
		MigrationsDir: migrationsDir,
		HooksDir:      hooksDir,
		HooksWatch:    hooksWatch,
		HooksPoolSize: hooksPool,
	})

	// migrate command (with js templates)
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		TemplateLang: migratecmd.TemplateLangJS,
		Automigrate:  automigrate,
		Dir:          migrationsDir,
	})

	// GitHub selfupdate
	ghupdate.MustRegister(app, app.RootCmd, ghupdate.Config{})

	// static route to serves files from the provided public dir
	// (if publicDir exists and the route path is not already defined)
	app.OnServe().Bind(&hook.Handler[*core.ServeEvent]{
		Func: func(e *core.ServeEvent) error {
			if !e.Router.HasRoute(http.MethodGet, "/{path...}") {
				e.Router.GET("/{path...}", apis.Static(os.DirFS(publicDir), indexFallback))
			}

			return e.Next()
		},
		Priority: 999, // execute as latest as possible to allow users to provide their own route
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// the default pb_public dir location is relative to the executable
func defaultPublicDir() string {
	if strings.HasPrefix(os.Args[0], os.TempDir()) {
		// most likely ran with go run
		return "./pb_public"
	}

	return filepath.Join(os.Args[0], "../pb_public")
}
