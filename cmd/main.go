package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/amar-jay/amaros/pkg/core"
	"github.com/amar-jay/amaros/pkg/msgs"
	"github.com/amar-jay/amaros/pkg/registry"
	"github.com/amar-jay/amaros/pkg/topic"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "amaros",
		EnableBashCompletion: true,
		Usage:                "an agentic orchestrator in Go",
		Commands: []*cli.Command{
			{
				Name:        "core",
				Usage:       "start master server",
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
						Usage: "master host",
					},
					&cli.BoolFlag{
						Name:    "debug",
						Value:   false,
						Aliases: []string{"verbose", "v"},
						Usage:   "Enable debug logging",
					},
				},
				Action: func(cCtx *cli.Context) error {
					host := cCtx.String("host")
					txPort := cCtx.Int("tx_port")
					rxPort := cCtx.Int("rx_port")

					r := core.NewCore()
					if cCtx.Bool("debug") {
						r.LogLevel("debug")
					}
					r.Listen(host, txPort, rxPort)
					return nil
				},
			},
			{
				Name:        "node",
				Usage:       "run node methods",
				Subcommands: []*cli.Command{},
			},
			{
				Name:    "registry",
				Usage:   "manage AMAROS node registry",
				Aliases: []string{"reg"},
				Subcommands: []*cli.Command{
					{
						Name:    "search",
						Aliases: []string{"s"},
						Usage:   "search for nodes in the remote registry",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "tag",
								Aliases: []string{"t"},
								Usage:   "filter by exact tag",
							},
						},
						Action: func(cCtx *cli.Context) error {
							reg, err := registry.New()
							if err != nil {
								return err
							}

							query := cCtx.Args().First()
							tag := cCtx.String("tag")

							var results []registry.SearchResult
							if tag != "" {
								results, err = reg.SearchByTag(tag)
							} else if query != "" {
								results, err = reg.Search(query)
							} else {
								results, err = reg.ListRemote()
							}
							if err != nil {
								return err
							}

							if len(results) == 0 {
								fmt.Println("No nodes found.")
								return nil
							}

							for _, r := range results {
								fmt.Printf("  %-20s %-8s  %s\n", r.Name, r.Latest, r.Description)
							}
							fmt.Printf("\n%d node(s) found.\n", len(results))
							return nil
						},
					},
					{
						Name:    "install",
						Aliases: []string{"add", "i"},
						Usage:   "install a node from the remote registry",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "version",
								Aliases: []string{"v"},
								Usage:   "specific version to install (default: latest)",
							},
						},
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								return fmt.Errorf("node name is required")
							}
							reg, err := registry.New()
							if err != nil {
								return err
							}
							return reg.Install(cCtx.Args().First(), cCtx.String("version"))
						},
					},
					{
						Name:    "uninstall",
						Aliases: []string{"remove", "rm"},
						Usage:   "uninstall a locally installed node",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								return fmt.Errorf("node name is required")
							}
							reg, err := registry.New()
							if err != nil {
								return err
							}
							return reg.Uninstall(cCtx.Args().First())
						},
					},
					{
						Name:    "upgrade",
						Aliases: []string{"up"},
						Usage:   "upgrade a node to its latest version",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								return fmt.Errorf("node name is required")
							}
							reg, err := registry.New()
							if err != nil {
								return err
							}
							return reg.Upgrade(cCtx.Args().First())
						},
					},
					{
						Name:    "list",
						Aliases: []string{"ls"},
						Usage:   "list available nodes",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:  "remote",
								Usage: "list remote nodes",
							},
							&cli.BoolFlag{
								Name:  "all",
								Usage: "list all nodes (remote and local)",
							},
						},
						Action: func(cCtx *cli.Context) error {

							if cCtx.Bool("remote") && cCtx.Bool("all") {
								return fmt.Errorf("--remote and --all cannot be used together")
							}

							reg, err := registry.New()
							if err != nil {
								return err
							}

							nodes := []map[string]string{}

							switch {
							case cCtx.Bool("remote"):

								local, err := reg.List()
								if err != nil {
									return err
								}

								for _, n := range local {
									nodes = append(nodes, map[string]string{
										"name":    n.Name,
										"version": n.Version + " (installed)",
									})
								}

								remote, err := reg.ListRemote()
								if err != nil {
									return err
								}

								for _, r := range remote {
									nodes = append(nodes, map[string]string{
										"name":    r.Name,
										"version": r.Latest,
									})
								}

							default:

								local, err := reg.List()
								if err != nil {
									return err
								}

								for _, n := range local {
									nodes = append(nodes, map[string]string{
										"name":    n.Name,
										"version": n.Version,
									})
								}
							}

							if len(nodes) == 0 {
								fmt.Println("No nodes found.")
								return nil
							}

							for _, n := range nodes {
								fmt.Printf("  %-20s %s\n", n["name"], n["version"])
							}

							fmt.Printf("\n%d node(s) available.\n", len(nodes))
							return nil
						},
					},
					{
						Name:  "info",
						Usage: "show detailed information about a node",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								return fmt.Errorf("node name is required")
							}
							reg, err := registry.New()
							if err != nil {
								return err
							}
							manifest, _, err := reg.Info(cCtx.Args().First())
							if err != nil {
								return err
							}
							fmt.Printf("Name:         %s\n", manifest.Name)
							fmt.Printf("Description:  %s\n", manifest.Description)
							fmt.Printf("Author:       %s\n", manifest.Author)
							if manifest.Organization != "" {
								fmt.Printf("Organization: %s\n", manifest.Organization)
							}
							fmt.Printf("License:      %s\n", manifest.License)
							if manifest.Repository != "" {
								fmt.Printf("Repository:   %s\n", manifest.Repository)
							}
							fmt.Printf("Latest:       %s\n", manifest.Latest)
							fmt.Printf("Tags:         %s\n", strings.Join(manifest.Tags, ", "))
							fmt.Printf("Capabilities: %s\n", strings.Join(manifest.Capabilities, ", "))
							fmt.Printf("Subscribes:   %s\n", strings.Join(manifest.SubscribesTo, ", "))
							fmt.Printf("Publishes:    %s\n", strings.Join(manifest.PublishesTo, ", "))
							fmt.Printf("Versions:\n")
							for _, v := range manifest.Versions {
								fmt.Printf("  %s  (%s, %d downloads)\n", v.Version, v.PublishedAt, v.Downloads)
							}
							return nil
						},
					},
					{
						Name:  "readme",
						Usage: "show the readme for a node",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								return fmt.Errorf("node name is required")
							}
							reg, err := registry.New()
							if err != nil {
								return err
							}
							content, err := reg.Readme(cCtx.Args().First())
							if err != nil {
								return err
							}
							fmt.Println(content)
							return nil
						},
					},
				},
			},
			{
				Name:  "topic",
				Usage: "run topic methods",

				Flags: []cli.Flag{

					&cli.StringFlag{
						Name:    "tx_address",
						Aliases: []string{"tx"},
						Value:   "localhost:11311",
						Usage:   "TX (publish) server address",
					},
					&cli.StringFlag{
						Name:    "rx_address",
						Aliases: []string{"rx"},
						Value:   "localhost:11312",
						Usage:   "RX (subscribe) server address",
					},
				},
				Subcommands: []*cli.Command{
					{
						Name:     "publish",
						Category: "topic",
						Aliases:  []string{"pub"},
						Usage:    "publish a topic",

						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:    "message",
								Aliases: []string{"msg"},
								Usage:   "Message to send",
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
							conn := topic.DialServer(cCtx.String("tx_address"))
							var msg interface{}
							if message == "" {
								demoMsg := new(msgs.Message)
								demoMsg.Data = "Hello AMAROS!"
								msg = interface{}(demoMsg)
							} else {
								fmt.Println("MESSAGE IS :", message)
								err := json.Unmarshal([]byte(message), &msg)
								if err != nil {
									log.Fatal("Unable to unmarshal message")
								}
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
						Usage:    "subscribe to a topic",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								log.Fatal("Topic name is required")
							}

							topicName := cCtx.Args().Get(0)
							rxconn := topic.DialServer(cCtx.String("rx_address"))
							txconn := topic.DialServer(cCtx.String("tx_address"))
							topics, err := topic.FetchList(txconn)
							if err == nil {
								if listedTopic, ok := findTopicByName(topics, topicName); ok {
									fmt.Printf("Subscribing to %s\n", listedTopic.Name)
									fmt.Printf("  type: %s\n", defaultString(listedTopic.Type, "unknown"))
									if listedTopic.OwnerNode != "" {
										fmt.Printf("  owner: %s\n", listedTopic.OwnerNode)
									}
									if listedTopic.Purpose != "" {
										fmt.Printf("  description: %s\n", listedTopic.Purpose)
									}
									fmt.Println()
								}
							}

							msg := msgs.Message{}
							_topic := topicName
							callback := func(ctx topic.CallbackContext) {
								// a simple callback that just prints the message content and type
								ctx.Logger.WithFields(map[string]interface{}{
									"topic": _topic,
									"type":  reflect.TypeOf(msg),
								}).Debug(msg.Data)
							}

							topic.Subscribe(rxconn, txconn, _topic, &msg, callback)
							return nil
						},
					},
					{
						Name:     "status",
						Aliases:  []string{"stats", "stat"},
						Category: "topic",
						Usage:    "get stats on a topic",
						Action: func(cCtx *cli.Context) error {
							if cCtx.NArg() == 0 {
								log.Fatal("Topic name is required")
							}
							conn := topic.DialServer(cCtx.String("rx_address"))
							topics, err := topic.FetchList(conn)
							if err != nil {
								return err
							}

							listedTopic, ok := findTopicByName(topics, cCtx.Args().Get(0))
							if !ok {
								return fmt.Errorf("topic %s not found", cCtx.Args().Get(0))
							}

							printTopic(listedTopic, true)
							return nil
						},
					},
					{
						Name:     "list",
						Category: "topic",
						Usage:    "get list of all topics",
						Flags: []cli.Flag{
							&cli.BoolFlag{
								Name:     "verbose",
								Aliases:  []string{"v"},
								Usage:    "detailed topics list",
								Required: false,
							},
						},
						Action: func(cCtx *cli.Context) error {
							conn := topic.DialServer(cCtx.String("rx_address"))
							topics, err := topic.FetchList(conn)
							if err != nil {
								return err
							}

							sort.Slice(topics, func(i, j int) bool {
								return topics[i].Name < topics[j].Name
							})

							if len(topics) == 0 {
								fmt.Println("No topics found.")
								return nil
							}

							for _, listedTopic := range topics {
								printTopic(listedTopic, cCtx.Bool("verbose"))
							}

							fmt.Printf("%d topic(s) found.\n", len(topics))
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

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func findTopicByName(topics []topic.Topic, name string) (topic.Topic, bool) {
	for _, listedTopic := range topics {
		if listedTopic.Name == name {
			return listedTopic, true
		}
	}

	return topic.Topic{}, false
}

func printTopic(listedTopic topic.Topic, verbose bool) {
	if !verbose {
		fmt.Printf("%-28s type=%-24s subs=%-3d", listedTopic.Name, defaultString(listedTopic.Type, "unknown"), listedTopic.Subscribers)
		if listedTopic.OwnerNode != "" {
			fmt.Printf(" owner=%s", listedTopic.OwnerNode)
		}
		if listedTopic.ResponseTopic != "" {
			fmt.Printf(" response=%s", listedTopic.ResponseTopic)
		}
		fmt.Println()
		return
	} else {
		fmt.Printf("%s\n", listedTopic.Name)
		fmt.Printf("  type: %s\n", defaultString(listedTopic.Type, "unknown"))
		fmt.Printf("  subscribers: %d\n", listedTopic.Subscribers)
		if listedTopic.OwnerNode != "" {
			fmt.Printf("  owner: %s\n", listedTopic.OwnerNode)
		}
		if listedTopic.Purpose != "" {
			fmt.Printf("  purpose: %s\n", listedTopic.Purpose)
		}
		if listedTopic.RequestTopic != "" {
			fmt.Printf("  request_topic: %s\n", listedTopic.RequestTopic)
		}
		if listedTopic.ResponseTopic != "" {
			fmt.Printf("  response_topic: %s\n", listedTopic.ResponseTopic)
		}
		if listedTopic.ResponseType != "" {
			fmt.Printf("  response_type: %s\n", listedTopic.ResponseType)
		}
	}
}
