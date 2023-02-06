package feishu

import (
	"github.com/hb0730/feishu-robot"
	"github.com/sirupsen/logrus"
)

func Send(msg string) {
	webhok := ""
	secret := ""
	client := feishu.NewClient(webhok, secret)
	_, err := client.Send(feishu.NewTextMessage().SetContent(msg).SetAtAll(true))
	if err != nil {
		logrus.Errorf("send msg err: %v", err)
	}
}
