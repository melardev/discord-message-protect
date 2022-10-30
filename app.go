package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/melardev/discord-message-protect/captchas"
	"github.com/melardev/discord-message-protect/core"
	"github.com/melardev/discord-message-protect/http_server"
	"github.com/melardev/discord-message-protect/logging"
	"github.com/melardev/discord-message-protect/pollution"
	"github.com/melardev/discord-message-protect/secrets"
	"github.com/melardev/discord-message-protect/sessions"
	"github.com/melardev/discord-message-protect/utils"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var app *Application

type ApplicationContext struct {
	DiscordBot        discordgo.Session
	CaptchaValidator  captchas.ICaptchaValidator
	SessionManager    sessions.ISessionManager
	SecretManager     secrets.ISecretManager
	DefaultLogger     logging.ILogger
	PollutionLogger   logging.ILogger
	PollutionStrategy pollution.IPollutionStrategy
	Config            *core.Config
	HttpServer        *http_server.MuxHttpServer
}

var revealSecretMutex sync.Mutex
var revealSecretRequests = map[string]*secrets.RevealRequest{}

type Args struct {
	ConfigPath string
	Verbose    bool
}

type Application struct {
	Context *ApplicationContext
}

func GetApplication(args *Args) *Application {
	if app != nil {
		return app
	}

	app = &Application{}

	app.Context = &ApplicationContext{}

	if args.ConfigPath != "" {
		content, err := ioutil.ReadFile(args.ConfigPath)
		if err != nil {
			panic(err)
		}

		err = json.Unmarshal(content, &app.Context.Config)
		if err != nil {
			panic(err)
		}
	}

	if app.Context.Config.AppPath == "" {
		appPath, err := filepath.Abs(".")
		if err != nil {
			panic(err)
		}
		app.Context.Config.AppPath = appPath
	}

	if app.Context.Config.LogPath == "" {
		logPath := filepath.Join(app.Context.Config.AppPath, "logs")

		app.Context.Config.LogPath = logPath
	}

	if !utils.DirExists(app.Context.Config.LogPath) {
		err := os.MkdirAll(app.Context.Config.LogPath, 0644)
		if err != nil {
			panic(err)
		}
	}

	app.Context.DefaultLogger = logging.NewCompositeLogger(
		&logging.ConsoleLogger{},
		logging.NewFileLogger(filepath.Join(app.Context.Config.LogPath, "app.log")),
	)

	app.Context.PollutionLogger = logging.NewCompositeLogger(
		&logging.ConsoleLogger{},
		logging.NewFileLogger(filepath.Join(app.Context.Config.LogPath, "pollution.log")),
	)

	if args.Verbose {
		app.Context.DefaultLogger.SetMinLevel(logging.Debug)
	} else {
		app.Context.DefaultLogger.SetMinLevel(logging.Warn)
	}

	app.Context.PollutionLogger.SetMinLevel(logging.Warn)
	app.Context.SessionManager = sessions.NewInMemoryAuthenticator(app.Context.Config)
	app.Context.SecretManager = secrets.NewInMemorySecretManager(app.Context.Config)

	if app.Context.Config.HttpConfig != nil {
		app.Context.HttpServer = http_server.NewMuxHttpServer(app.Context.Config, app)
		app.Context.HttpServer.Run()
	}

	if app.Context.Config.PollutionConfig != nil {
		app.Context.PollutionStrategy = pollution.GetPollutionStrategy(app.Context.Config.PollutionConfig.StrategyName,
			app.Context.Config.PollutionConfig.Args)
	}

	dg, err := discordgo.New("Bot " + app.Context.Config.DiscordConfig.BotToken)
	if err != nil {
		message := fmt.Sprintf("error creating DiscordConfig session %v\n", err)
		panic(message)
	}

	// Just like the ping pong example, we only care about receiving message
	// events in this example.
	dg.Identify.Intents = discordgo.IntentsGuildMessages
	// Open a websocket connection to DiscordConfig and begin listening.
	err = dg.Open()
	if err != nil {
		message := fmt.Sprintf("error opening connection %v\n", err)
		panic(message)
	}

	dg.AddHandler(app.OnMessageCreate)
	dg.AddHandler(app.OnDiscordBotReady)
	dg.AddHandler(app.OnCreateInteraction)

	_, err = dg.ApplicationCommandCreate(
		app.Context.Config.DiscordConfig.AppId,
		app.Context.Config.DiscordConfig.GuildId, &discordgo.ApplicationCommand{
			Name:        app.Context.Config.DiscordConfig.ProtectCommandName,
			Description: "Protects a message by challenging the user",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "content",
					Description: "Content",
					Required:    true,
				},
				{
					// TODO: Implement me, for now it is just in paper, there is no implementation backing this
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "pollute",
					Description: "Pollute message",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{
							Name: "incremental",
							Value: pollution.GetPollutionStrategy(pollution.IncrementIntStrategyName,
								map[string]interface{}{
									"position": "beginning",
								}),
						},
						{
							Name: "faker",
							Value: pollution.GetPollutionStrategy(pollution.FakerStrategyName,
								map[string]interface{}{
									"position":  pollution.Random,
									"min_words": 1,
									"max_words": 10,
								}),
						},
					},
					Required: false,
				},
				{
					// TODO: Implement me, for now it is just in paper, there is no implementation backing this
					Type:        discordgo.ApplicationCommandOptionAttachment,
					Name:        "image",
					Description: "Content",
					Required:    false,
				},
			},
		})

	app.Context.DefaultLogger.Info(fmt.Sprintf("Application started\n"))
	return app
}

