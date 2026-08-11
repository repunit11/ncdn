package control

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/yzp0n/ncdn/l4lb/api"
	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

type DataPlaneClient struct {
	baseURL string
	client  *http.Client
}

func NewDataPlaneClient(baseURL string) *DataPlaneClient {
	return &DataPlaneClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *DataPlaneClient) Apply(state l4lbdrv.ForwardingState) error {
	reqBody := toAPIState(state)
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, c.baseURL+"/v1/forwarding-state", bytes.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func toAPIState(state l4lbdrv.ForwardingState) api.ForwardingState {
	vip := state.VIP.String()
	dests := make([]api.Destination, 0, len(state.Dests))
	for _, dest := range state.Dests {
		dests = append(dests, api.Destination{
			IP:  dest.IPAddr.String(),
			MAC: dest.HardwareAddr.String(),
		})
	}
	return api.ForwardingState{
		VIP:   vip,
		Dests: dests,
	}
}
