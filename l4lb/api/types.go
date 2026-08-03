package api

type ForwardingState struct {
	VIP   string        `json:"vip"`
	Dests []Destination `json:"dests"`
}

type Destination struct {
	IP  string `json:"ip"`
	MAC string `json:"mac"`
}
