package auctionaudit

import (
	"crypto/sha512"

	"github.com/IBM/sarama"
	"github.com/prebid/prebid-server/v3/config"
	"github.com/xdg-go/scram"
)

var SHA512 scram.HashGeneratorFcn = sha512.New

type XDGSCRAMClient struct {
	*scram.Client
	*scram.ClientConversation
	scram.HashGeneratorFcn
}

func (x *XDGSCRAMClient) Begin(userName, password, authzID string) (err error) {
	x.Client, err = x.HashGeneratorFcn.NewClient(userName, password, authzID)
	if err != nil {
		return err
	}
	x.ClientConversation = x.Client.NewConversation()
	return nil
}

func (x *XDGSCRAMClient) Step(challenge string) (response string, err error) {
	response, err = x.ClientConversation.Step(challenge)
	return
}

func (x *XDGSCRAMClient) Done() bool {
	return x.ClientConversation.Done()
}

func configureSASL(cfg *sarama.Config, saslCfg config.SASLConfig) {
	cfg.Net.SASL.Enable = true
	cfg.Net.SASL.User = saslCfg.Username
	cfg.Net.SASL.Password = saslCfg.Password
	cfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	cfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
		return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
	}
}

