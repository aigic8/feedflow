package utils

import (
	"context"
	"time"

	"github.com/nikoksr/notify"
	"github.com/nikoksr/notify/service/discord"
)

type DiscordNotify struct {
	core       *notify.Notify
	BotToken   string
	ChannelIDs []string
	Timeout    time.Duration
}

func NewDiscordNotify(botToken string, channelIDs []string, timeout time.Duration) *DiscordNotify {
	discordService := discord.New()
	discordService.AuthenticateWithBotToken(botToken)
	discordService.AddReceivers(channelIDs...)

	appNotify := notify.New()
	appNotify.UseServices(discordService)

	return &DiscordNotify{core: appNotify, BotToken: botToken, ChannelIDs: channelIDs, Timeout: timeout}
}

func (n *DiscordNotify) Send(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), n.Timeout)
	defer cancel()
	return n.core.Send(ctx, "", text)
}
