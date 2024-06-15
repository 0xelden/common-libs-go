package models

var (
	DefaultMessage = map[string]string{
		"id": "Berhasil",
	}
)

type Error struct {
	UserMessage     string `json:"userMessage" xml:"UserMessage"`
	InternalMessage string `json:"internalMessage" xml:"InternalMessage"`
	Code            int    `json:"code" xml:"Code"`
	MoreInfo        string `json:"moreInfo" xml:"MoreInfo"`
}

type ResponseBody struct {
	Response   int
	Error      string
	Appid      string
	Svcid      string
	Controller string
	Action     string
	Result     interface{}
}

type Response struct {
	Response   int         `json:"response" xml:"Response"`
	Error      string      `json:"error" xml:"Error"`
	Appid      string      `json:"appid" xml:"Appid"`
	Svcid      string      `json:"svcid" xml:"Svcid"`
	Controller string      `json:"controller" xml:"Controller"`
	Action     string      `json:"action" xml:"Action"`
	Result     interface{} `json:"result" xml:"Result"`
}
