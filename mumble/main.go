package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	_ "github.com/joho/godotenv/autoload"

	"github.com/charmbracelet/log"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"layeh.com/gumble/gumble"
	"layeh.com/gumble/gumbleutil"
)

var (
	DEBUG = getEnvExists("DEBUG")

	ROD_DOCKER_HOST = getEnv("ROD_DOCKER_HOST", "")

	MUMBLE_USERNAME = getEnv("MUMBLE_USERNAME", "mikogo")
	MUMBLE_SERVER   = getEnv("MUMBLE_SERVER", "")
	MUMBLE_CHANNEL  = getEnv("MUMBLE_CHANNEL", "")
)

func handleTextMessage(e *gumble.TextMessageEvent, msg string, browser *rod.Browser) {
	if msg == "test" {
		wordArtImg := makeWordArtPng(browser, e.Sender.Name)

		html := imageForMumble(wordArtImg, &MumbleImageOptions{
			Transparent: true,
			MaxHeight:   100,
		})

		e.Sender.Channel.Send(html, false)

		// sendToAll(e.Client, html)
	}
}

func handleUserConnected(e *gumble.UserChangeEvent, browser *rod.Browser) {
	wordArtImg := makeWordArtPng(browser, e.User.Name)

	html := imageForMumble(wordArtImg, &MumbleImageOptions{
		Transparent: true,
		MaxHeight:   100,
	})

	sendToAll(e.Client, html)
}

func main() {
	if DEBUG {
		log.SetLevel(log.DebugLevel)
	}

	if MUMBLE_SERVER == "" {
		log.Fatal("please specify MUMBLE_SERVER")
	}

	var browser *rod.Browser

	if ROD_DOCKER_HOST != "" {
		log.Info("initializing browser through docker...")
		// https://github.com/go-rod/rod/blob/main/lib/examples/launch-managed/main.go
		browserLauncher := launcher.MustNewManaged(fmt.Sprintf("ws://%s:7317", ROD_DOCKER_HOST))
		browserLauncher.Set("disable-gpu").Delete("disable-gpu")
		browserLauncher.Headless(false).XVFB("--server-num=5", "--server-args=-screen 0 1600x900x16")
		browser = rod.New().Client(browserLauncher.MustClient()).MustConnect()
	} else {
		log.Info("initializing browser regularly...")
		browserLauncher := launcher.New().Headless(true).MustLaunch()
		browser = rod.New().ControlURL(browserLauncher).MustConnect()
	}

	log.Info("connecting to mumble...")

	keepAlive := make(chan bool)

	config := gumble.NewConfig()

	config.Username = MUMBLE_USERNAME
	config.Attach(gumbleutil.AutoBitrate)

	config.Attach(gumbleutil.Listener{
		Connect: func(e *gumble.ConnectEvent) {
			log.Infof("connected as: %s", e.Client.Self.Name)

			var foundChannel *gumble.Channel

			for _, channel := range e.Client.Channels {
				if channel.Name == MUMBLE_CHANNEL {
					foundChannel = channel
					break
				}
			}

			if foundChannel == nil {
				return
			}

			e.Client.Self.Move(foundChannel)
			log.Infof("moved to: %s", foundChannel.Name)

			e.Client.Self.SetSelfDeafened(true)
		},

		TextMessage: func(e *gumble.TextMessageEvent) {
			if e.Sender == nil {
				return
			}

			msg := strings.TrimSpace(e.Message)

			log.Infof("%s: %s", e.Sender.Name, msg)

			go handleTextMessage(e, msg, browser)
		},

		UserChange: func(e *gumble.UserChangeEvent) {
			if e.Type.Has(gumble.UserChangeConnected) {
				log.Infof("%s %s", e.User.Name, "joined")
				go handleUserConnected(e, browser)
			}
		},

		Disconnect: func(e *gumble.DisconnectEvent) {
			keepAlive <- true
		},
	})

	_, err := gumble.DialWithDialer(new(net.Dialer), MUMBLE_SERVER, config, &tls.Config{})
	if err != nil {
		log.Fatal(err)
	}

	<-keepAlive

}
