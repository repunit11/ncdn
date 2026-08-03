package dataplane

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"

	"github.com/yzp0n/ncdn/l4lb/api"
	"github.com/yzp0n/ncdn/l4lb/l4lbdrv"
)

type StateApplier interface {
	Apply(state l4lbdrv.ForwardingState) error
}

type Server struct {
	applier StateApplier
}

func NewServer(applier StateApplier) *Server {
	return &Server{applier: applier}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/forwarding-state" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request api.ForwardingState
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	state, err := toDriverState(request)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := s.applier.Apply(state); err != nil {
		http.Error(w, "failed to apply state", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toDriverState(state api.ForwardingState) (l4lbdrv.ForwardingState, error) {
	vip, err := netip.ParseAddr(state.VIP)
	if err != nil {
		return l4lbdrv.ForwardingState{}, err
	}

	dests := make(l4lbdrv.DestinationEntries, 0, len(state.Dests))

	for _, dest := range state.Dests {
		ip, err := netip.ParseAddr(dest.IP)
		if err != nil {
			return l4lbdrv.ForwardingState{}, err
		}

		mac, err := net.ParseMAC(dest.MAC)
		if err != nil {
			return l4lbdrv.ForwardingState{}, err
		}

		dests = append(dests, l4lbdrv.DestinationEntry{
			IPAddr:       ip,
			HardwareAddr: mac,
		})
	}

	return l4lbdrv.ForwardingState{
		VIP:   vip,
		Dests: dests,
	}, nil
}
