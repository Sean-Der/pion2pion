package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

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

func main() {
	http.Handle(`/`, http.FileServer(http.Dir(`client`)))
	http.HandleFunc(`/signaler`, SignalingServer)
	log.Println(`Server Initialized`)
	log.Fatal(http.ListenAndServeTLS(`:443`, `server.crt`, `server.key`, nil))
}

var (
	wsUpgrader = websocket.Upgrader{
		ReadBufferSize:  512,
		WriteBufferSize: 512,
	}
	config = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{`stun:stun.l.google.com:19302`}}},
	}
)

func SignalingServer(w http.ResponseWriter, r *http.Request) {
	ws, e := wsUpgrader.Upgrade(w, r, nil)
	if e != nil {
		panic(e)
	}
	signaler := SignalingSocket{ws, &sync.Mutex{}}
	peer, e := webrtc.NewPeerConnection(webrtc.Configuration{})
	if e != nil {
		panic(e)
	}
	defer peer.Close()
	peer.OnICECandidate(func(ice *webrtc.ICECandidate) {
		if ice != nil {
			bin, _ := json.Marshal(ice.ToJSON())
			signaler.SendSignal(Signal{Event: `ice`, Data: string(bin)})
		}
	})
	peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		log.Println(`got a chan!`)
		dc.OnOpen(func() {
			for range time.NewTicker(time.Second).C {
				dc.SendText(`echo back!`)
			}
		})
	})
	signal := Signal{Event: `offer-request`, Data: `{}`}
	signaler.SendSignal(signal)
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
		case `offer`:
			offer := webrtc.SessionDescription{}
			if e := json.Unmarshal([]byte(signal.Data), &offer); e != nil {
				panic(e)
			}
			if e := peer.SetRemoteDescription(offer); e != nil {
				panic(e)
			}
			println(offer.SDP)
			log.Println(`got offer!`)
			answer, e := peer.CreateAnswer(nil)
			if e != nil {
				panic(e)
			}
			if e := peer.SetLocalDescription(answer); e != nil {
				panic(e)
			}
			bin, _ := json.Marshal(answer)
			signaler.SendSignal(Signal{Event: `answer`, Data: string(bin)})
			log.Println(`sent answer!`)
		}
	}
}