func (a *Application) Run() {
	// TODO: Improve this
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Graceful shutdown")
}

func (a *Application) OnDiscordBotReady(s *discordgo.Session, r *discordgo.Ready) {
	a.Context.DefaultLogger.Info(fmt.Sprintf("DiscordConfig bot initialized, logged in on %s as - %s#%s\n",
		strings.Join(utils.GetGuildNames(s.State.Guilds), ","),
		s.State.User.Username,
		s.State.User.ID))
}

func (a *Application) OnCreateInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		switch i.ApplicationCommandData().Name {
		case a.Context.Config.DiscordConfig.ProtectCommandName:
			a.OnProtectCommand(s, i)
		}
	case discordgo.InteractionMessageComponent:
		customId := i.MessageComponentData().CustomID
		if strings.HasPrefix(customId, "btn_unlock_") {
			a.OnUnlockInteraction(s, i)
		}
	}
}

func (a *Application) OnProtectCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {

	messageContent := ""
	if data, ok := i.Interaction.Data.(discordgo.ApplicationCommandInteractionData); ok {
		for _, option := range data.Options {
			if option.Name == "content" {
				messageContent = option.Value.(string)
			}

			if option.Name == "images" {
				fmt.Printf("We got some image! %#v\n", option.Value)
			}
		}
	}

	// it is documented that in some conditions i.User is nil instead we should use i.Member
	user := i.User
	if user == nil {
		user = i.Member.User
	}

	secret, err := a.Context.SecretManager.CreateOrUpdate(&secrets.CreateSecretDto{
		User:      user,
		Message:   messageContent,
		ChannelId: i.ChannelID,
	})

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:   discordgo.MessageFlagsEphemeral,
			Content: "Users are notified about secret, they must click on the unlock button now",
		},
	})

	if err != nil {
		a.Context.DefaultLogger.Error(fmt.Sprintf("An error occurred on OnProtectCommand::InteractionRespond - %v\n", err))
		return
	}

	// Send a message to everyone indicating we have a new protected message
	// to read it they have to pass the challenge (a click for now, probably a captcha too)
	_, err = s.ChannelMessageSendComplex(i.ChannelID, &discordgo.MessageSend{
		Content: "Click to Unlock",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Click to Unlock",
						CustomID: fmt.Sprintf("btn_unlock_%s", secret.Id),
						Style:    discordgo.PrimaryButton,
						Disabled: false,
						Emoji: discordgo.ComponentEmoji{
							Name:     "🔒",
							Animated: false,
						},
					},
				},
			},
		},
	})

	if err != nil {
		panic(err)
	}
}

