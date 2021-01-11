package main

import (
	"encoding/json"
	"github.com/incognitochain/incognito-chain/wire"
)

func GetMsgVersion(msg *wire.MessageBFT) int{
	if msg.Type == "propose" {
		type Alias struct {
			Block struct {
				Header struct {
					Version int
				}
			}
		}
		alias := Alias{}
		json.Unmarshal(msg.Content,&alias)
		return alias.Block.Header.Version
	}

	if msg.Type == "vote" {
		type Alias struct {
			RoundKey string
		}
		alias := Alias{}
		json.Unmarshal(msg.Content,&alias)
		if alias.RoundKey != ""  {
			return 1
		}
		return 2
	}

	return 0
}
