package secrets

import (
	"github.com/bwmarrin/discordgo"
	"github.com/melardev/discord-message-protect/core"
	"time"
)

type RevealRequest struct {
	User      *core.DiscordUser
	Secret    *Secret
	ChannelId string

	// It is imperative to use the session passed along with the interaction
	// we can not use the global bot session, for some reason the global bot session works
	// while i was experimenting in a unit test
	// however in the app it does not, probably because of some delays? anyway, save the session
	// and use it to edit interactions
	Session     *discordgo.Session
	Interaction *discordgo.Interaction

	CreatedAt time.Time
	UpdatedAt time.Time
}

type Secret struct {
	Id        string
	Message   string
	ChannelId string
	CreatedAt time.Time
	UpdatedAt time.Time
	User      *core.DiscordUser
}

type CreateSecretDto struct {
	Id        string
	Content   string
	Message   string
	ChannelId string
	User      *discordgo.User
}

type ISecretManager interface {
	GetById(id string) *Secret
	CreateOrUpdate(dto *CreateSecretDto) (*Secret, error)
	Delete(id string)
}