// OnUnlockInteraction is a callback triggered when the user wants to see a protected message
// if the user is authenticated (passed challenge recently) he is gonna get the message without further interaction
// if not, the user would need to pass an additional challenge(if configured)
func (a *Application) OnUnlockInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	customId := i.MessageComponentData().CustomID
	secretId := strings.Replace(customId, "btn_unlock_", "", -1)

	secret := a.Context.SecretManager.GetById(secretId)

	if secret == nil {
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: fmt.Sprintf("Protected message has expired or no longer available"),
			},
		})

		if err != nil {
			a.Context.DefaultLogger.Error(fmt.Sprintf("An error occurred on OnUnlockInteraction::SecretNull - %v\n", err))
		}

		return
	}

	// If we are not using captcha based protection, or we do but the user already passed it previously
	// then just show the protected message without further action from the user
	if a.Context.Config.HttpConfig.CaptchaService == "" ||
		a.Context.SessionManager.IsAuthenticated(s.State.SessionID) {
		secretContent := secret.Message

		if a.Context.PollutionStrategy != nil {
			modifiedMessage, indicators := a.Context.PollutionStrategy.Apply(secretContent)
			a.Context.PollutionLogger.Info(fmt.Sprintf("Applied %s strategy, User: %s, SecretId: %s, Indicators: %s\n",
				a.Context.PollutionStrategy.GetName(),
				fmt.Sprintf("%s#%s", s.State.User.Username, s.State.User.ID),
				secret.Id, indicators))
			secretContent = modifiedMessage
		}

		// User is authenticated, reveal the secret
		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags:   discordgo.MessageFlagsEphemeral,
				Content: secretContent,
			},
		})

		if err != nil {
			a.Context.DefaultLogger.Error(fmt.Sprintf("An error occurred on OnUnlockInteraction::InteractionRespond for authenticated user - %v\n",
				err))
		}
	} else {
		// Send another interaction, the ideal would be to use a LinkButton with CustomID to get its interaction
		// but it is not possible, so we must notify the user to click on the embed's link and only click
		// the unlock button after he resolved the captcha challenge, I have to find a better solution for this
		// but for now it does not seem to be possible otherwise.
		reqId := utils.GetRandomString(16)

		challengePath := a.Context.Config.HttpConfig.ChallengePath
		if !strings.HasPrefix(challengePath, "/") {
			challengePath = "/" + challengePath
		}

		now := time.Now().UTC()
		challengeUrl := fmt.Sprintf("%s://%s:%d%s?req_id=%s",
			a.Context.Config.HttpConfig.Scheme,
			a.Context.Config.HttpConfig.Hostname,
			a.Context.Config.HttpConfig.Port,
			challengePath,
			reqId)

		req := &secrets.RevealRequest{
			User: &core.DiscordUser{
				Id:       s.State.User.ID,
				Username: s.State.User.Username,
			},
			Secret:    secret,
			ChannelId: secret.ChannelId,
			CreatedAt: now,
			UpdatedAt: now,
		}

		err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Flags: discordgo.MessageFlagsEphemeral,
				/*Embeds: []*discordgo.MessageEmbed{
					{
						Type:        discordgo.EmbedTypeLink,
						Title:       "Security Check",
						Description: "Please click the following link, solve the captcha, then click on Unlock",
						URL:         challengeUrl,
					},
				},*/
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.Button{
								Label:    "Resolve Captcha",
								Style:    discordgo.LinkButton,
								Disabled: false,
								URL:      challengeUrl,
								Emoji: discordgo.ComponentEmoji{
									Name:     "🔒",
									Animated: false,
								},
							},
						},
					},
				},
			},
		})

		if err != nil {
			a.Context.DefaultLogger.Error(fmt.Sprintf("An error occurred on OnUnlockInteraction::CreateChallenge - %v\n", err))
		} else {
			revealSecretMutex.Lock()
			revealSecretRequests[reqId] = req
			revealSecretMutex.Unlock()
			// The interaction object is going to be used later to send the message to the user
			req.Interaction = i.Interaction
			req.Session = s
		}
	}

}

func (a *Application) OnMessageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Nothing for now
}

func OnUpdateSecret() {
	// TODO: Implement me
}

func OnAddSecret() {
	// TODO: Implement me
}

func OnDeleteSecret() {
	// TODO: Implement me
}

// OnValidCaptcha Application implements the interface http server needs to notify us about a user succeeding the
// captcha challenge
func (a *Application) OnValidCaptcha(requestId string) {

	revealSecretMutex.Lock()
	if revealReq, found := revealSecretRequests[requestId]; !found {
		// Reveal request not found, probably came too late, or a user just forged/tampered the request
		revealSecretMutex.Unlock()
		return
	} else {

		a.Context.SessionManager.Authenticate(revealReq.User.Id, revealReq.User.Username)

		secretContent := revealReq.Secret.Message
		if a.Context.PollutionStrategy != nil {
			modifiedMessage, indicators := a.Context.PollutionStrategy.Apply(revealReq.Secret.Message)
			a.Context.PollutionLogger.Info(fmt.Sprintf("Applied %s strategy, User: %s, SecretId: %s, Indicators: %s\n",
				a.Context.PollutionStrategy.GetName(),
				fmt.Sprintf("%s#%s", revealReq.User.Username, revealReq.User.Id),
				revealReq.Secret.Id, indicators))
			secretContent = modifiedMessage
		}

		_, err := revealReq.Session.InteractionResponseEdit(revealReq.Interaction, &discordgo.WebhookEdit{
			Content: &secretContent,
		})

		if err != nil {
			a.Context.DefaultLogger.Error(fmt.Sprintf("An error occurred on OnValidCaptcha::InteractionResponseEdit - %v\n", err))
		}

		// The request is fulfilled, delete it
		delete(revealSecretRequests, requestId)
		revealSecretMutex.Unlock()
	}
}
