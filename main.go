package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	_ "github.com/glebarez/go-sqlite"
	"github.com/urfave/cli/v2"
	"go.bug.st/serial"
)

/*
|--------------------------------------------------------------------------
| Variables
|--------------------------------------------------------------------------
|
*/
var database *sql.DB

/*
|--------------------------------------------------------------------------
| Structs
|--------------------------------------------------------------------------
*/
type payload struct {
	Value int `json:"value:"`
}

type Mqtt struct {
	ID       int    `json:"id"`
	ClientID string `json:"client_id"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

/*
|--------------------------------------------------------------------------
| Callbacks
|--------------------------------------------------------------------------
*/
var f mqtt.MessageHandler = func(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("TOPIC: %s\n", msg.Topic())
	fmt.Printf("MSG: %s\n", msg.Payload())
}

func main() {
	var dir string
	currentOs := runtime.GOOS
	if currentOs == "windows" {
		dir = "./.pws/pws.db"
	} else {
		osUser, _ := user.Current()
		dir = osUser.HomeDir + "/.pws/pws.db"
	}

	var err error
	database, err = sql.Open("sqlite", dir)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	cli.VersionFlag = &cli.BoolFlag{
		Name:    "print-version",
		Aliases: []string{"v"},
		Usage:   "print only the version",
	}

	app := &cli.App{
		Name:    "PWS",
		Usage:   "Plant Watering System",
		Version: "v0.0.1",
		Commands: []*cli.Command{

			{
				Name:  "auth",
				Usage: "Set the credentials for the mqtt broker",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "username", Aliases: []string{"usr"}},
					&cli.StringFlag{Name: "password", Aliases: []string{"pwd"}},
					&cli.StringFlag{Name: "host"},
				},
				Action: func(cCtx *cli.Context) error {
					mqttConfig := Mqtt{
						ClientID: cCtx.String("username"),
						Password: cCtx.String("password"),
						Host:     cCtx.String("host"),
					}

					_, err := database.Exec("INSERT INTO mqtt (client_id, password, host) VALUES (?, ?, ?)", mqttConfig.ClientID, mqttConfig.Password, mqttConfig.Host)
					if err != nil {
						log.Fatal(err)
					}

					fmt.Println("auth credentials set")
					return nil
				},
			},
			{
				Name:    "provision-url",
				Aliases: []string{"purl"},
				Usage:   "provision the device",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("provision url: ", cCtx.Args().First())
					return nil
				},
			},
			{
				Name:  "run",
				Usage: "run the application",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("running the application")
					StartMqtt()
					return nil
				},
			},
			{
				Name:  "migrate",
				Usage: "migrate the database",
				Action: func(cCtx *cli.Context) error {
					fmt.Println("migrating the database")
					Migration()
					fmt.Println("migrated the database")
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func StartMqtt() {
	keepAlive := make(chan os.Signal, 1)
	signal.Notify(keepAlive, os.Interrupt, syscall.SIGTERM)

	mqtt.ERROR = log.New(os.Stdout, "", 0)

	var mqttConfig Mqtt

	row := database.QueryRow("SELECT * FROM mqtt ORDER BY id DESC LIMIT 1")
	if err := row.Scan(&mqttConfig.ID, &mqttConfig.ClientID, &mqttConfig.Password, &mqttConfig.Host); err != nil {
		if err == sql.ErrNoRows {
			log.Fatal("No auth credentials found")
		}
		log.Fatal(err)
	}

	opts := mqtt.NewClientOptions().AddBroker("tcp://" + mqttConfig.Host + ":1883").SetClientID(mqttConfig.ClientID).SetPassword(mqttConfig.Password).SetUsername(mqttConfig.ClientID)

	opts.SetKeepAlive(60 * time.Second)
	// Set the message callback handler
	opts.SetDefaultPublishHandler(f)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)

	retry := time.NewTicker(5 * time.Second)

RetryLoop:
	for {
		_, ok := <-retry.C
		if ok {
			if token := c.Connect(); token.Wait() && token.Error() != nil {
				fmt.Println(token.Error())
			} else {
				retry.Stop()
				break RetryLoop
			}
		}
	}

	go func() {
		port, err := GetSerialPort()
		if err != nil {
			fmt.Println(err)
		}

		for {
			value := ReadFromSerialPort(port)

			// string to int
			i, err := strconv.Atoi(value)
			if err != nil {
				continue
			}

			data := &payload{
				Value: i,
			}

			dataByte, _ := json.Marshal(data)
			token := c.Publish("devices/"+mqttConfig.ClientID+"/sensors", 0, false, dataByte)
			token.Wait()
			time.Sleep(time.Second)
		}
	}()

	<-keepAlive
}

func Migration() {
	statement, err := database.Prepare("CREATE TABLE IF NOT EXISTS mqtt (id INTEGER PRIMARY KEY, client_id TEXT, password TEXT, host TEXT)")
	if err != nil {
		log.Fatal(err)
	}

	defer statement.Close()
	_, err = statement.Exec()
	if err != nil {
		log.Fatal(err)
	}
}

func GetSerialPort() (serial.Port, error) {
	// Retrieve the port list
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}
	if len(ports) == 0 {
		log.Fatal("No serial ports found!")
	}

	// Print the list of detected ports
	for _, port := range ports {
		fmt.Printf("Found port: %v\n", port)
	}

	// Open the first serial port detected at 9600bps N81
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.NoParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	for _, port := range ports {
		if strings.Contains(port, "ttyUSB") {
			return serial.Open(port, mode)
		}
	}

	return serial.Open(ports[0], mode)
}

func ReadFromSerialPort(port serial.Port) string {

	// Read and print the response
	buff := make([]byte, 100)

	var value string

	// Reads up to 100 bytes
	for {
		// Reads up to 100 bytes
		n, err := port.Read(buff)
		if err != nil {
			log.Fatal(err)
		}
		if n == 0 {
			fmt.Println("\nEOF")
			break
		}

		value += string(buff[:n])

		// If we receive a newline stop reading
		if strings.Contains(string(buff[:n]), "\n") {
			break
		}
	}

	return strings.TrimSpace(value)
}
