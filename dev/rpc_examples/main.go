package main

import (
	"fmt"
	devframework "github.com/0xkumi/incognito-dev-framework"
	"sort"
)

type topCoin struct {
	prvAmount uint64
	coinAmount uint64
	coinID string
}


var coinMap = map[string]string {
	"b832e5d3b1f01a4f0623f7fe91d6673461e1f5d37d91fe78c5c2e6183ff39696":"BTC",
	"ffd8d42dc40a8d166ea4848baf8b5f6e912ad79875f4373070b59392b1756c8f":"ETH",
	"716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0":"USDT",
	"1ff2da446abfebea3ba30385e2ca99b0f0bbeda5c6371f4c23c939672b429a42":"USDC",
	"c01e7dc1d1aba995c19b257412340b057f8ad1482ccb6a9bb0adce61afbf05d4":"XMR",
	"3f89c75324b46f13c7b036871060e641d996a24c09b3065835cb1d38b799d6c1":"DAI",
	"b2655152784e8639fa19521a7035f331eea1f1e911b2f3200a507ebb4554387b":"BNB",
	"dae027b21d8d57114da11209dce8eeb587d01adf59d4fc356a8be5eedc146859":"MATIC",
	"9e1142557e63fd20dee7f3c9524ffe0aa41198c494aa8d36447d12e85f0ddce7":"BUSD",
}
func main() {
	client := devframework.NewRPCClient("http://139.162.54.236:38934")
	bb,_ := client.API_GetBeaconBestState()
	pde,_ := client.API_GetPDEState(float64(bb.BeaconHeight))
	topList := PairList{}
	for _,pair := range pde.PDEPoolPairs {
		if pair.Token1IDStr == "0000000000000000000000000000000000000000000000000000000000000004" && pair.Token1PoolValue > uint64(20000*1e9){
			topList = append(topList, topCoin{
				coinAmount: pair.Token2PoolValue,
				prvAmount: pair.Token1PoolValue,
				coinID: pair.Token2IDStr,
			})
		}
	}

	sort.Sort(topList)
	for _, pair := range topList{
		if coinMap[pair.coinID] == "USDT" || coinMap[pair.coinID] == "USDC" {
			fmt.Println(coinMap[pair.coinID],  float64(pair.coinAmount)*1e-6)
		} else {
			fmt.Println(coinMap[pair.coinID],  float64(pair.coinAmount)*1e-9)
		}

	}

}

type PairList []topCoin

func (p PairList) Len() int {
	return len(p)
}

func (p PairList) Less(i, j int) bool {
	return (p[i].prvAmount > p[j].prvAmount)
}

func (s PairList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

