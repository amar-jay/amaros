package main

import (
	"encoding/json"
	"log"
	"os"
	"reflect"
	"time"

	"github.com/amar-jay/amaros/core"
	"github.com/amar-jay/amaros/msgs"
	"github.com/amar-jay/amaros/topic"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "amaros",
		EnableBashCompletion: true,
		Usage:                "a simple agentic orchestrator in Go",
		Commands: []*cli.Command{
			{
				Name:        "core",
				Usage:       "start a ROS core server",
				Subcommands: []*cli.Command{},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "tx_port",
						Value: "11311",
						Usage: "amaros TX port",
					},
					&cli.StringFlag{
						Name:  "rx_port",
						Value: "11312",
						Usage: "amaros RX port",
					},

					&cli.StringFlag{
						Name:  "host",
						Value: "0.0.0.0",
						Usage: "ROS master host",
					},
					&cli.BoolFlag{
						Name:    "debug",
						Value:   false,
						Aliases: []string{"verbose"},
						Usage:   "Enable debug logging",
					},
				},
				Action: func(cCtx *cli.Context) error {
					host := cCtx.String("host")
					tx_port := cCtx.Int("tx_port")
					rx_port := cCtx.Int("rx_port")

					r := core.NewCore()
					if cCtx.Bool("debug") {
						r.LogLevel("debug")
					}
					r.Listen(host, port)
					return nil
				},
			},
			{
				Name:        "node",
				Usage:       "run node methods",
				Subcommands: []*cli.Command{},
			},
			{
				Name:  "topic",
				Usage: "run topic methods",

				Flags: []cli.Flag{

					&cli.StringFlag{
						Name:    "address",
						Aliases: []string{"add", "a"},
						Value:   "localhost:11311",
						Usage:   "ROS master host",
					},
				},
				Subcommands: []*cli.Command{
					{
						Name:     "publish",
						Category: "topic",
						Aliases:  []string{"pub"},
						Usage:    "publish a ROS topic",

						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "message",
								Aliases: []string{"msg"},
								// Value:   string(demoMsgBytes),
								Usage: "Message to send",
							},
							&cli.BoolFlag{
								Name:    "once",
								Aliases: []string{"o"},
								Value:   false,
								Usage:   "Publish message once",
							},
						},

						Action: func(cCtx *cli.Context) error {

							if cCtx.NArg() == 0 {
								log.Fatal("Topic name is required")
							}

							message := cCtx.String("message")
							conn := topic.DialServer(cCtx.String("address"))

							// a simple publisher that publishes the message every 5 seconds, or just once if --once flag is set
							demoMsg := new(msgs.DemoMsg)
							demoMsg.Message = message
							if message == "" {
								demoMsg.Message = "Hello Mini ROS!"
							}
							demoMsgBytes, _ := json.Marshal(demoMsg)
							message = string(demoMsgBytes)
							println("MESSAGE IS :" + message)

							var msg interface{}
							err := json.Unmarshal([]byte(message), &msg)
							if err != nil {
								log.Fatal("Unable to unmarshal message")
							}

							if cCtx.Bool("once") {
								topic.Publish(conn, cCtx.Args().Get(0), msg)
							} else {
								for {
									topic.Publish(conn, cCtx.Args().Get(0), msg)
									time.Sleep(5 * time.Second)
								}
							}

							return nil
						},
					},
					{
						Name:     "subscribe",
						Category: "topic",
						Aliases:  []string{"sub"},
						Usage:    "subscribe to a ROS topic",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								log.Fatal("Topic name is required")
							}
							conn := topic.DialServer(cCtx.String("address"))
							msg := msgs.DemoMsg{}
							_topic := cCtx.Args().Get(0)
							callback := func(ctx topic.CallbackContext) {
								// a simple callback that just prints the message content and type
								ctx.Logger.WithFields(map[string]interface{}{
									"topic": _topic,
									"type":  reflect.TypeOf(msg),
								}).Debug(msg.Message)
							}

							topic.Subscribe(conn, _topic, &msg, callback)
							return nil
						},
					},
					{
						Name:     "status",
						Aliases:  []string{"stats", "stat"},
						Category: "topic",
						Usage:    "get stats of a ROS topic",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								log.Fatal("Topic name is required")
							}
							conn := topic.DialServer(cCtx.String("address"))
							topic.SubscribeStatus(conn, cCtx.Args().Get(0))
							return nil
						},
					},
					{
						Name:     "list",
						Category: "topic",
						Usage:    "get list of all topics",
						Action: func(cCtx *cli.Context) error {
							conn := topic.DialServer(cCtx.String("address"))
							topic.List(conn)
							return nil
						},
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
