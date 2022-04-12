package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var config = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{{URLs: []string{`stun:stun.l.google.com:19302`}}},
}

func main() {
	wsDialer := websocket.Dialer{
		ReadBufferSize:  512,
		WriteBufferSize: 512,
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	ws, response, e := wsDialer.Dial(`wss://localhost:443/signaler`, nil)
	if e != nil || response.StatusCode != http.StatusSwitchingProtocols {
		panic(fmt.Errorf(`failed to establish WebSocket connection: %v, status code: %v`, e, response.StatusCode))
	}
	signaler := SignalingSocket{ws, &sync.Mutex{}}
	peer, e := webrtc.NewPeerConnection(webrtc.Configuration{})
	if e != nil {
		panic(e)
	}
	defer peer.Close()
	peer.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice != nil {
			bin, _ := json.Marshal(ice)
			signaler.SendSignal(Signal{Event: `ice`, Data: string(bin)})
		}
	})
	chaN, e := peer.CreateDataChannel(`sample`, nil) //dummy channel to connect
	if e != nil {
		panic(e)
	}
	chaN.OnMessage(func(msg webrtc.DataChannelMessage) { log.Println(string(msg.Data)) })
	peer.OnICEConnectionStateChange(func(is webrtc.ICEConnectionState) { log.Println(`ice state:`, is.String()) })
	peer.OnConnectionStateChange(func(pcs webrtc.PeerConnectionState) { log.Println(`connection state:`, pcs.String()) })
	signal := Signal{}
	for {
		if e := signaler.ReadJSON(&signal); e != nil {
			panic(e)
		}
		switch signal.Event {
		case `ice`:
			log.Println(`got ice!`)
			ice := webrtc.ICECandidateInit{}
			if e := json.Unmarshal([]byte(signal.Data), &ice); e != nil {
				panic(e)
			}
			if e := peer.AddICECandidate(ice); e != nil {
				panic(e)
			}
		case `offer-request`:
			offer, e := peer.CreateOffer(nil)
			if e != nil {
				panic(e)
			}
			if e := peer.SetLocalDescription(offer); e != nil {
				panic(e)
			}
			bin, _ := json.Marshal(offer)
			signaler.SendSignal(Signal{Event: `offer`, Data: string(bin)})
			log.Println(`sent offer!`)
		case `answer`:
			answer := webrtc.SessionDescription{}
			if e := json.Unmarshal([]byte(signal.Data), &answer); e != nil {
				panic(e)
			}
			println(answer.SDP)
			log.Println(`got answer!`)
			if e := peer.SetRemoteDescription(answer); e != nil {
				panic(e)
			}
		}
	}
}

type SignalingSocket struct {
	*websocket.Conn
	*sync.Mutex
}

type Signal struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

func (ws *SignalingSocket) SendSignal(s Signal) {
	ws.Lock()
	defer ws.Unlock()
	ws.WriteJSON(s) //this will never fail so the error is ignored
}
